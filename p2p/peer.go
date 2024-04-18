package p2p

import (
	"bytes"
	"github.com/ginchuco/ginchu/lib"
	"google.golang.org/protobuf/proto"
	"sync"
)

const (
	MinimumPeerReputation = -10
)

type PeerSet struct {
	sync.RWMutex
	config   lib.P2PConfig
	m        map[string]*Peer // public key -> Peer
	inbound  int
	outbound int

	book *PeerBook
}

func NewPeerSet(c lib.Config, peerBook *PeerBook) PeerSet {
	return PeerSet{
		RWMutex: sync.RWMutex{},
		config:  c.P2PConfig,
		m:       make(map[string]*Peer),
		book:    peerBook,
	}
}

type Peer struct {
	conn *MultiConn
	*lib.PeerInfo
	stop sync.Once
}

func (ps *PeerSet) Add(p *Peer) (err lib.ErrorI) {
	ps.Lock()
	defer ps.Unlock()
	pubKey := lib.BytesToString(p.Address.PublicKey)
	if _, found := ps.m[pubKey]; found {
		return ErrPeerAlreadyExists(pubKey)
	}
	if !p.IsTrusted && !p.IsValidator {
		if p.IsOutbound && ps.config.MaxOutbound <= ps.outbound-1 {
			return ErrMaxOutbound()
		} else if !p.IsOutbound && ps.config.MaxInbound <= ps.inbound-1 {
			return ErrMaxInbound()
		}
	}
	if p.IsOutbound {
		ps.outbound++
	} else {
		ps.inbound++
	}
	ps.set(p)
	return nil
}

func (ps *PeerSet) UpdateValidators(selfPublicKey []byte, vs []*lib.PeerAddress) (toDial []*lib.PeerAddress) {
	ps.Lock()
	defer ps.Unlock()
	var selfIsValidator bool
	for _, val := range vs {
		if bytes.Equal(selfPublicKey, val.PublicKey) {
			selfIsValidator = true
		}
		publicKey := lib.BytesToString(val.PublicKey)
		v, found := ps.m[publicKey]
		if found {
			v.IsValidator = true
			ps.m[publicKey] = v
		} else {
			toDial = append(toDial, val)
		}
	}
	if !selfIsValidator {
		return nil
	}
	return
}

func (ps *PeerSet) Remove(publicKey []byte) lib.ErrorI {
	ps.Lock()
	defer ps.Unlock()
	peer, err := ps.get(publicKey)
	if err != nil {
		return err
	}
	peer.stop.Do(peer.conn.Stop)
	ps.del(publicKey)
	ps.book.Remove(publicKey)
	return err
}

func (ps *PeerSet) ChangeReputation(publicKey []byte, delta int32) {
	ps.Lock()
	defer ps.Unlock()
	if publicKey == nil { // self
		return
	}
	peer, err := ps.get(publicKey)
	if err != nil {
		return
	}
	peer.Reputation += delta
	if !peer.IsTrusted && !peer.IsValidator && peer.Reputation < MinimumPeerReputation {
		ps.del(peer.Address.PublicKey)
		return
	}
	ps.set(peer)
}

func (ps *PeerSet) GetPeerInfo(publicKey []byte) (*lib.PeerInfo, lib.ErrorI) {
	ps.RLock()
	defer ps.RUnlock()
	peer, err := ps.get(publicKey)
	if err != nil {
		return nil, err
	}
	return peer.PeerInfo.Copy(), nil
}

func (ps *PeerSet) SendToPeer(topic lib.Topic, msg proto.Message) (*lib.PeerInfo, lib.ErrorI) {
	ps.RLock()
	defer ps.RUnlock()
	for _, p := range ps.m {
		return p.Copy(), ps.send(p, topic, msg)
	}
	return nil, nil
}

func (ps *PeerSet) SendTo(publicKey []byte, topic lib.Topic, msg proto.Message) lib.ErrorI {
	ps.RLock()
	defer ps.RUnlock()
	peer, err := ps.get(publicKey)
	if err != nil {
		return err
	}
	return ps.send(peer, topic, msg)
}

func (ps *PeerSet) SendToAll(topic lib.Topic, msg proto.Message) lib.ErrorI {
	ps.RLock()
	defer ps.RUnlock()
	for _, p := range ps.m {
		if err := ps.send(p, topic, msg); err != nil {
			return err
		}
	}
	return nil
}

func (ps *PeerSet) SendToValidators(msg proto.Message) lib.ErrorI {
	ps.RLock()
	defer ps.RUnlock()
	for _, p := range ps.m {
		if p.IsValidator {
			if err := ps.send(p, lib.Topic_CONSENSUS, msg); err != nil {
				return err
			}
		}
	}
	return nil
}

func (ps *PeerSet) Has(publicKey []byte) bool {
	ps.RLock()
	defer ps.RUnlock()
	pubKey := lib.BytesToString(publicKey)
	_, found := ps.m[pubKey]
	return found
}

func (ps *PeerSet) send(peer *Peer, topic lib.Topic, msg proto.Message) lib.ErrorI {
	a, err := lib.ToAny(msg)
	if err != nil {
		return err
	}
	peer.conn.Send(topic, &Envelope{Payload: a})
	return nil
}

func (ps *PeerSet) Inbound() (inbound int) {
	ps.RLock()
	defer ps.RUnlock()
	return ps.inbound
}

func (ps *PeerSet) Outbound() (outbound int) {
	ps.RLock()
	defer ps.RUnlock()
	return ps.outbound
}

func (ps *PeerSet) set(p *Peer)          { ps.m[lib.BytesToString(p.Address.PublicKey)] = p }
func (ps *PeerSet) del(publicKey []byte) { delete(ps.m, lib.BytesToString(publicKey)) }
func (ps *PeerSet) get(publicKey []byte) (*Peer, lib.ErrorI) {
	pub := lib.BytesToString(publicKey)
	peer, ok := ps.m[pub]
	if !ok {
		return nil, ErrPeerNotFound(pub)
	}
	return peer, nil
}
