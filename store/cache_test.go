package store

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"github.com/ginchuco/ginchu/types"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/storage"
	math "math/rand"
	"testing"
)

func TestCacheStoreBasic(t *testing.T) {
	parent := memoryLevelDB(t)
	cache := NewStoreCache(parent)
	key, val := []byte("key"), []byte("val")
	if err := cache.Set(key, val); err != nil {
		t.Fatal(err)
	}
	// parent get
	getAndAssert(t, parent, key, nil)
	// cache get
	getAndAssert(t, cache, key, val)
	// cache write and parent get
	cache.Write()
	getAndAssert(t, parent, key, val)
	// cache get
	getAndAssert(t, cache, key, val)
	// cache delete and parent get
	if err := cache.Delete(key); err != nil {
		t.Fatal(err)
	}
	getAndAssert(t, cache, key, nil)
	getAndAssert(t, parent, key, val)
	// cache write and parent get
	cache.Write()
	getAndAssert(t, parent, key, nil)
}

func getAndAssert(t *testing.T, store types.StoreI, key, expectedValue []byte) {
	gotValue, err := store.Get(key)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotValue, expectedValue) {
		t.Fatalf("wanted %s got %s", string(expectedValue), string(gotValue))
	}
}

func TestIteratorWithDelete(t *testing.T) {
	expectedVals := []string{"a", "b", "c", "d", "e", "f", "g"}
	parent := memoryLevelDB(t)
	cache := NewStoreCache(parent)
	bulkSetKV(t, cache, expectedVals...)
	for i := 0; i < 10; i++ {
		randomindex := math.Intn(len(expectedVals))
		if err := cache.Delete([]byte(expectedVals[randomindex])); err != nil {
			t.Fatal(err)
		}
		expectedVals = append(expectedVals[:randomindex], expectedVals[randomindex+1:]...)
		cIt, err := cache.Iterator(nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		validateIterators(t, expectedVals, cIt)
		add := make([]byte, 1)
		if _, err = rand.Read(add); err != nil {
			t.Fatal(err)
		}
		expectedVals = append(expectedVals, hex.EncodeToString(add))
	}
}

func TestDoubleCacheWrap(t *testing.T) {
	parent := memoryLevelDB(t)
	child := NewStoreCache(parent)
	grandchild := NewStoreCache(child)
	bulkSetKV(t, grandchild, "a")
	getAndAssert(t, grandchild, []byte("a"), []byte("a"))
	getAndAssert(t, child, []byte("a"), nil)
	getAndAssert(t, parent, []byte("a"), nil)
	grandchild.Write()
	getAndAssert(t, grandchild, []byte("a"), []byte("a"))
	getAndAssert(t, child, []byte("a"), []byte("a"))
	getAndAssert(t, parent, []byte("a"), nil)
	child.Write()
	getAndAssert(t, grandchild, []byte("a"), []byte("a"))
	getAndAssert(t, child, []byte("a"), []byte("a"))
	getAndAssert(t, parent, []byte("a"), []byte("a"))
}

func TestCacheIterator(t *testing.T) {
	parent := memoryLevelDB(t)
	cache := NewStoreCache(parent)
	parent2 := memoryLevelDB(t)
	expectedVals := []string{"a", "b", "c", "d", "e", "f", "g"}
	expectedValsReverse := []string{"g", "f", "e", "d", "c", "b", "a"}
	bulkSetKV(t, parent, "a", "c", "e", "g")
	bulkSetKV(t, cache, "b", "d", "f", "h")
	bulkSetKV(t, parent2, expectedVals...)
	if err := cache.Delete([]byte("h")); err != nil { // delete a
		t.Fatal(err)
	}
	// forward - ensure cache iterator matches behavior of normal iterator
	cIt, err := cache.Iterator(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cIt.Close()
	validateIterators(t, expectedVals, cIt)
	// backward - ensure cache iterator matches behavior of normal iterator
	cRIT, err := cache.ReverseIterator(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cRIT.Close()
	validateIterators(t, expectedValsReverse, cRIT)
}

func validateIterators(t *testing.T, expectedVals []string, iterators ...types.IteratorI) {
	for _, it := range iterators {
		for i := 0; it.Valid(); func() { i++; it.Next() }() {
			got, wanted := string(it.Key()), expectedVals[i]
			if wanted != got {
				t.Fatalf("wanted %s got %s", wanted, got)
			}
			got = string(it.Value())
			if wanted != got {
				t.Fatalf("wanted %s got %s", wanted, got)
			}
		}
	}
}

func bulkSetKV(t *testing.T, store types.StoreI, keyValue ...string) {
	for _, kv := range keyValue {
		if err := store.Set([]byte(kv), []byte(kv)); err != nil {
			t.Fatal(err)
		}
	}
}

func memoryLevelDB(t *testing.T) *LevelDBWrapper {
	db, err := leveldb.Open(storage.NewMemStorage(), nil)
	if err != nil {
		t.Fatal(newOpenStoreError(err).Error())
	}
	return &LevelDBWrapper{db}
}
