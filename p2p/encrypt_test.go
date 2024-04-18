package p2p

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"github.com/ginchuco/ginchu/lib/crypto"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test(t *testing.T) {
	_, bobPrivKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	bobPubKey := bobPrivKey.Public()
	_, alicePrivKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	alicePubKey := alicePrivKey.Public()

	aliceSecret, err := crypto.SharedSecret(bobPubKey.(ed25519.PublicKey), alicePrivKey[:])
	require.NoError(t, err)
	bobSecret, err := crypto.SharedSecret(alicePubKey.(ed25519.PublicKey), bobPrivKey[:])
	require.NoError(t, err)
	require.Equal(t, aliceSecret, bobSecret)
	clearText := "hello world"
	encrypted, err := crypto.EncryptMessage(aliceSecret, clearText)
	require.NoError(t, err)
	text, err := crypto.DecryptMessage(bobSecret, encrypted)
	require.NoError(t, err)
	fmt.Println(text)
}
