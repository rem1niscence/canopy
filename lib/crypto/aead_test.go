package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"github.com/stretchr/testify/require"
	"reflect"
	"testing"
)

func TestHKDFSecretsAndChallenge(t *testing.T) {
	// initialize two arbitrary shared secrets
	dhSecret := []byte("shared secret")
	// create two new public key object
	pub1, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	pub2, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	// execute the function call in 1 direction
	gotSend, gotReceive, gotChallenge, err := HKDFSecretsAndChallenge(dhSecret, pub1, pub2)
	require.NoError(t, err)
	// execute the function call in the other direction
	gotSend2, gotReceive2, gotChallenge2, err := HKDFSecretsAndChallenge(dhSecret, pub2, pub1)
	require.NoError(t, err)
	// test for counter equality
	require.Equal(t, gotSend, gotReceive2)
	require.Equal(t, gotSend2, gotReceive)
	// ensure the challenge is equal
	require.Equal(t, gotChallenge, gotChallenge2)
}

func TestAEAD(t *testing.T) {
	// predefine a params structure for testing
	type params struct {
		dhSecret            []byte
		ephemeralPubKey     []byte
		peerEphemeralPubKey []byte
	}
	// predefine a few public keys to test with
	pubA, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	pubB, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	// define test cases
	tests := []struct {
		name    string
		detail  string
		peerA   params
		peerB   params
		success bool
	}{
		{
			name:   "wrong diffie hellman secret",
			detail: "the dh secret is not the same for the two peers",
			peerA: params{
				dhSecret:            []byte("secret_a"),
				ephemeralPubKey:     pubA,
				peerEphemeralPubKey: pubB,
			},
			peerB: params{
				dhSecret:            []byte("secret_b"),
				ephemeralPubKey:     pubB,
				peerEphemeralPubKey: pubA,
			},
		},
		{
			name:   "wrong public key order",
			detail: "the public key ordering is wrong, peerB mixed up local and remote keys",
			peerA: params{
				dhSecret:            []byte("secret"),
				ephemeralPubKey:     pubA,
				peerEphemeralPubKey: pubB,
			},
			peerB: params{
				dhSecret:            []byte("secret"),
				ephemeralPubKey:     pubA,
				peerEphemeralPubKey: pubB,
			},
		},
		{
			name:   "happy path",
			detail: "the shared secret is the same and the keys are in the right order",
			peerA: params{
				dhSecret:            []byte("secret"),
				ephemeralPubKey:     pubA,
				peerEphemeralPubKey: pubB,
			},
			peerB: params{
				dhSecret:            []byte("secret"),
				ephemeralPubKey:     pubB,
				peerEphemeralPubKey: pubA,
			},
			success: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// execute the function call for peer A
			gotSend, gotReceive, gotChallenge, e := HKDFSecretsAndChallenge(test.peerA.dhSecret, test.peerA.ephemeralPubKey, test.peerA.peerEphemeralPubKey)
			require.NoError(t, e)
			// execute the function call for peer B
			gotSend2, gotReceive2, gotChallenge2, e := HKDFSecretsAndChallenge(test.peerB.dhSecret, test.peerB.ephemeralPubKey, test.peerB.peerEphemeralPubKey)
			require.NoError(t, e)
			// test for counter equality
			if test.success {
				require.Equal(t, gotSend, gotReceive2)
				require.Equal(t, gotSend2, gotReceive)
				// ensure the challenge is equal
				require.Equal(t, gotChallenge, gotChallenge2)
			} else {
				require.False(t, reflect.DeepEqual(gotSend, gotReceive2) && reflect.DeepEqual(gotSend2, gotReceive) && reflect.DeepEqual(gotChallenge, gotChallenge2))
			}
		})
	}
}
