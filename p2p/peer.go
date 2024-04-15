package p2p

import (
	"bytes"
	lib "github.com/ginchuco/ginchu/types"
	"google.golang.org/protobuf/proto"
	"sync"
)

type PeerSet struct {
	sync.RWMutex
	config    Config
	m         map[string]*Peer // public key -> Peer
	maxHeight uint64
	inbound   int
	outbound  int
}

type Peer struct {
	conn *MultiConn
	lib.PeerInfo
	stop sync.Once
}

func (ps *PeerSet) Add(p *Peer) (err lib.ErrorI) {
	ps.Lock()
	defer ps.Unlock()
	pubKey := lib.BytesToString(p.PublicKey)
	if _, found := ps.m[pubKey]; found {
		return ErrPeerAlreadyExists(pubKey)
	}
	var trusted bool
	for _, t := range ps.config.TrustedPeerIDs {
		if lib.BytesToString(p.PublicKey) == t {
			trusted = true
			break
		}
	}
	if !trusted && !p.IsValidator {
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
	ps.calculateMaxHeight()
	return nil
}

func (ps *PeerSet) UpdateValidators(selfPublicKey []byte, vs []lib.PeerInfo) (toDial []lib.PeerInfo) {
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
	ps.calculateMaxHeight()
	return err
}

func (ps *PeerSet) ChangeReputation(publicKey []byte, delta int32) {
	ps.Lock()
	defer ps.Unlock()
	peer, err := ps.get(publicKey)
	if err != nil {
		return
	}
	peer.Reputation += delta
	ps.set(peer)
}

func (ps *PeerSet) GetPeerInfo(publicKey []byte) (lib.PeerInfo, lib.ErrorI) {
	ps.RLock()
	defer ps.RUnlock()
	peer, err := ps.get(publicKey)
	if err != nil {
		return lib.PeerInfo{}, err
	}
	return peer.PeerInfo, nil
}

func (ps *PeerSet) SendToPeer(topic lib.Topic, msg proto.Message) (*lib.PeerInfo, lib.ErrorI) {
	ps.RLock()
	defer ps.RUnlock()
	for _, p := range ps.m {
		return &p.PeerInfo, ps.send(p, topic, msg)
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

func (ps *PeerSet) SendToValidators(msg proto.Message) lib.ErrorI {
	ps.RLock()
	defer ps.RUnlock()
	for _, p := range ps.m {
		if p.IsValidator {
			if err := ps.send(p, lib.Topic_CONSENSUS, msg); err != nil {
				// log error
			}
		}
	}
	return nil
}

func (ps *PeerSet) send(peer *Peer, topic lib.Topic, msg proto.Message) lib.ErrorI {
	a, err := lib.ToAny(msg)
	if err != nil {
		return err
	}
	peer.conn.Send(topic, lib.ProtoMessage{Payload: a})
	return nil
}

func (ps *PeerSet) GetPeerWithHeight(height uint64) *lib.PeerInfo {
	ps.RLock()
	defer ps.RUnlock()
	for _, p := range ps.m {
		if p.MaxHeight >= height && height >= p.MinHeight {
			return &p.PeerInfo
		}
	}
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

func (ps *PeerSet) GetMaxPeerHeight() uint64 {
	ps.RLock()
	defer ps.RUnlock()
	return ps.maxHeight
}

func (ps *PeerSet) del(publicKey []byte) { delete(ps.m, lib.BytesToString(publicKey)) }
func (ps *PeerSet) calculateMaxHeight() {
	ps.maxHeight = uint64(0)
	for _, p := range ps.m {
		if p.MaxHeight > ps.maxHeight {
			ps.maxHeight = p.MaxHeight
		}
	}
}
func (ps *PeerSet) get(publicKey []byte) (*Peer, lib.ErrorI) {
	pub := lib.BytesToString(publicKey)
	peer, ok := ps.m[pub]
	if !ok {
		return nil, ErrPeerNotFound(pub)
	}
	return peer, nil
}
func (ps *PeerSet) set(p *Peer) {
	ps.m[lib.BytesToString(p.PublicKey)] = p
	return
}
