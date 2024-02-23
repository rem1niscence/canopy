package crypto

import (
	"crypto/sha256"
)

const (
	HashSize = sha256.Size
)

func Hash(bz []byte) []byte {
	h := sha256.Sum256(bz)
	return h[:]
}
