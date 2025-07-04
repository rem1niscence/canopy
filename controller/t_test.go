package controller

import (
	"bytes"
	"fmt"
	"github.com/canopy-network/canopy/bft"
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
	"github.com/canopy-network/canopy/p2p"
	"github.com/stretchr/testify/require"
	"net"
	"os"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"sync"
	"testing"
	"time"
)

func TestMultiSendBenchmark(t *testing.T) {
	// ✅ Enable profiling before workload
	runtime.SetMutexProfileFraction(1)
	runtime.SetBlockProfileRate(1)

	// ✅ Start trace before workload
	fTrace, _ := os.Create("trace.out")
	defer fTrace.Close()
	trace.Start(fTrace)
	defer trace.Stop()

	n1, n2, cleanup := newTestP2PPair(t)
	defer cleanup()

	s := time.Now()
	pk, err := crypto.NewBLS12381PrivateKey()
	require.NoError(t, err)

	signedMsg, err := signConsensusMessage(pk, &bft.Message{
		Qc: &lib.QuorumCertificate{Block: bytes.Repeat([]byte("F"), 83_500_000)},
	})
	require.NoError(t, err)

	go func() {
		require.NoError(t, n1.SendTo(n2.pub, lib.Topic_CONSENSUS, signedMsg))
	}()

	msg := <-n2.Inbox(lib.Topic_CONSENSUS)
	gotMsg, ok := msg.Message.(*lib.ConsensusMessage)
	require.True(t, ok)

	received := new(bft.Message)
	require.NoError(t, lib.Unmarshal(gotMsg.Message, received))

	fmt.Println("Elapsed:", time.Since(s))

	// ✅ Dump profiles *after* workload is done
	fMutex, _ := os.Create("mutex.prof")
	defer fMutex.Close()
	pprof.Lookup("mutex").WriteTo(fMutex, 0)

	fBlock, _ := os.Create("block.prof")
	defer fBlock.Close()
	pprof.Lookup("block").WriteTo(fBlock, 0)
}

func signConsensusMessage(privateKey crypto.PrivateKeyI, msg lib.Signable) (*lib.ConsensusMessage, lib.ErrorI) {
	// sign the message
	if err := msg.Sign(privateKey); err != nil {
		return nil, err
	}
	// convert the message to bytes
	messageBytes, err := lib.Marshal(msg)
	if err != nil {
		return nil, err
	}
	// wrap the message in consensus
	return &lib.ConsensusMessage{
		ChainId: 1,
		Message: messageBytes,
	}, nil
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
		require.NoError(t, n1.AddPeer(c2, &lib.PeerInfo{Address: &lib.PeerAddress{
			PublicKey:  n2.pub,
			NetAddress: c2.RemoteAddr().String(),
			PeerMeta: &lib.PeerMeta{
				ChainId: 0,
			},
		}}, false, true))
		wg.Done()
	}()
	require.NoError(t, n2.AddPeer(c1, &lib.PeerInfo{Address: &lib.PeerAddress{
		PublicKey:  n1.pub,
		NetAddress: c1.RemoteAddr().String(),
		PeerMeta:   &lib.PeerMeta{ChainId: 0},
	}},
		false, true))
	wg.Wait()
	require.True(t, n1.PeerSet.Has(n2.pub))
	require.True(t, n2.PeerSet.Has(n1.pub))
	return
}

type testP2PNode struct {
	*p2p.P2P
	priv crypto.PrivateKeyI
	pub  []byte
}

func newTestP2PNode(t *testing.T) (n testP2PNode) {
	return newTestP2PNodeWithConfig(t, newTestP2PConfig(t), true)
}

func newTestP2PNodeWithConfig(t *testing.T, c lib.Config, noLog ...bool) (n testP2PNode) {
	var err error
	n.priv, err = crypto.NewBLS12381PrivateKey()
	require.NoError(t, err)
	n.pub = n.priv.PublicKey().Bytes()
	require.NoError(t, err)
	logger := lib.NewDefaultLogger()
	if len(noLog) == 1 && noLog[0] == true {
		logger = lib.NewNullLogger()
	}
	n.P2P = p2p.New(n.priv, 1, nil, c, logger)
	return
}

func newTestP2PConfig(_ *testing.T) lib.Config {
	config := lib.DefaultConfig()
	config.ChainId = lib.CanopyChainId
	config.ListenAddress = ":0"
	return config
}
