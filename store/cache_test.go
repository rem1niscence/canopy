package store

import (
	"bytes"
	"github.com/ginchuco/ginchu/types"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/storage"
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

func getAndAssert(t *testing.T, store types.KVStoreI, key, expectedValue []byte) {
	gotValue, err := store.Get(key)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotValue, expectedValue) {
		t.Fatalf("wanted %s got %s", string(expectedValue), string(gotValue))
	}
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

func bulkSetKV(t *testing.T, store types.KVStoreI, keyValue ...string) {
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
