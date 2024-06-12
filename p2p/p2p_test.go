package p2p

import (
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"github.com/stretchr/testify/require"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	testTimeout = 3 * time.Second
)

func TestConnection(t *testing.T) {
	_, _, cleanup := newTestP2PPair(t)
	cleanup()
}

func TestMultiSendRec(t *testing.T) {
	n1, n2, cleanup := newTestP2PPair(t)
	defer cleanup()
	expectedMsg := &BookPeer{
		Address: &lib.PeerAddress{
			PublicKey:  n1.pub,
			NetAddress: "pipe",
		},
		ConsecutiveFailedDial: 1,
	}
	go func() {
		require.NoError(t, n1.SendTo(n2.pub, lib.Topic_PEERS_REQUEST, &PeerBookRequestMessage{}))
		require.NoError(t, n1.SendTo(n2.pub, lib.Topic_PEERS_RESPONSE, &PeerBookResponseMessage{Book: []*BookPeer{expectedMsg}}))
		time.AfterFunc(testTimeout, func() { panic("timeout") })
	}()
	<-n2.ReceiveChannel(lib.Topic_PEERS_REQUEST)
	msg := <-n2.ReceiveChannel(lib.Topic_PEERS_RESPONSE)
	gotMsg, ok := msg.Message.(*PeerBookResponseMessage)
	require.True(t, ok)
	require.True(t, len(gotMsg.Book) == 1)
	require.Equal(t, expectedMsg.Address.NetAddress, gotMsg.Book[0].Address.NetAddress)
	require.Equal(t, expectedMsg.Address.PublicKey, gotMsg.Book[0].Address.PublicKey)
	require.Equal(t, expectedMsg.ConsecutiveFailedDial, gotMsg.Book[0].ConsecutiveFailedDial)
}

func TestDialReceive(t *testing.T) {
	n1, n2 := newStartedTestP2PNode(t), newStartedTestP2PNode(t)
	defer func() { n1.Stop(); n2.Stop() }()
	require.NoError(t, n1.Dial(&lib.PeerAddress{
		PublicKey:  n2.pub,
		NetAddress: n2.listener.Addr().String(),
	}, false))
	peer, err := n1.PeerSet.GetPeerInfo(n2.pub)
	require.NoError(t, err)
	require.True(t, peer.IsOutbound)
o2:
	for {
		select {
		default:
			if n2.PeerSet.Has(n1.pub) {
				break o2
			}
		case <-time.NewTimer(testTimeout).C:
			t.Fatal("timeout")
		}
	}
	peer, err = n2.PeerSet.GetPeerInfo(n1.pub)
	require.NoError(t, err)
	require.False(t, peer.IsOutbound)
}

func TestDialDisconnect(t *testing.T) {
	n1, n2 := newStartedTestP2PNode(t), newStartedTestP2PNode(t)
	defer func() { n1.Stop(); n2.Stop() }()
	require.NoError(t, n1.DialAndDisconnect(&lib.PeerAddress{
		PublicKey:  n2.pub,
		NetAddress: n2.listener.Addr().String(),
	}))
	_, err := n1.PeerSet.GetPeerInfo(n2.pub)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "not found"))
}

func TestConnectValidator(t *testing.T) {
	n1, n2 := newStartedTestP2PNode(t), newStartedTestP2PNode(t)
	defer func() { n1.Stop(); n2.Stop() }()
	n1.ValidatorsReceiver() <- []*lib.PeerAddress{{
		PublicKey:  n2.pub,
		NetAddress: n2.listener.Addr().String(),
	}}
out:
	for {
		select {
		default:
			n1.state.RLock()
			numVals := len(n1.state.validators)
			n1.state.RUnlock()
			if numVals != 0 {
				break out
			}
		case <-time.NewTimer(testTimeout).C:
			t.Fatal("timeout")
		}
	}
	require.NoError(t, n1.Dial(&lib.PeerAddress{
		PublicKey:  n2.pub,
		NetAddress: n2.listener.Addr().String(),
	}, false))
	peer, err := n1.PeerSet.GetPeerInfo(n2.pub)
	require.NoError(t, err)
	require.True(t, peer.IsOutbound)
	require.True(t, peer.IsValidator)
}

func newStartedTestP2PNode(t *testing.T) testP2PNode {
	n := newTestP2PNode(t)
	n.Start()
	for {
		select {
		default:
			if n.listener != nil && n.listener.Addr() != nil {
				return n
			}
		case <-time.NewTimer(testTimeout).C:
			t.Fatal("timeout")
		}
	}
}

func newTestP2PPair(t *testing.T) (n1, n2 testP2PNode, cleanup func()) {
	n1, n2 = newTestP2PNode(t), newTestP2PNode(t)
	c1, c2 := net.Pipe()
	pipeTO := time.Now().Add(time.Second * 20)
	err := c1.SetReadDeadline(pipeTO)
	require.NoError(t, err)
	err = c2.SetReadDeadline(pipeTO)
	require.NoError(t, err)
	cleanup = func() { n1.Stop(); n2.Stop() }
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		require.NoError(t, n1.AddPeer(c2, &lib.PeerInfo{Address: &lib.PeerAddress{NetAddress: c1.RemoteAddr().String()}}, false))
		wg.Done()
	}()
	require.NoError(t, n2.AddPeer(c1, &lib.PeerInfo{Address: &lib.PeerAddress{NetAddress: c2.RemoteAddr().String()}}, false))
	wg.Wait()
	require.True(t, n1.PeerSet.Has(n2.pub))
	require.True(t, n2.PeerSet.Has(n1.pub))
	return
}

type testP2PNode struct {
	*P2P
	priv crypto.PrivateKeyI
	pub  []byte
}

func newTestP2PNode(t *testing.T) (n testP2PNode) {
	var err error
	n.priv, err = crypto.NewBLSPrivateKey()
	require.NoError(t, err)
	n.pub = n.priv.PublicKey().Bytes()
	require.NoError(t, err)
	n.P2P = New(n.priv, 1, newTestP2PConfig(t), lib.NewDefaultLogger())
	return
}

func newTestP2PConfig(_ *testing.T) lib.Config {
	config := lib.DefaultConfig()
	config.ListenAddress = ":0"
	return config
}
