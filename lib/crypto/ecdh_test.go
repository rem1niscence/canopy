package crypto

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSharedSecret(t *testing.T) {
	p1, err := NewEd25519PrivateKey()
	require.NoError(t, err)
	p2, err := NewEd25519PrivateKey()
	require.NoError(t, err)
	sharedSecret, err := SharedSecret(p2.PublicKey().Bytes(), p1.Bytes())
	require.NoError(t, err)
	sharedSecret2, err := SharedSecret(p1.PublicKey().Bytes(), p2.Bytes())
	require.NoError(t, err)
	require.Equal(t, sharedSecret, sharedSecret2)
}
