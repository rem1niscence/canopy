package crypto

import (
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNewPublicKeyFromString(t *testing.T) {
	// pre-generate a secp256k1
	secp256k1Pk, err := secp256k1.GeneratePrivateKey()
	require.NoError(t, err)
	secp256k1Public, err := BytesToSECP256K1Public(secp256k1Pk.PubKey().SerializeCompressed())
	require.NoError(t, err)
	// pre-generate a ED25519
	ed25519Pk, err := NewEd25519PrivateKey()
	require.NoError(t, err)
	// pre-generate a BLS12381
	blsPrivateKey, err := NewBLS12381PrivateKey()
	require.NoError(t, err)
	tests := []struct {
		name     string
		string   string
		expected PublicKeyI
		error    string
	}{
		{
			name:   "not a recognized key",
			string: "abcd",
			error:  "unrecognized public key format",
		},
		{
			name:     "secp256k1 public key",
			string:   secp256k1Public.String(),
			expected: secp256k1Public,
		},
		{
			name:     "ed25519 public key",
			string:   ed25519Pk.PublicKey().String(),
			expected: ed25519Pk.PublicKey(),
		},
		{
			name:     "bls12381 public key",
			string:   blsPrivateKey.PublicKey().String(),
			expected: blsPrivateKey.PublicKey(),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// execute the function call
			got, e := NewPublicKeyFromString(test.string)
			// check if an error is expected or not
			require.Equal(t, test.error != "", e != nil)
			// check the error
			if e != nil {
				require.ErrorContains(t, e, test.error)
				return
			}
			// compare got vs expected
			require.EqualExportedValues(t, test.expected, got)
		})
	}
}

func TestNewPublicKeyFromBytes(t *testing.T) {
	// pre-generate a secp256k1
	secp256k1Pk, err := NewSECP256K1PrivateKey()
	require.NoError(t, err)
	// pre-generate a ED25519
	ed25519Pk, err := NewEd25519PrivateKey()
	require.NoError(t, err)
	// pre-generate a BLS12381
	blsPrivateKey, err := NewBLS12381PrivateKey()
	require.NoError(t, err)
	tests := []struct {
		name     string
		bytes    []byte
		expected PublicKeyI
		error    string
	}{
		{
			name:  "not a recognized key",
			bytes: []byte("abcd"),
			error: "unrecognized public key format",
		},
		{
			name:     "secp256k1 public key",
			bytes:    secp256k1Pk.PublicKey().Bytes(),
			expected: secp256k1Pk.PublicKey(),
		},
		{
			name:     "ed25519 public key",
			bytes:    ed25519Pk.PublicKey().Bytes(),
			expected: ed25519Pk.PublicKey(),
		},
		{
			name:     "bls12381 public key",
			bytes:    blsPrivateKey.PublicKey().Bytes(),
			expected: blsPrivateKey.PublicKey(),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// execute the function call
			got, e := NewPublicKeyFromBytes(test.bytes)
			// check if an error is expected or not
			require.Equal(t, test.error != "", e != nil)
			// check the error
			if e != nil {
				require.ErrorContains(t, e, test.error)
				return
			}
			// compare got vs expected
			require.EqualExportedValues(t, test.expected, got)
		})
	}
}

func TestNewPrivateKeyFromString(t *testing.T) {
	// pre-generate a ED25519
	ed25519Pk, err := NewEd25519PrivateKey()
	require.NoError(t, err)
	// pre-generate a BLS12381
	blsPrivateKey, err := NewBLS12381PrivateKey()
	require.NoError(t, err)
	tests := []struct {
		name     string
		string   string
		expected PrivateKeyI
		error    string
	}{
		{
			name:   "not a recognized key",
			string: "abcd",
			error:  "unrecognized private key format",
		},
		{
			name:     "ed25519 public key",
			string:   ed25519Pk.String(),
			expected: ed25519Pk,
		},
		{
			name:     "bls12381 public key",
			string:   blsPrivateKey.String(),
			expected: blsPrivateKey,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// execute the function call
			got, e := NewPrivateKeyFromString(test.string)
			// check if an error is expected or not
			require.Equal(t, test.error != "", e != nil)
			// check the error
			if e != nil {
				require.ErrorContains(t, e, test.error)
				return
			}
			// compare got vs expected
			require.EqualExportedValues(t, test.expected, got)
		})
	}
}

func TestNewPrivateKeyFromBytes(t *testing.T) {
	// pre-generate a ED25519
	ed25519Pk, err := NewEd25519PrivateKey()
	require.NoError(t, err)
	// pre-generate a BLS12381
	blsPrivateKey, err := NewBLS12381PrivateKey()
	require.NoError(t, err)
	tests := []struct {
		name     string
		bytes    []byte
		expected PrivateKeyI
		error    string
	}{
		{
			name:  "not a recognized key",
			bytes: []byte("abcd"),
			error: "unrecognized private key format",
		},
		{
			name:     "ed25519 public key",
			bytes:    ed25519Pk.Bytes(),
			expected: ed25519Pk,
		},
		{
			name:     "bls12381 public key",
			bytes:    blsPrivateKey.Bytes(),
			expected: blsPrivateKey,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// execute the function call
			got, e := NewPrivateKeyFromBytes(test.bytes)
			// check if an error is expected or not
			require.Equal(t, test.error != "", e != nil)
			// check the error
			if e != nil {
				require.ErrorContains(t, e, test.error)
				return
			}
			// compare got vs expected
			require.EqualExportedValues(t, test.expected, got)
		})
	}
}
