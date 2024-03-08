package store

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"github.com/dgraph-io/badger/v4"
	"github.com/ginchuco/ginchu/types"
	"github.com/stretchr/testify/require"
	math "math/rand"
	"testing"
)

type TestingOp int

const (
	SetTesting TestingOp = iota
	DelTesting
	GetTesting
	IterateTesting
)

func TestFuzz(t *testing.T) {
	db, err := badger.OpenManaged(badger.DefaultOptions("").
		WithInMemory(true).WithLoggingLevel(badger.ERROR))
	require.NoError(t, err)
	store, _, cleanup := testStore(t)
	defer cleanup()
	keys := make([][]byte, 0)
	compareStore2 := NewTxnWrapper(db.NewTransactionAt(1, true), types.NewDefaultLogger(), stateStorePrefix)
	for i := 0; i < 1000; i++ {
		doRandomOperation(t, store, compareStore2, &keys)
	}
}

func doRandomOperation(t *testing.T, db types.KVStoreI, compare types.KVStoreI, keys *[][]byte) {
	k, v := getRandomBytes(t), getRandomBytes(t)
	switch getRandomOperation(t) {
	case SetTesting:
		testDBSet(t, db, k, v)
		testDBSet(t, compare, k, v)
		*keys = append(*keys, k)
	case DelTesting:
		k = randomTestKey(t, k, *keys)
		testDBDelete(t, db, k)
		testDBDelete(t, compare, k)
	case GetTesting:
		k = randomTestKey(t, k, *keys)
		v1, v2 := testDBGet(t, db, k), testDBGet(t, compare, k)
		if !bytes.Equal(v1, v2) {
			fmt.Printf("key=%s db.Get=%s compare.Get=%s\n", k, v1, v2)
		}
		require.Equalf(t, v1, v2, "key=%s db.Get=%s compare.Get=%s", k, v1, v2)
	case IterateTesting:
		testCompareIterators(t, db, compare)
	default:
		t.Fatal("invalid op")
	}
}

func getRandomBytes(t *testing.T) []byte {
	bz := make([]byte, math.Intn(999)+40)
	if _, err := rand.Read(bz); err != nil {
		t.Fatal(err)
	}
	return bz
}

func getRandomOperation(_ *testing.T) TestingOp {
	return TestingOp(math.Intn(4))
}

func randomTestKey(_ *testing.T, k []byte, keys [][]byte) []byte {
	if len(keys) != 0 && math.Intn(100) < 85 {
		// 85% of time use key already found
		// else default to the random value
		k = keys[math.Intn(len(keys))]
	}
	return k
}

func testDBSet(t *testing.T, db types.WritableStoreI, k, v []byte) {
	require.NoError(t, db.Set(k, v))
}

func testDBDelete(t *testing.T, db types.WritableStoreI, k []byte) {
	require.NoError(t, db.Delete(k))
}

func testDBGet(t *testing.T, db types.KVStoreI, k []byte) (value []byte) {
	value, err := db.Get(k)
	require.NoError(t, err)
	return
}

func testCompareIterators(t *testing.T, db types.KVStoreI, compare types.KVStoreI) {
	var (
		it1, it2 types.IteratorI
		err      error
	)
	switch math.Intn(2) {
	case 0:
		it1, err = db.Iterator(nil)
		require.NoError(t, err)
		it2, err = compare.Iterator(nil)
		require.NoError(t, err)
	case 1:
		it1, err = db.RevIterator(nil)
		require.NoError(t, err)
		it2, err = compare.RevIterator(nil)
		require.NoError(t, err)
	}
	defer func() { it1.Close(); it2.Close() }()
	for ; func() bool { return it1.Valid() || it2.Valid() }(); func() { it1.Next(); it2.Next() }() {
		require.Equal(t, it1.Valid(), it2.Valid(), fmt.Sprintf("it1.valid=%t it2.valid=%t ", it1.Valid(), it2.Valid()))
		require.Equal(t, it1.Key(), it2.Key(), fmt.Sprintf("it1.key=%s it2.key=%s ", it1.Key(), it2.Key()))
		require.Equal(t, it1.Value(), it2.Value(), fmt.Sprintf("it1.value=%s it2.value=%s ", it1.Value(), it2.Value()))
	}
}
