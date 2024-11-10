package crypto

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"hash"
)

const (
	HashSize = sha256.Size
)

var (
	MaxHash = bytes.Repeat([]byte{0xFF}, HashSize)
)

// Hasher() returns the global hashing algorithm used
func Hasher() hash.Hash { return sha256.New() }

// Hash() executes the global hashing algorithm on input bytes
func Hash(msg []byte) []byte {
	h := sha256.Sum256(msg)
	return h[:]
}

// HashString() returns the hex byte version of a hash
func HashString(msg []byte) string { return hex.EncodeToString(Hash(msg)) }

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
			store[offset] = append(store[i], store[i]...)

		// normal case, parent = hash(concat(left, right))
		default:
			store[offset] = append(store[i], store[i+1]...)
		}
		offset++
	}
	return store[n-1], store, nil
}

// nextPowerOfTwo() calculates the smallest power of 2 that is greater than or equal to the input value
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
