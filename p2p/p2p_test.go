package p2p

import (
	"errors"
	"fmt"
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
		require.NoError(t, n1.SendTo(n2.pub, lib.Topic_TX, &PeerBookRequestMessage{}))
		require.NoError(t, n1.SendTo(n2.pub, lib.Topic_CONSENSUS, &PeerBookResponseMessage{Book: []*BookPeer{expectedMsg}}))
		time.AfterFunc(testTimeout, func() { panic("timeout") })
	}()
	<-n2.ReceiveChannel(lib.Topic_TX)
	msg := <-n2.ReceiveChannel(lib.Topic_CONSENSUS)
	gotMsg, ok := msg.Message.(*PeerBookResponseMessage)
	require.True(t, ok)
	require.True(t, len(gotMsg.Book) == 1)
	require.Equal(t, expectedMsg.Address.NetAddress, gotMsg.Book[0].Address.NetAddress)
	require.Equal(t, expectedMsg.Address.PublicKey, gotMsg.Book[0].Address.PublicKey)
	require.Equal(t, expectedMsg.ConsecutiveFailedDial, gotMsg.Book[0].ConsecutiveFailedDial)
}

func TestSendToRand(t *testing.T) {
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
		peerInfo, err := n1.SendToRandPeer(lib.Topic_CONSENSUS, &PeerBookResponseMessage{Book: []*BookPeer{expectedMsg}})
		require.NoError(t, err)
		require.Equal(t, peerInfo.Address.PublicKey, n2.pub)
		time.AfterFunc(testTimeout, func() { panic("timeout") })
	}()
	msg := <-n2.ReceiveChannel(lib.Topic_CONSENSUS)
	gotMsg, ok := msg.Message.(*PeerBookResponseMessage)
	require.True(t, ok)
	require.True(t, len(gotMsg.Book) == 1)
	require.Equal(t, expectedMsg.Address.NetAddress, gotMsg.Book[0].Address.NetAddress)
	require.Equal(t, expectedMsg.Address.PublicKey, gotMsg.Book[0].Address.PublicKey)
	require.Equal(t, expectedMsg.ConsecutiveFailedDial, gotMsg.Book[0].ConsecutiveFailedDial)
}

func TestSendToAll(t *testing.T) {
	n1, n2, cleanup := newTestP2PPair(t)
	n3 := newStartedTestP2PNode(t)
	connectStartedNodes(t, n1, n3)
	defer func() { n3.Stop(); cleanup() }()
	expectedMsg := &BookPeer{
		Address: &lib.PeerAddress{
			PublicKey:  n1.pub,
			NetAddress: "pipe",
		},
		ConsecutiveFailedDial: 1,
	}
	go func() {
		require.NoError(t, n1.SendToAll(lib.Topic_CONSENSUS, &PeerBookResponseMessage{Book: []*BookPeer{expectedMsg}}))
		time.AfterFunc(testTimeout, func() { panic("timeout") })
	}()
	msg := <-n2.ReceiveChannel(lib.Topic_CONSENSUS)
	gotMsg, ok := msg.Message.(*PeerBookResponseMessage)
	require.True(t, ok)
	require.True(t, len(gotMsg.Book) == 1)
	require.Equal(t, expectedMsg.Address.NetAddress, gotMsg.Book[0].Address.NetAddress)
	require.Equal(t, expectedMsg.Address.PublicKey, gotMsg.Book[0].Address.PublicKey)
	require.Equal(t, expectedMsg.ConsecutiveFailedDial, gotMsg.Book[0].ConsecutiveFailedDial)
	msg = <-n3.ReceiveChannel(lib.Topic_CONSENSUS)
	gotMsg, ok = msg.Message.(*PeerBookResponseMessage)
	require.True(t, ok)
	require.True(t, len(gotMsg.Book) == 1)
	require.Equal(t, expectedMsg.Address.NetAddress, gotMsg.Book[0].Address.NetAddress)
	require.Equal(t, expectedMsg.Address.PublicKey, gotMsg.Book[0].Address.PublicKey)
	require.Equal(t, expectedMsg.ConsecutiveFailedDial, gotMsg.Book[0].ConsecutiveFailedDial)
}

