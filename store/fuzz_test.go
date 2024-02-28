package store

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"github.com/ginchuco/ginchu/types"
	math "math/rand"
	"testing"
)

type TestingOp int

const (
	SetTesting TestingOp = iota
	DelTesting
	GetTesting
	WriteTesting
	IterateTesting
	Commit
)

func TestFuzz(t *testing.T) {
	logger := types.NewDefaultLogger()
	cacheStore := NewStoreCache(NewMemoryStore(logger))
	keys := make([][]byte, 0)
	compareStore := NewMemoryStore(logger)
	for i := 0; i < 1000; i++ {
		doRandomOperation(t, cacheStore, compareStore, keys)
	}
}

func doRandomOperation(t *testing.T, db *storeCache, compare types.StoreI, keys [][]byte) {
	k, v := getRandomBytes(t), getRandomBytes(t)
	switch getRandomOperation(t) {
	case SetTesting:
		testDBSet(t, db, k, v)
		testDBSet(t, compare, k, v)
		keys = append(keys, k)
	case DelTesting:
		k = randomTestKey(t, k, keys)
		testDBDelete(t, db, k)
		testDBDelete(t, compare, k)
	case GetTesting:
		k = randomTestKey(t, k, keys)
		v1, v2 := testDBGet(t, db, k), testDBGet(t, compare, k)
		if !bytes.Equal(v1, v2) {
			t.Fatalf("db.Get=%s compare.Get=%s", v1, v2)
		}
	case WriteTesting:
		db.Write()
	case IterateTesting:
		testCompareIterators(t, db, compare)
	case Commit:
		testCompareIterators(t, db, compare)
	default:
		t.Fatal("invalid op")
	}
}

func getRandomBytes(_ *testing.T) []byte {
	bz := make([]byte, math.Intn(1000))
	if _, err := rand.Read(bz); err != nil {
		return nil
	}
	return bz
}

func getRandomOperation(_ *testing.T) TestingOp {
	return TestingOp(math.Intn(3))
}

func randomTestKey(_ *testing.T, k []byte, keys [][]byte) []byte {
	if len(keys) != 0 && math.Intn(100) < 85 {
		// 85% of time use key already found
		// else default to the random value
		k = keys[math.Intn(len(keys))]
	}
	return k
}

func testDBSet(t *testing.T, db types.StoreI, k, v []byte) {
	if err := db.Set(k, v); err != nil {
		t.Fatal(err)
	}
}

func testDBDelete(t *testing.T, db types.StoreI, k []byte) {
	if err := db.Delete(k); err != nil {
		t.Fatal(err)
	}
}

func testDBGet(t *testing.T, db types.StoreI, k []byte) (value []byte) {
	value, err := db.Get(k)
	if err != nil {
		t.Fatal(err)
	}
	return
}

func testCommit(t *testing.T, db *storeCache, compare types.StoreI) {
	db.Write()
}

func testCompareIterators(t *testing.T, db *storeCache, compare types.StoreI) {
	var (
		it1, it2 types.IteratorI
		err      error
	)
	switch math.Intn(1) {
	case 0:
		it1, err = db.Iterator(nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		it2, err = compare.Iterator(nil, nil)
		if err != nil {
			t.Fatal(err)
		}
	case 1:
		it1, err = db.ReverseIterator(nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		it2, err = compare.ReverseIterator(nil, nil)
		if err != nil {
			t.Fatal(err)
		}
	}
	defer func() { it1.Close(); it2.Close() }()
	for ; func() bool { return it1.Valid() || it2.Valid() }(); func() { it1.Next(); it2.Next() }() {
		if it1.Valid() != it2.Valid() {
			t.Fatalf("it1.valid=%t it2.valid=%t ", it1.Valid(), it2.Valid())
		}
		if bytes.Equal(it1.Key(), it2.Key()) {
			t.Fatalf("it1.key=%s it2.key=%s ", hex.EncodeToString(it1.Key()), hex.EncodeToString(it2.Key()))
		}
		if bytes.Equal(it1.Value(), it2.Value()) {
			t.Fatalf("it1.key=%s it2.key=%s ", hex.EncodeToString(it1.Key()), hex.EncodeToString(it2.Key()))
		}
	}
}
