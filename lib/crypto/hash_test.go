package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestHashAndString(t *testing.T) {
	// generate arbitrary data
	msg := make([]byte, 100)
	_, err := rand.Read(msg)
	require.NoError(t, err)
	// hash the data using the hasher
	hasher := Hasher()
	_, err = hasher.Write(msg)
	require.NoError(t, err)
	byHasher := hasher.Sum(nil)
	// hash the data directly
	hash := Hash(msg)
	// check equivalence
	require.Equal(t, hash, byHasher)
	// ensure size is correct
	require.Len(t, hash, HashSize)
	// validate string
	require.Equal(t, hex.EncodeToString(hash), HashString(msg))
}
