package crypto

import (
	"bytes"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSharedSecret(t *testing.T) {
	// generate a standard ed25519 private key
	p1, err := NewEd25519PrivateKey()
	require.NoError(t, err)
	// generate a second ed25519 private key
	p2, err := NewEd25519PrivateKey()
	require.NoError(t, err)
	// generate a shared secret from 1 direction
	sharedSecret, err := SharedSecret(p2.PublicKey().Bytes(), p1.Bytes())
	require.NoError(t, err)
	// generate a shared secret from the other direction
	sharedSecret2, err := SharedSecret(p1.PublicKey().Bytes(), p2.Bytes())
	require.NoError(t, err)
	// ensure they're the same secret
	require.Equal(t, sharedSecret, sharedSecret2)
}

func TestECDH(t *testing.T) {
	// generate a few keys for testing
	p1, e := NewEd25519PrivateKey()
	require.NoError(t, e)
	p2, e := NewEd25519PrivateKey()
	require.NoError(t, e)
	p3, e := NewEd25519PrivateKey()
	require.NoError(t, e)
	// predefine a params structure to test with
	type params struct {
		selfPrivateKey []byte
		peerPublicKey  []byte
	}
	// define test cases
	tests := []struct {
		name    string
		detail  string
		peerA   params
		peerB   params
		error   string
		success bool
	}{
		{
			name:   "empty peer pub",
			detail: "the peer public key is empty",
			peerA: params{
				selfPrivateKey: p1.Bytes(),
				peerPublicKey:  nil,
			},
			error: "length",
		},
		{
			name:   "invalid peer pub key length",
			detail: "the peer public key is the wrong size",
			peerA: params{
				selfPrivateKey: p1.Bytes(),
				peerPublicKey:  p2.PublicKey().Address().Bytes(),
			},
			error: "length",
		},
		{
			name:   "wrong public key",
			detail: "the peer A has the wrong public key for peer B",
			peerA: params{
				selfPrivateKey: p1.Bytes(),
				peerPublicKey:  p3.PublicKey().Bytes(),
			},
			peerB: params{
				selfPrivateKey: p2.Bytes(),
				peerPublicKey:  p1.PublicKey().Bytes(),
			},
			success: false,
		},
		{
			name:   "success",
			detail: "the input parameters are correct",
			peerA: params{
				selfPrivateKey: p1.Bytes(),
				peerPublicKey:  p2.PublicKey().Bytes(),
			},
			peerB: params{
				selfPrivateKey: p2.Bytes(),
				peerPublicKey:  p1.PublicKey().Bytes(),
			},
			success: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// execute the function for peer A
			secret, err := SharedSecret(test.peerA.peerPublicKey, test.peerA.selfPrivateKey)
			require.Equal(t, err != nil, test.error != "")
			if err != nil {
				require.ErrorContains(t, err, test.error)
				return
			}
			// execute the function for peer B
			secret2, err := SharedSecret(test.peerB.peerPublicKey, test.peerB.selfPrivateKey)
			require.NoError(t, err)
			// check for expected result
			require.Equal(t, test.success, bytes.Equal(secret, secret2))
		})
	}
}
