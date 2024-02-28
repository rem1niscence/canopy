package store

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"github.com/ginchuco/ginchu/types"
	math "math/rand"
	"testing"
)

func TestStoreSetGetDelete(t *testing.T) {
	store := NewMemoryStore(types.NewDefaultLogger())
	key, val := []byte("key"), []byte("val")
	if err := store.Set(key, val); err != nil {
		t.Fatal(err)
	}
	gotVal, err := store.Get(key)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotVal, val) {
		t.Fatalf("wanted %s got %s", string(val), string(gotVal))
	}
	if err = store.Delete(key); err != nil {
		t.Fatal(err)
	}
	gotVal, err = store.Get(key)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(gotVal, val) {
		t.Fatalf("%s should be deleted", string(val))
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestIterator(t *testing.T) {
	parent := memoryLevelDB(t)
	defer parent.Close()
	expectedVals := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	expectedValsReverse := []string{"h", "g", "f", "e", "d", "c", "b", "a"}
	bulkSetKV(t, parent, expectedVals...)
	it, err := parent.Iterator(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer it.Close()
	validateIterators(t, expectedVals, it)
	rIt, err := parent.ReverseIterator(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer rIt.Close()
	validateIterators(t, expectedValsReverse, rIt)
}

func TestProof(t *testing.T) {
	store := NewMemoryStore(types.NewDefaultLogger())
	key, val := []byte("key"), []byte("val")
	if err := store.Set(key, val); err != nil {
		t.Fatal(err)
	}
	addRandomValues(t, store)
	proof, value, err := store.GetProof(key)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(val, value) {
		t.Fatalf("wanted %s got %s", hex.EncodeToString(val), hex.EncodeToString(val))
	}
	testProof(t, store, key, value, proof)
	// proof of non-inclusion
	nonInclusionKey := []byte("key2")
	proof, value, err = store.GetProof(nonInclusionKey)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(value, nil) {
		t.Fatal("wanted nil bytes for non-inclusion")
	}
	testProof(t, store, nonInclusionKey, value, proof)
}

func testProof(t *testing.T, store types.CommitStoreI, key, value, proof []byte) {
	if !store.VerifyProof(key, value, proof) {
		t.Fatal("expected valid proof")
	}
	if store.VerifyProof(key, []byte("other value"), proof) {
		t.Fatal("expected invalid proof")
	}
	if store.VerifyProof([]byte("other key"), value, proof) {
		t.Fatal("expected invalid proof")
	}
}

func addRandomValues(t *testing.T, store types.CommitStoreI) {
	for i := 0; i < math.Intn(1000); i++ {
		key := make([]byte, 256)
		if _, err := rand.Read(key); err != nil {
			t.Fatal(err)
		}
		value := make([]byte, 256)
		if _, err := rand.Read(value); err != nil {
			t.Fatal(err)
		}
		if err := store.Set(key, value); err != nil {
			t.Fatal(err)
		}
	}
}
