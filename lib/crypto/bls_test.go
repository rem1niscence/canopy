package crypto

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestBLS(t *testing.T) {
	msg := []byte("hello world")
	k1, err := NewBLSPrivateKey()
	require.NoError(t, err)
	k1Sig := k1.Sign(msg)
	k2, err := NewBLSPrivateKey()
	require.NoError(t, err)
	k3, err := NewBLSPrivateKey()
	require.NoError(t, err)
	k3Sig := k3.Sign(msg)
	publicKeys := [][]byte{k1.PublicKey().Bytes(), k2.PublicKey().Bytes(), k3.PublicKey().Bytes()}
	multiKey, err := NewMultiBLS(publicKeys, nil)
	require.NoError(t, err)
	require.NoError(t, multiKey.AddSigner(k1Sig, 0))
	require.NoError(t, multiKey.AddSigner(k3Sig, 2))
	sig, err := multiKey.AggregateSignatures()
	require.NoError(t, err)
	require.True(t, multiKey.VerifyBytes(msg, sig))
	enabled, err := multiKey.SignerEnabledAt(0)
	require.NoError(t, err)
	require.True(t, enabled)
	enabled, err = multiKey.SignerEnabledAt(1)
	require.NoError(t, err)
	require.False(t, enabled)
	enabled, err = multiKey.SignerEnabledAt(2)
	require.NoError(t, err)
	require.True(t, enabled)
}

func TestNewBLSPointFromBytes(t *testing.T) {
	k1, err := NewBLSPrivateKey()
	require.NoError(t, err)
	k1Pub := k1.PublicKey().(*BLS12381PublicKey)
	point := k1Pub.Point
	bytes := k1Pub.Bytes()
	point2, err := NewBLSPointFromBytes(bytes)
	require.NoError(t, err)
	require.True(t, point.Equal(point2))
}

func Test(t *testing.T) {
	msg := []byte("hello world")
	k1, err := NewBLSPrivateKey()
	require.NoError(t, err)
	k1Sig := k1.Sign(msg)
	k2, err := NewBLSPrivateKey()
	require.NoError(t, err)
	k3, err := NewBLSPrivateKey()
	require.NoError(t, err)
	k3Sig := k3.Sign(msg)
	publicKeys := [][]byte{k1.PublicKey().Bytes(), k2.PublicKey().Bytes(), k3.PublicKey().Bytes()}
	multiKey, err := NewMultiBLS(publicKeys, nil)
	require.NoError(t, err)
	require.NoError(t, multiKey.AddSigner(k1Sig, 0))
	require.NoError(t, multiKey.AddSigner(k3Sig, 2))
	sig, err := multiKey.AggregateSignatures()
	require.NoError(t, err)
	require.True(t, multiKey.VerifyBytes(msg, sig))
	enabled, err := multiKey.SignerEnabledAt(0)
	require.NoError(t, err)
	require.True(t, enabled)
	enabled, err = multiKey.SignerEnabledAt(1)
	require.NoError(t, err)
	require.False(t, enabled)
	enabled, err = multiKey.SignerEnabledAt(2)
	require.NoError(t, err)
	require.True(t, enabled)
	fmt.Println(multiKey.(*BLS12381MultiPublicKey).mask.Len())

}
