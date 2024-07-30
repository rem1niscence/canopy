package p2p

import (
	"bytes"
	"context"
	"fmt"
	"github.com/cenkalti/backoff/v4"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"golang.org/x/net/netutil"
	"google.golang.org/protobuf/proto"
	"net"
	"runtime/debug"
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
	transport            = "tcp"
	dialTimeout          = time.Second
	defaultMaxValidators = 1000
)

type P2P struct {
	privateKey crypto.PrivateKeyI
	listener   net.Listener
	channels   lib.Channels
	config     lib.Config
	PeerSet              // active set
	book       *PeerBook // not active set

	state              State
	validatorsReceiver chan []*lib.PeerAddress
	maxValidators      int
	bannedIPs          []net.IPAddr // banned IPs (non-string)
	log                lib.LoggerI
}

func New(p crypto.PrivateKeyI, maxValidators uint64, c lib.Config, l lib.LoggerI) *P2P {
	channels := make(lib.Channels)
	for i := lib.Topic(0); i < lib.Topic_INVALID; i++ {
		channels[i] = make(chan *lib.MessageWrapper, maxChanSize)
	}
	peerBook := &PeerBook{
		RWMutex:  sync.RWMutex{},
		book:     make([]*BookPeer, 0),
		bookSize: 0,
		log:      l,
	} // TODO load from file / write to file
	if maxValidators == 0 {
		maxValidators = defaultMaxValidators
	}
	var bannedIPs []net.IPAddr
	for _, ip := range c.BannedIPs {
		i, err := net.ResolveIPAddr("", ip)
		if err != nil {
			l.Fatalf(err.Error())
		}
		bannedIPs = append(bannedIPs, *i)
	}
	return &P2P{
		privateKey:         p,
		channels:           channels,
		config:             c,
		PeerSet:            NewPeerSet(c, peerBook),
		book:               peerBook,
		state:              State{RWMutex: sync.RWMutex{}},
		validatorsReceiver: make(chan []*lib.PeerAddress, maxChanSize),
		maxValidators:      int(maxValidators),
		bannedIPs:          bannedIPs,
		log:                l,
	}
}

func (p *P2P) Start() {
	p.log.Info("Starting P2P 🤝 ")
	go p.InternalListenValidators(p.validatorsReceiver)
	go p.Listen(&lib.PeerAddress{NetAddress: p.config.ListenAddress})
	go p.StartPeerBookService()
	go p.StartDialService()
}

func (p *P2P) Stop() {
	if p.listener != nil {
		if err := p.listener.Close(); err != nil {
			p.log.Error(err.Error())
		}
	}
	p.PeerSet.Stop()
}

func (p *P2P) StartDialService() {
	go func() {
		for _, peer := range p.config.DialPeers {
			pi := new(lib.PeerAddress)
			if err := pi.FromString(peer); err != nil {
				p.log.Error(err.Error())
				continue
			}
			go p.DialWithBackoff(pi)
		}
	}()
	dialing := 0
	for range time.NewTicker(dialTimeout).C {
		if p.PeerSet.Outbound()+dialing >= p.config.MaxOutbound && p.GetBookSize() > 1 { // self
			continue
		}
		rand := p.book.GetRandom()
		if rand == nil || p.IsSelf(rand.Address) || p.Has(rand.Address.PublicKey) {
			continue
		}
		dialing++
		if err := p.Dial(rand.Address, false); err != nil {
			p.log.Warn(err.Error())
		}
		dialing--
	}
}

