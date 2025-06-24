package crypto

import (
	"crypto/rand"
	"fmt"
	"github.com/dgraph-io/ristretto/v2"
	oasisEd25519 "github.com/oasisprotocol/curve25519-voi/primitives/ed25519"
	"sort"
	"sync"
)

var (
	DisableCache      = false
	SignatureCache, _ = ristretto.NewCache[string, struct{}](&ristretto.Config[string, struct{}]{
		NumCounters: 2_000_000, // 10x number of items
		MaxCost:     200_000,   // total cost (1 byte * 200K items)
		BufferItems: 64,        // recommended default
	})
)

// BatchVerifier is an efficient, multi-threaded, batch verifier for many common keys
type BatchVerifier struct {
	ed25519Index [8][]BatchTuple
	ethSecp256k1 [8][]BatchTuple
	secp256k1    [8][]BatchTuple
	bls12381     [8][]BatchTuple
	count        int
	noOp         bool // no op disables all batch verifier operations
}

// BatchTuple is a convenient structure to validate the batch
type BatchTuple struct {
	PublicKey PublicKeyI
	Message   []byte
	Signature []byte
	index     int
}

// NewBatchVerifier() constructs a batch verifier
func NewBatchVerifier(noOp ...bool) (b *BatchVerifier) {
	if noOp != nil {
		return &BatchVerifier{noOp: true}
	}
	capacity := 20_000
	b = new(BatchVerifier)
	for i := 0; i < 8; i++ {
		b.ed25519Index[i] = make([]BatchTuple, 0, capacity)
		b.ethSecp256k1[i] = make([]BatchTuple, 0, capacity)
		b.secp256k1[i] = make([]BatchTuple, 0, capacity)
		b.bls12381[i] = make([]BatchTuple, 0, capacity)
	}
	return
}

// Add() adds a tuple to the batch verifier
func (b *BatchVerifier) Add(pk PublicKeyI, publicKey, message, signature []byte) (err error) {
	if b.noOp {
		return nil
	}
	t := BatchTuple{PublicKey: pk, Message: message, Signature: signature, index: b.count}
	i := b.count % 8
	switch len(publicKey) {
	case Ed25519PubKeySize:
		b.ed25519Index[i] = append(b.ed25519Index[i], t)
	case ETHSECP256K1PubKeySize, ETHSECP256K1PubKeySize + 1:
		b.ethSecp256k1[i] = append(b.ethSecp256k1[i], t)
	case SECP256K1PubKeySize:
		b.secp256k1[i] = append(b.secp256k1[i], t)
	case BLS12381PubKeySize:
		b.bls12381[i] = append(b.bls12381[i], t)
	default:
		return fmt.Errorf("unrecognized public key format")
	}
	b.count++
	return
}

// Verify() returns the indices of bad signatures
func (b *BatchVerifier) Verify() []int {
	var wg sync.WaitGroup
	var mutex sync.Mutex
	var badIndices []int

	if b.noOp {
		return nil
	}

	for i := 0; i < 8; i++ {
		idx := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			if bad := b.verifyAll(idx); len(bad) > 0 {
				mutex.Lock()
				badIndices = append(badIndices, bad...)
				mutex.Unlock()
			}
		}()
	}
	wg.Wait()
	if len(badIndices) != 0 {
		// sort them
		sort.Ints(badIndices)
	}
	return badIndices
}

// verifyAll() verifies a group of signatures and returns a list of bad signatures
func (b *BatchVerifier) verifyAll(idx int) (badIndices []int) {
	verifyBatch := func(tuples []BatchTuple) {
		for _, tuple := range tuples {
			if ok := tuple.PublicKey.VerifyBytes(tuple.Message, tuple.Signature); ok {
				SignatureCache.Set(tuple.Key(), struct{}{}, 1)
			} else {
				badIndices = append(badIndices, tuple.index)
			}
		}
		return
	}
	// verify ed25519
	if len(b.ed25519Index[idx]) != 0 {
		verifier := oasisEd25519.NewBatchVerifier()
		for _, t := range b.ed25519Index[idx] {
			if _, found := SignatureCache.Get(t.Key()); !found {
				verifier.Add(t.PublicKey.Bytes(), t.Message, t.Signature)
			}
		}
		if !verifier.VerifyBatchOnly(rand.Reader) {
			// if batch verification fails, check each signature individually
			// TODO: use `Verify() bool, []bool` for an additional performance boost when there are bad signatures
			for _, t := range b.ed25519Index[idx] {
				if ok := t.PublicKey.VerifyBytes(t.Message, t.Signature); !ok {
					badIndices = append(badIndices, t.index)
				} else {
					SignatureCache.Set(t.Key(), struct{}{}, 1)
				}
			}
		} else {
			for _, t := range b.ed25519Index[idx] {
				SignatureCache.Set(t.Key(), struct{}{}, 1)
			}
		}
	}
	// verify ethSecp256k1
	verifyBatch(b.ethSecp256k1[idx])
	// verify secp256k1
	verifyBatch(b.secp256k1[idx])
	// verify bls12381
	verifyBatch(b.bls12381[idx])
	// all valid
	return
}

// Key() returns a unique string key for the cache
func (bt *BatchTuple) Key() string {
	pk := bt.PublicKey.Bytes()
	totalLen := len(pk) + len(bt.Message) + len(bt.Signature)
	b := make([]byte, totalLen)
	offset := 0
	copy(b[offset:], pk)
	offset += len(pk)
	copy(b[offset:], bt.Message)
	offset += len(bt.Message)
	copy(b[offset:], bt.Signature)
	return string(b)
}

// CheckCache() is a convenience function that checks the signature cache for a combination
// and returns a callback for the caller to easily add the signature to the cache
func CheckCache(pk PublicKeyI, msg, sig []byte) (found bool, addToCache func()) {
	if DisableCache {
		return false, func() {}
	}
	cacheTuple := BatchTuple{PublicKey: pk, Message: msg, Signature: sig}
	key := cacheTuple.Key()
	addToCache = func() { SignatureCache.Set(key, struct{}{}, 1) }
	_, found = SignatureCache.Get(key)
	return
}
