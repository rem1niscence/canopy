package crypto

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
)

const (
	HashSize = sha256.Size
)

func Hasher() hash.Hash {
	return sha256.New()
}

func Hash(bz []byte) []byte {
	h := sha256.Sum256(bz)
	return h[:]
}

func HashString(bz []byte) string {
	return hex.EncodeToString(Hash(bz))
}
