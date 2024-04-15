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
	- Peer list: discover, churn, share
	- Message dissemination: rain tree
*/

// TODO share peers and remove height excchange from P2P and move it to sync message (block + maxHeight)

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
	PeerSet

	state State
}

func NewP2P(p crypto.PrivateKeyI, height uint64, maxValidators int, channels lib.Channels, c Config) *P2P {
	c.maxValidators = maxValidators
	return &P2P{
		privateKey: p,
		channels:   channels,
		config:     c,
		PeerSet: PeerSet{
			RWMutex: sync.RWMutex{},
			config:  c,
			m:       make(map[string]*Peer),
		},
		state: State{
			RWMutex: sync.RWMutex{},
			height:  height,
		},
	}
}

func (p *P2P) Start(validatorsReceiver chan []lib.PeerInfo, heightReceiver chan uint64) {
	go p.InternalListenValidators(validatorsReceiver)
	go p.InternalListenHeight(heightReceiver)
	go p.Listen(lib.PeerInfo{NetAddress: p.config.ListenAddress})
	for _, peer := range p.config.DialPeers {
		pi := new(lib.PeerInfo)
		if err := pi.FromString(peer); err != nil {
			// log error
			continue
		}
		p.DialWithBackoff(*pi)
	}
}

func (p *P2P) Dial(peerInfo lib.PeerInfo) lib.ErrorI {
	if p.IsSelf(peerInfo) {
		return nil
	}
	conn, er := net.DialTimeout(transport, peerInfo.NetAddress, dialTimeout)
	if er != nil {
		return ErrFailedDial(er)
	}
	peerInfo.IsOutbound = true
	return p.AddPeer(conn, peerInfo)
}

func (p *P2P) Listen(listenAddress lib.PeerInfo) {
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
			peerInfo, e := p.filter(c)
			if e != nil {
				_ = c.Close()
				return
			}
			if err = p.AddPeer(c, peerInfo); err != nil {
				_ = c.Close()
				return
			}
		}(c)
	}
}

func (p *P2P) AddPeer(conn net.Conn, info lib.PeerInfo) lib.ErrorI {
	connection, err := NewConnection(conn, p.NewStreams(), p, p.OnPeerError, p.privateKey)
	if err != nil {
		return err
	}
	if info.PublicKey != nil && !bytes.Equal(connection.peerPublicKey, info.PublicKey) {
		return ErrMismatchPeerPublicKey(info.PublicKey, connection.peerPublicKey)
	}
	p.state.RLock()
	for _, v := range p.state.validators {
		if bytes.Equal(v.PublicKey, info.PublicKey) {
			info.IsValidator = true
			break
		}
	}
	p.state.RUnlock()
	info.PublicKey = connection.peerPublicKey
	streams := make(map[lib.Topic]*Stream)
	for i := lib.Topic(0); i < lib.Topic_INVALID; i++ {
		streams[i] = &Stream{
			topic:         i,
			sendQueue:     make(chan []byte, maxQueueSize),
			sendQueueSize: atomic.Int32{},
			receive:       p.ReceiveChannel(i),
		}
	}
	return p.PeerSet.Add(&Peer{
		conn:     connection,
		PeerInfo: info,
		stop:     sync.Once{},
	})
}

func (p *P2P) DialWithBackoff(peerInfo lib.PeerInfo) {
	_ = backoff.Retry(func() error { return p.Dial(peerInfo) }, backoff.NewExponentialBackOff())
}

func (p *P2P) InternalListenValidators(validatorsReceiver chan []lib.PeerInfo) {
	selfPubKey := p.privateKey.PublicKey().Bytes()
	for vs := range validatorsReceiver {
		p.state.Lock()
		p.state.validators = vs
		p.state.Unlock()
		for _, val := range p.UpdateValidators(selfPubKey, vs) {
			go p.DialWithBackoff(val)
		}
	}
}

func (p *P2P) InternalListenHeight(heightReceiver chan uint64) {
	for h := range heightReceiver {
		p.state.Lock()
		p.state.height = h
		p.state.Unlock()
	}
}

func (p *P2P) OnPeerError(publicKey []byte) {
	if err := p.PeerSet.Remove(publicKey); err != nil {
		// handle error
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
func (p *P2P) IsSelf(i lib.PeerInfo) bool {
	return bytes.Equal(p.privateKey.PublicKey().Bytes(), i.PublicKey)
}
func (p *P2P) ReceiveChannel(topic lib.Topic) chan *lib.MessageWrapper { return p.channels[topic] }
func (p *P2P) Close()                                                  { _ = p.listener.Close() }
func (p *P2P) filter(conn net.Conn) (lib.PeerInfo, lib.ErrorI) {
	remoteAddr := conn.RemoteAddr()
	tcpAddr, ok := remoteAddr.(*net.TCPAddr)
	if !ok {
		return lib.PeerInfo{}, ErrNonTCPAddress()
	}
	host := tcpAddr.IP.String()
	ips, err := net.DefaultResolver.LookupIPAddr(context.Background(), host)
	if err != nil {
		return lib.PeerInfo{}, ErrIPLookup(err)
	}
	for _, ip := range ips {
		for _, bannedIP := range p.config.bannedIPs {
			if ip.IP.Equal(bannedIP.IP) {
				return lib.PeerInfo{}, ErrBannedIP(ip.String())
			}
		}
	}
	return lib.PeerInfo{NetAddress: net.JoinHostPort(host, fmt.Sprintf("%d", tcpAddr.Port))}, nil
}
func (p *P2P) catchPanic() {
	if r := recover(); r != nil {
		// handle error
	}
}

type State struct {
	sync.RWMutex
	validators []lib.PeerInfo
	height     uint64
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
