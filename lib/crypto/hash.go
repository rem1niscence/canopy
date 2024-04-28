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

// MerkleTree creates a merkle tree from a slice of bytes. A
// linear slice was chosen since it uses about half as much memory as a tree
func MerkleTree(items [][]byte) (root []byte, store [][]byte, err error) {
	if len(items) == 0 {
		return []byte{}, [][]byte{}, nil
	}
	// Calculate how many entries are required to hold the binary merkle
	// tree as a linear array and create a slice of that size.
	offset := nextPowerOfTwo(len(items))
	n := offset*2 - 1
	store = make([][]byte, n)
	// Create the base hashes and populate the slice with them.
	for i, item := range items {
		store[i] = Hash(item)
	}
	// offset index = after the last transaction and adjusted to the next power of two.
	for i := 0; i < n-1; i += 2 {
		switch {
		// no left child, parent = nil
		case store[i] == nil:
			store[offset] = nil

		// no right child, parent = hash(concat(left, left))
		case store[i+1] == nil:
			store[offset] = hashMerkleNodes(store[i], store[i])

		// normal case, parent = hash(concat(left, right))
		default:
			store[offset] = hashMerkleNodes(store[i], store[i+1])
		}
		offset++
	}
	return store[n-1], store, nil
}

func hashMerkleNodes(left []byte, right []byte) []byte {
	var h [HashSize * 2]byte
	copy(h[:HashSize], left[:])
	copy(h[HashSize:], right[:])
	return Hash(h[:])
}

func nextPowerOfTwo(v int) int {
	v--
	v |= v >> 1
	v |= v >> 2
	v |= v >> 4
	v |= v >> 8
	v |= v >> 16
	v++
	return v
}