func TestSendToValidators(t *testing.T) {
	n1, n2, cleanup := newTestP2PPair(t)
	n1.UpdateValidators(n1.pub, []*lib.PeerAddress{{PublicKey: n2.pub}})
	n3 := newStartedTestP2PNode(t)
	connectStartedNodes(t, n1, n3)
	defer func() { n3.Stop(); cleanup() }()
	expectedMsg := &BookPeer{
		Address: &lib.PeerAddress{
			PublicKey:  n1.pub,
			NetAddress: "pipe",
		},
		ConsecutiveFailedDial: 1,
	}
	go func() {
		require.NoError(t, n1.SendToValidators(&PeerBookResponseMessage{Book: []*BookPeer{expectedMsg}}))
		time.AfterFunc(testTimeout, func() { panic("timeout") })
	}()
	msg := <-n2.ReceiveChannel(lib.Topic_CONSENSUS)
	gotMsg, ok := msg.Message.(*PeerBookResponseMessage)
	require.True(t, ok)
	require.True(t, len(gotMsg.Book) == 1)
	require.Equal(t, expectedMsg.Address.NetAddress, gotMsg.Book[0].Address.NetAddress)
	require.Equal(t, expectedMsg.Address.PublicKey, gotMsg.Book[0].Address.PublicKey)
	require.Equal(t, expectedMsg.ConsecutiveFailedDial, gotMsg.Book[0].ConsecutiveFailedDial)
	for {
		select {
		case <-n3.ReceiveChannel(lib.Topic_CONSENSUS):
			t.Fatal("unexpected message received")
		case <-time.After(200 * time.Millisecond):
			return
		}
	}
}

func TestDialReceive(t *testing.T) {
	n1, n2 := newStartedTestP2PNode(t), newStartedTestP2PNode(t)
	defer func() { n1.Stop(); n2.Stop() }()
	connectStartedNodes(t, n1, n2)
}

