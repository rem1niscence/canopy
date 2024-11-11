package crypto

import (
	"github.com/drand/kyber"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestBLS(t *testing.T) {
	// generate a message to test with
	msg := []byte("hello world")
	// create a new bls private key
	k1, err := NewBLS12381PrivateKey()
	require.NoError(t, err)
	// create a second bls private key
	k2, err := NewBLS12381PrivateKey()
	require.NoError(t, err)
	// create a third bls private key
	k3, err := NewBLS12381PrivateKey()
	require.NoError(t, err)
	// organize the 3 keys in a list
	publicKeys := [][]byte{k1.PublicKey().Bytes(), k2.PublicKey().Bytes(), k3.PublicKey().Bytes()}
	// convert the keys to kyber points and save to a list
	var points []kyber.Point
	for _, bz := range publicKeys {
		point, e := BytesToBLS12381Point(bz)
		require.NoError(t, e)
		points = append(points, point)
	}
	// generate a new multi-public key from that list
	multiKey, err := NewMultiBLSFromPoints(points, nil)
	require.NoError(t, err)
	// sign the message with the first private key
	k1Sig := k1.Sign(msg)
	// sign the message with the third private key
	k3Sig := k3.Sign(msg)
	// update the bitmap with those who signed and their respective indices
	require.NoError(t, multiKey.AddSigner(k1Sig, 0))
	require.NoError(t, multiKey.AddSigner(k3Sig, 2))
	// ensure signer 1 was enabled
	enabled, err := multiKey.SignerEnabledAt(0)
	require.NoError(t, err)
	require.True(t, enabled)
	// ensure signer 2 was disabled
	enabled, err = multiKey.SignerEnabledAt(1)
	require.NoError(t, err)
	require.False(t, enabled)
	// ensure signer 3 was enabled
	enabled, err = multiKey.SignerEnabledAt(2)
	require.NoError(t, err)
	require.True(t, enabled)
	// aggregate the signature
	sig, err := multiKey.AggregateSignatures()
	require.NoError(t, err)
	// ensure that a +2/3rds majority passes
	require.True(t, multiKey.VerifyBytes(msg, sig))
}

func TestNewBLSPointFromBytes(t *testing.T) {
	k1, err := NewBLS12381PrivateKey()
	require.NoError(t, err)
	k1Pub := k1.PublicKey().(*BLS12381PublicKey)
	point := k1Pub.Point
	bytes := k1Pub.Bytes()
	point2, err := BytesToBLS12381Point(bytes)
	require.NoError(t, err)
	require.True(t, point.Equal(point2))
}
