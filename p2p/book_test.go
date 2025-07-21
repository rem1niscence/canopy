package p2p

import (
	"github.com/canopy-network/canopy/lib"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGetRandom(t *testing.T) {
	n1, n2 := newTestP2PNode(t), newTestP2PNode(t)
	require.Nil(t, n1.book.GetRandom())
	require.Nil(t, n1.book.GetRandom())
	n1.book.Add(&BookPeer{Address: &lib.PeerAddress{
		PublicKey:  n2.pub,
		NetAddress: "",
		PeerMeta:   &lib.PeerMeta{ChainId: 1},
	}})
	got := n1.book.GetRandom()
	require.Equal(t, got.Address.PublicKey, n2.pub)
	got = n1.book.GetRandom()
	require.Equal(t, got.Address.PublicKey, n2.pub)
}

func TestGetAll(t *testing.T) {
	n1, n2, n3 := newTestP2PNode(t), newTestP2PNode(t), newTestP2PNode(t)
	require.Len(t, n1.book.GetAll(), 0)
	n2PeerAddress := &lib.PeerAddress{
		PublicKey:  n2.pub,
		NetAddress: "localhost:90001",
		PeerMeta:   &lib.PeerMeta{ChainId: 1},
	}
	n1.book.Add(&BookPeer{
		Address: n2PeerAddress,
	})
	got := n1.book.GetAll()
	require.Len(t, got, 1)
	require.Equal(t, got[0].Address.PublicKey, n2.pub)
	n3PeerAddress := &lib.PeerAddress{PublicKey: n3.pub, NetAddress: "localhost:90001", PeerMeta: &lib.PeerMeta{ChainId: 1}}
	n1.book.Add(&BookPeer{Address: n3PeerAddress})
	got = n1.book.GetAll()
	require.Len(t, got, 2)
	require.True(t, n1.book.Has(n3PeerAddress))
	require.True(t, n1.book.Has(n2PeerAddress))
}

func TestAddRemoveHas(t *testing.T) {
	n1, n2 := newTestP2PNode(t), newTestP2PNode(t)
	require.Len(t, n1.book.GetAll(), 0)
	n2PeerAddress := &lib.PeerAddress{PublicKey: n2.pub, PeerMeta: &lib.PeerMeta{ChainId: 1}, NetAddress: "localhost:90001"}
	n1.book.Add(&BookPeer{Address: n2PeerAddress})
	require.True(t, n1.book.Has(n2PeerAddress))
	n1.book.Remove(n2PeerAddress)
	require.False(t, n1.book.Has(n2PeerAddress))
}

func TestAddFailedDialAttempt(t *testing.T) {
	startConsecutiveFailedDialAttempt := int32(3)
	MaxFailedDialAttempts = 5
	n1, n2, n3 := newTestP2PNode(t), newTestP2PNode(t), newTestP2PNode(t)
	require.Len(t, n1.book.GetAll(), 0)
	n2PeerAddress := &lib.PeerAddress{PublicKey: n2.pub, PeerMeta: &lib.PeerMeta{ChainId: 1}, NetAddress: "localhost:90001"}
	n3PeerAddress := &lib.PeerAddress{PublicKey: n3.pub, PeerMeta: &lib.PeerMeta{ChainId: 1}, NetAddress: "localhost:90001"}
	n1.book.Add(&BookPeer{
		Address:               n2PeerAddress,
		ConsecutiveFailedDial: startConsecutiveFailedDialAttempt,
	})
	peer := n1.book.GetRandom()
	require.Equal(t, peer.Address.PublicKey, n2.pub)
	require.Equal(t, peer.ConsecutiveFailedDial, startConsecutiveFailedDialAttempt)
	n1.book.AddFailedDialAttempt(n3PeerAddress)
	peer = n1.book.GetRandom()
	require.Equal(t, peer.Address.PublicKey, n2.pub)
	require.Equal(t, peer.ConsecutiveFailedDial, startConsecutiveFailedDialAttempt)
	n1.book.AddFailedDialAttempt(n2PeerAddress)
	peer = n1.book.GetRandom()
	require.Equal(t, peer.Address.PublicKey, n2.pub)
	require.Equal(t, peer.ConsecutiveFailedDial, startConsecutiveFailedDialAttempt+1)
	n1.book.AddFailedDialAttempt(n2PeerAddress)
	require.False(t, n1.book.Has(n2PeerAddress))
}

func TestResetFailedDialAttempt(t *testing.T) {
	startConsecutiveFailedDialAttempt := int32(4)
	n1, n2 := newTestP2PNode(t), newTestP2PNode(t)
	require.Len(t, n1.book.GetAll(), 0)
	n2PeerAddress := &lib.PeerAddress{PublicKey: n2.pub, PeerMeta: &lib.PeerMeta{ChainId: 1}, NetAddress: "localhost:90001"}
	n1.book.Add(&BookPeer{
		Address:               n2PeerAddress,
		ConsecutiveFailedDial: startConsecutiveFailedDialAttempt,
	})
	peer := n1.book.GetRandom()
	require.Equal(t, peer.Address.PublicKey, n2.pub)
	require.Equal(t, peer.ConsecutiveFailedDial, startConsecutiveFailedDialAttempt)
	n1.book.ResetFailedDialAttempts(n2PeerAddress)
	require.True(t, n1.book.Has(n2PeerAddress))
	peer = n1.book.GetRandom()
	require.Equal(t, peer.Address.PublicKey, n2.pub)
	require.Equal(t, peer.ConsecutiveFailedDial, int32(0))
}

// Has() returns if the
func (p *PeerBook) Has(peerAddress *lib.PeerAddress) bool {
	p.l.Lock()
	defer p.l.Unlock()
	_, found := p.getIndex(peerAddress)
	return found
}