func TestStart(t *testing.T) {
	n2, n3, n4 := newTestP2PNodeWithConfig(t, newTestP2PConfig(t), true), newTestP2PNodeWithConfig(t, newTestP2PConfig(t), true), newTestP2PNodeWithConfig(t, newTestP2PConfig(t), true)
	n3.log, n2.log = lib.NewNullLogger(), lib.NewNullLogger()
	startTestP2PNode(t, n2)
	startTestP2PNode(t, n3)
	startTestP2PNode(t, n4)
	c := newTestP2PConfig(t)
	// test dial peers
	c.DialPeers = []string{fmt.Sprintf("%s@%s", lib.BytesToString(n2.pub), n2.listener.Addr().String())}
	n1 := newTestP2PNodeWithConfig(t, c)
	// test churn process
	random, _ := crypto.NewBLSPublicKey()
	n1.book.Add(&BookPeer{
		Address: &lib.PeerAddress{
			PublicKey:  random.Bytes(),
			NetAddress: n4.listener.Addr().String(),
		},
		ConsecutiveFailedDial: MaxFailedDialAttempts - 1,
	})
	// test validator receiver
	n1.ValidatorsReceiver() <- []*lib.PeerAddress{{
		PublicKey:  n2.pub,
		NetAddress: n2.listener.Addr().String(),
	}}
	test := func() (ok bool, reason string) {
		peerInfo, _ := n1.GetPeerInfo(n2.pub)
		if peerInfo == nil {
			return false, "n2 not found"
		}
		if !peerInfo.IsValidator {
			return false, "n2 not validator"
		}
		if n1.book.Has(random.Bytes()) {
			return false, "n1 did not churn peer book"
		}
		n3PI, _ := n1.GetPeerInfo(n3.pub)
		if n3PI == nil {
			return false, "n3 not found"
		}
		if n3PI.IsOutbound {
			return false, "n3 incorrectly marked as outbound"
		}
		return true, ""
	}
	startTestP2PNode(t, n1)
	defer func() { n1.Stop(); n2.Stop(); n3.Stop() }()
	// test listener
	require.NoError(t, n3.Dial(&lib.PeerAddress{
		PublicKey:  n1.pub,
		NetAddress: n1.listener.Addr().String(),
	}, false))
	for {
		select {
		default:
			if ok, _ := test(); ok {
				return
			}
		case <-time.After(testTimeout):
			_, reason := test()
			t.Fatal(reason)
		}
	}
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
		case <-time.After(testTimeout):
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

func TestSelfSend(t *testing.T) {
	topic := lib.Topic_CONSENSUS
	n := newStartedTestP2PNode(t)
	expected := (&lib.MessageWrapper{
		Message: &PeerBookRequestMessage{},
		Sender: &lib.PeerInfo{
			Address: &lib.PeerAddress{
				PublicKey:  n.pub,
				NetAddress: "",
			},
		},
	}).WithHash()
	require.NoError(t, n.SelfSend(n.pub, topic, &PeerBookRequestMessage{}))
	for {
		select {
		case msg := <-n.ReceiveChannel(topic):
			require.Equal(t, msg.Sender.Address.PublicKey, n.pub)
			require.Equal(t, msg.Hash, expected.Hash)
			return
		case <-time.After(testTimeout):
			t.Fatal("timeout")
		}
	}
}

func TestOnPeerError(t *testing.T) {
	n1, n2, cleanup := newTestP2PPair(t)
	defer cleanup()
	_, found := n1.book.getIndex(n2.pub)
	require.True(t, found)
	peer, err := n1.PeerSet.get(n2.pub)
	require.NoError(t, err)
	n1.OnPeerError(errors.New(""), n2.pub, "")
	_, found = n1.book.getIndex(n2.pub)
	require.False(t, found)
	_, err = n1.PeerSet.get(n2.pub)
	require.Error(t, err)
	_, e := peer.conn.conn.Read(make([]byte, 8))
	require.Error(t, e)
}

func TestNewStreams(t *testing.T) {
	n1, n2, cleanup := newTestP2PPair(t)
	defer cleanup()
	streams := n1.NewStreams()
	peer, err := n1.PeerSet.get(n2.pub)
	require.NoError(t, err)
	for i, s := range streams {
		ps := peer.conn.streams[i]
		require.Equal(t, ps.topic, s.topic)
		require.Equal(t, ps.receive, s.receive)
	}
}

func TestIsSelf(t *testing.T) {
	n1, n2 := newTestP2PNode(t), newTestP2PNode(t)
	require.True(t, n1.IsSelf(&lib.PeerAddress{PublicKey: n1.pub}))
	require.False(t, n1.IsSelf(&lib.PeerAddress{PublicKey: n2.pub}))
	require.True(t, n2.IsSelf(&lib.PeerAddress{PublicKey: n2.pub}))
	require.False(t, n2.IsSelf(&lib.PeerAddress{PublicKey: n1.pub}))
}

func TestID(t *testing.T) {
	n := newTestP2PNode(t)
	want := &lib.PeerAddress{
		PublicKey:  n.pub,
		NetAddress: n.config.ExternalAddress,
	}
	got := n.ID()
	require.Equal(t, want.PublicKey, got.PublicKey)
	require.Equal(t, want.NetAddress, got.NetAddress)
}

func connectStartedNodes(t *testing.T, n1, n2 testP2PNode) {
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
		case <-time.After(testTimeout):
			t.Fatal("timeout")
		}
	}
	peer, err = n2.PeerSet.GetPeerInfo(n1.pub)
	require.NoError(t, err)
	require.False(t, peer.IsOutbound)
}

func newStartedTestP2PNode(t *testing.T) testP2PNode {
	n := newTestP2PNode(t)
	return startTestP2PNode(t, n)
}

func startTestP2PNode(t *testing.T, n testP2PNode) testP2PNode {
	n.Start()
	for {
		select {
		default:
			if n.listener != nil {
				return n
			}
		case <-time.After(testTimeout):
			t.Fatal("timeout")
		}
	}
}

func newTestP2PPair(t *testing.T) (n1, n2 testP2PNode, cleanup func()) {
	n1, n2 = newTestP2PNode(t), newTestP2PNode(t)
	c1, c2 := net.Pipe()
	pipeTO := time.Now().Add(time.Second)
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
	return newTestP2PNodeWithConfig(t, newTestP2PConfig(t))
}

func newTestP2PNodeWithConfig(t *testing.T, c lib.Config, noLog ...bool) (n testP2PNode) {
	var err error
	n.priv, err = crypto.NewBLSPrivateKey()
	require.NoError(t, err)
	n.pub = n.priv.PublicKey().Bytes()
	require.NoError(t, err)
	logger := lib.NewDefaultLogger()
	if len(noLog) == 1 && noLog[0] == true {
		logger = lib.NewNullLogger()
	}
	n.P2P = New(n.priv, 1, c, logger)
	return
}

func newTestP2PConfig(_ *testing.T) lib.Config {
	config := lib.DefaultConfig()
	config.ListenAddress = ":0"
	return config
}
