package p2p

import (
	"github.com/ginchuco/ginchu/lib/crypto"
	"github.com/stretchr/testify/require"
	"net"
	"sync"
	"testing"
)

func TestHandshake(t *testing.T) {
	msg1, msg2 := []byte("foo"), []byte("bar")
	p1, err := crypto.NewBLSPrivateKey()
	require.NoError(t, err)
	p2, err := crypto.NewBLSPrivateKey()
	require.NoError(t, err)
	c1, c2 := net.Pipe()
	defer func() { c1.Close(); c2.Close() }()
	e1, e2 := new(EncryptedConn), new(EncryptedConn)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		e1, err = NewHandshake(c1, p1)
		wg.Done()
		require.NoError(t, err)
	}()
	e2, err = NewHandshake(c2, p2)
	require.NoError(t, err)
	wg.Wait()
	require.True(t, e1.peerPubKey.Equals(p2.PublicKey()))
	require.True(t, e2.peerPubKey.Equals(p1.PublicKey()))
	go func() {
		_, err = e1.Write(msg1)
		require.NoError(t, err)
	}()
	buff := make([]byte, 3)
	_, err = e2.Read(buff)
	require.NoError(t, err)
	require.Equal(t, msg1, buff)
	go func() {
		_, err = e2.Write(msg2)
		require.NoError(t, err)
	}()
	_, err = e1.Read(buff)
	require.NoError(t, err)
	require.Equal(t, msg2, buff)
}