func (p *P2P) Dial(address *lib.PeerAddress, disconnect bool) lib.ErrorI {
	if p.IsSelf(address) || p.PeerSet.Has(address.PublicKey) {
		return nil
	}
	p.log.Debugf("Dialing %s@%s", lib.BytesToString(address.PublicKey), address.NetAddress)
	conn, er := net.DialTimeout(transport, address.NetAddress, dialTimeout)
	if er != nil {
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
	p.log.Debugf("Starting net.Listener on tcp://%s", listenAddress.NetAddress)
	p.listener = netutil.LimitListener(ln, p.config.MaxInbound+len(p.config.TrustedPeerIDs)+p.maxValidators)
	for {
		c, err := p.listener.Accept()
		if err != nil {
			return
		}
		go func(c net.Conn) {
			defer p.catchPanic()
			p.log.Debugf("Received ephemeral connection %s", c.RemoteAddr().String())
			peerAddress, e := p.filter(c)
			if e != nil {
				p.log.Debugf("Closing ephemeral connection %s", c.RemoteAddr().String())
				_ = c.Close()
				return
			}
			p.log.Debugf("Received ephemeral connection from %s@%s", lib.BytesToString(peerAddress.PublicKey), peerAddress.NetAddress)
			if err = p.AddPeer(c, &lib.PeerInfo{Address: peerAddress}, false); err != nil {
				_ = c.Close()
				return
			}
		}(c)
	}
}

func (p *P2P) AddPeer(conn net.Conn, info *lib.PeerInfo, disconnect bool) lib.ErrorI {
	connection, err := p.NewConnection(conn)
	if err != nil {
		return err
	}
	p.log.Debugf("Try Add peer: %s@%s", lib.BytesToString(connection.peerPublicKey), info.Address.NetAddress)
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
	p.log.Infof("Adding peer: %s@%s", lib.BytesToString(info.Address.PublicKey), info.Address.NetAddress)
	p.book.Add(&BookPeer{Address: info.Address})
	return p.PeerSet.Add(&Peer{
		conn:     connection,
		PeerInfo: info,
		stop:     sync.Once{},
	})
}

func (p *P2P) DialWithBackoff(peerInfo *lib.PeerAddress) {
	_ = backoff.Retry(func() error {
		err := p.Dial(peerInfo, false)
		if err != nil {
			p.log.Errorf("Dial %s@%s failed: %s", peerInfo.PublicKey, peerInfo.NetAddress, err.Error())
		}
		return err
	}, backoff.NewExponentialBackOff())
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

func (p *P2P) OnPeerError(err error, publicKey []byte, remoteAddr string) {
	p.log.Warn(PeerError(publicKey, remoteAddr, err))
	_ = p.PeerSet.Remove(publicKey) // peer may have disconnected before added
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
func (p *P2P) SelfSend(fromPublicKey []byte, topic lib.Topic, payload proto.Message) lib.ErrorI {
	p.log.Debugf("Self sending %s message", topic)
	p.ReceiveChannel(topic) <- (&lib.MessageWrapper{
		Message: payload,
		Sender: &lib.PeerInfo{
			Address: &lib.PeerAddress{
				PublicKey:  fromPublicKey,
				NetAddress: "",
			},
		},
	}).WithHash()
	return nil
}
func (p *P2P) MaxPossiblePeers() int {
	return p.config.MaxInbound + p.config.MaxOutbound + p.maxValidators + len(p.config.TrustedPeerIDs)
}
func (p *P2P) ReceiveChannel(topic lib.Topic) chan *lib.MessageWrapper { return p.channels[topic] }
func (p *P2P) ValidatorsReceiver() chan []*lib.PeerAddress             { return p.validatorsReceiver }
func (p *P2P) ID() *lib.PeerAddress {
	p.Lock()
	defer p.Unlock()
	return &lib.PeerAddress{
		PublicKey:  p.privateKey.PublicKey().Bytes(),
		NetAddress: p.config.ExternalAddress,
	}
}
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
		for _, bannedIP := range p.bannedIPs {
			if ip.IP.Equal(bannedIP.IP) {
				return nil, ErrBannedIP(ip.String())
			}
		}
	}
	return &lib.PeerAddress{NetAddress: net.JoinHostPort(host, fmt.Sprintf("%d", tcpAddr.Port))}, nil
}
func (p *P2P) catchPanic() {
	if r := recover(); r != nil {
		p.log.Error(string(debug.Stack()))
	}
}

type State struct {
	sync.RWMutex
	validators []*lib.PeerAddress
}
