package p2p

import (
	"bytes"
	"context"
	"fmt"
	"github.com/cenkalti/backoff/v4"
	lib "github.com/ginchuco/ginchu/types"
	"github.com/ginchuco/ginchu/types/crypto"
	"golang.org/x/net/netutil"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

/*
	P2P TODOs
	- TCP/IP transport [x]
	- Multiplexing [x]
	- Encrypted connection [x]
	- UnPn & nat-pimp auto config [-]
	- DOS mitigation [x]
	- Peer configs: unconditional, num in/out, timeouts [x]
	- Peer list: discover[x], churn[x], share[x]
	- Message dissemination: gossip [x]
	- Message dissemination improved: raintree [ ]
*/

const (
	transport   = "TCP"
	dialTimeout = time.Second
)

var _ lib.P2P = new(P2P)

type P2P struct {
	privateKey crypto.PrivateKeyI
	listener   net.Listener
	channels   lib.Channels
	config     Config
	PeerSet              // active set
	book       *PeerBook // not active set

	state State
}

func NewP2P(p crypto.PrivateKeyI, maxValidators int, channels lib.Channels, c Config) *P2P {
	peerBook := &PeerBook{} // TODO load from file / write to file
	c.maxValidators = maxValidators
	return &P2P{
		privateKey: p,
		channels:   channels,
		config:     c,
		PeerSet: PeerSet{
			RWMutex: sync.RWMutex{},
			config:  c,
			m:       make(map[string]*Peer),
			book:    peerBook,
		},
		book: peerBook,
		state: State{
			RWMutex: sync.RWMutex{},
		},
	}
}

func (p *P2P) Start(validatorsReceiver chan []*lib.PeerAddress) {
	go p.InternalListenValidators(validatorsReceiver)
	go p.Listen(&lib.PeerAddress{NetAddress: p.config.ListenAddress})
	for _, peer := range p.config.DialPeers {
		pi := new(lib.PeerAddress)
		if err := pi.FromString(peer); err != nil {
			// log error
			continue
		}
		p.DialWithBackoff(pi)
	}
}

func (p *P2P) Dial(address *lib.PeerAddress, disconnect bool) lib.ErrorI {
	if p.IsSelf(address) || p.PeerSet.Has(address.PublicKey) {
		return nil
	}
	conn, er := net.DialTimeout(transport, address.NetAddress, dialTimeout)
	if er != nil {
		p.book.AddFailedDialAttempt(address.PublicKey)
		return ErrFailedDial(er)
	}
	return p.AddPeer(conn, &lib.PeerInfo{
		Address:    address,
		IsOutbound: true,
	}, disconnect)
}

func (p *P2P) Listen(listenAddress *lib.PeerAddress) {
	ln, er := net.Listen(transport, listenAddress.NetAddress)
	if er != nil {
		panic(ErrFailedListen(er))
	}
	p.listener = netutil.LimitListener(ln, p.config.MaxInbound+len(p.config.TrustedPeerIDs)+p.config.maxValidators)
	for {
		c, err := p.listener.Accept()
		if err != nil {
			return
		}
		go func(c net.Conn) {
			defer p.catchPanic()
			peerAddress, e := p.filter(c)
			if e != nil {
				_ = c.Close()
				return
			}
			if err = p.AddPeer(c, &lib.PeerInfo{Address: peerAddress}, false); err != nil {
				_ = c.Close()
				return
			}
		}(c)
	}
}

func (p *P2P) AddPeer(conn net.Conn, info *lib.PeerInfo, disconnect bool) lib.ErrorI {
	connection, err := NewConnection(conn, p.NewStreams(), p, p.OnPeerError, p.privateKey)
	if err != nil {
		return err
	}
	if info.Address.PublicKey != nil && !bytes.Equal(connection.peerPublicKey, info.Address.PublicKey) {
		return ErrMismatchPeerPublicKey(info.Address.PublicKey, connection.peerPublicKey)
	}
	if disconnect {
		connection.Stop()
		return nil
	}
	p.state.RLock()
	for _, v := range p.state.validators {
		if bytes.Equal(v.PublicKey, info.Address.PublicKey) {
			info.IsValidator = true
			break
		}
	}
	p.state.RUnlock()
	for _, t := range p.config.TrustedPeerIDs {
		if lib.BytesToString(connection.peerPublicKey) == t {
			info.IsTrusted = true
			break
		}
	}
	info.Address.PublicKey = connection.peerPublicKey
	streams := make(map[lib.Topic]*Stream)
	for i := lib.Topic(0); i < lib.Topic_INVALID; i++ {
		streams[i] = &Stream{
			topic:         i,
			sendQueue:     make(chan []byte, maxQueueSize),
			sendQueueSize: atomic.Int32{},
			receive:       p.ReceiveChannel(i),
		}
	}
	for _, banned := range p.config.BannedPeerIDs {
		pubKeyString := lib.BytesToString(connection.peerPublicKey)
		if pubKeyString == banned {
			return ErrBannedID(pubKeyString)
		}
	}
	p.book.Add(&BookPeer{Address: info.Address})
	return p.PeerSet.Add(&Peer{
		conn:     connection,
		PeerInfo: info,
		stop:     sync.Once{},
	})
}

func (p *P2P) DialWithBackoff(peerInfo *lib.PeerAddress) {
	_ = backoff.Retry(func() error { return p.Dial(peerInfo, false) }, backoff.NewExponentialBackOff())
}
func (p *P2P) DialAndDisconnect(a *lib.PeerAddress) lib.ErrorI {
	return p.Dial(a, true)
}

func (p *P2P) InternalListenValidators(validatorsReceiver chan []*lib.PeerAddress) {
	selfPubKey := p.privateKey.PublicKey().Bytes()
	for vs := range validatorsReceiver {
		p.state.Lock()
		p.state.validators = vs
		for _, val := range p.UpdateValidators(selfPubKey, vs) {
			go p.DialWithBackoff(val)
		}
		p.state.Unlock()
	}
}

func (p *P2P) OnPeerError(publicKey []byte) {
	if err := p.PeerSet.Remove(publicKey); err != nil {
		fmt.Println(err.Error()) // handle error
	}
}

func (p *P2P) NewStreams() (streams map[lib.Topic]*Stream) {
	streams = make(map[lib.Topic]*Stream)
	for i := lib.Topic(0); i < lib.Topic_INVALID; i++ {
		streams[i] = &Stream{
			topic:         i,
			sendQueue:     make(chan []byte, maxQueueSize),
			sendQueueSize: atomic.Int32{},
			receive:       p.ReceiveChannel(i),
		}
	}
	return
}
func (p *P2P) IsSelf(address *lib.PeerAddress) bool {
	return bytes.Equal(p.privateKey.PublicKey().Bytes(), address.PublicKey)
}
func (p *P2P) MaxPossiblePeers() int {
	c := p.config
	return c.MaxInbound + c.MaxOutbound + c.maxValidators + len(c.TrustedPeerIDs)
}
func (p *P2P) ReceiveChannel(topic lib.Topic) chan *lib.MessageWrapper { return p.channels[topic] }
func (p *P2P) Close()                                                  { _ = p.listener.Close() }
func (p *P2P) filter(conn net.Conn) (*lib.PeerAddress, lib.ErrorI) {
	remoteAddr := conn.RemoteAddr()
	tcpAddr, ok := remoteAddr.(*net.TCPAddr)
	if !ok {
		return nil, ErrNonTCPAddress()
	}
	host := tcpAddr.IP.String()
	ips, err := net.DefaultResolver.LookupIPAddr(context.Background(), host)
	if err != nil {
		return nil, ErrIPLookup(err)
	}
	for _, ip := range ips {
		for _, bannedIP := range p.config.bannedIPs {
			if ip.IP.Equal(bannedIP.IP) {
				return nil, ErrBannedIP(ip.String())
			}
		}
	}
	return &lib.PeerAddress{NetAddress: net.JoinHostPort(host, fmt.Sprintf("%d", tcpAddr.Port))}, nil
}
func (p *P2P) catchPanic() {
	if r := recover(); r != nil {
		fmt.Println("recovered") // handle error
	}
}

type State struct {
	sync.RWMutex
	validators []*lib.PeerAddress
}

type Config struct {
	ListenAddress   string       // listen for incoming connection
	ExternalAddress string       // advertise for external dialing
	MaxInbound      int          // max inbound peers
	MaxOutbound     int          // max outbound peers
	TrustedPeerIDs  []string     // trusted public keys
	DialPeers       []string     // peers to consistently dial (format pubkey@ip:port)
	BannedPeerIDs   []string     // banned public keys
	BannedIPs       []string     // banned IPs
	bannedIPs       []net.IPAddr // banned IPs (non-string)
	maxValidators   int
}
