package store

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"github.com/dgraph-io/badger/v4"
	"github.com/ginchuco/ginchu/types"
	"github.com/stretchr/testify/require"
	math "math/rand"
	"sort"
	"testing"
)

type TestingOp int

const (
	SetTesting TestingOp = iota
	DelTesting
	GetTesting
	IterateTesting
	WriteTesting
	CommitTesting
)

func TestFuzz(t *testing.T) {
	db, err := badger.OpenManaged(badger.DefaultOptions("").
		WithInMemory(true).WithLoggingLevel(badger.ERROR))
	require.NoError(t, err)
	store, _, cleanup := testStore(t)
	defer cleanup()
	defer db.Close()
	keys := make([]string, 0)
	compareStore := NewTxnWrapper(db.NewTransactionAt(1, true), types.NewDefaultLogger(), stateStorePrefix)
	for i := 0; i < 1000; i++ {
		doRandomOperation(t, store, compareStore, &keys)
	}
}

func TestFuzzTxn(t *testing.T) {
	db, err := badger.OpenManaged(badger.DefaultOptions("").
		WithInMemory(true).WithLoggingLevel(badger.ERROR))
	require.NoError(t, err)
	store := NewTxn(NewTxnWrapper(db.NewTransactionAt(1, true), types.NewDefaultLogger(), stateStorePrefix))
	keys := make([]string, 0)
	compareStore := NewTxnWrapper(db.NewTransactionAt(1, true), types.NewDefaultLogger(), stateStorePrefix)
	for i := 0; i < 1000; i++ {
		doRandomOperation(t, store, compareStore, &keys)
	}
	db.Close()
}

func doRandomOperation(t *testing.T, db types.RWStoreI, compare types.RWStoreI, keys *[]string) {
	k, v := getRandomBytes(t, math.Intn(4)), getRandomBytes(t, 3)
	switch getRandomOperation(t) {
	case SetTesting:
		testDBSet(t, db, k, v)
		testDBSet(t, compare, k, v)
		*keys = append(*keys, string(k))
		sort.Strings(*keys)
		*keys = deDuplicate(*keys)
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
		testCompareIterators(t, db, compare, *keys)
	case WriteTesting:
		if x, ok := db.(types.StoreTxnI); ok {
			switch math.Intn(10) {
			case 0:
				require.NoError(t, x.Write())
			}
		}
	case CommitTesting:
		if x, ok := db.(types.StoreI); ok {
			_, err := x.Commit()
			require.NoError(t, err)
		}
	default:
		t.Fatal("invalid op")
	}
}

func deDuplicate(s []string) []string {
	allKeys := make(map[string]bool)
	var list []string
	for _, i := range s {
		if _, value := allKeys[i]; !value {
			allKeys[i] = true
			list = append(list, i)
		}
	}
	return list
}

func getRandomBytes(t *testing.T, n int) []byte {
	bz := make([]byte, n)
	if _, err := rand.Read(bz); err != nil {
		t.Fatal(err)
	}
	return bz
}

//var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz")
//
//func getRandomBytes(t *testing.T, n int) []byte {
//	b := make([]rune, n)
//	for i := range b {
//		b[i] = letterRunes[math.Intn(len(letterRunes))]
//	}
//	return []byte(string(b))
//}

func getRandomOperation(_ *testing.T) TestingOp {
	return TestingOp(math.Intn(6))
}

func randomTestKey(_ *testing.T, k []byte, keys []string) []byte {
	if len(keys) != 0 && math.Intn(100) < 85 {
		// 85% of time use key already found
		// else default to the random value
		k = []byte(keys[math.Intn(len(keys))])
	}
	return k
}

func testDBSet(t *testing.T, db types.WritableStoreI, k, v []byte) {
	require.NoError(t, db.Set(k, v))
}

func testDBDelete(t *testing.T, db types.WritableStoreI, k []byte) {
	require.NoError(t, db.Delete(k))
}

func testDBGet(t *testing.T, db types.RWStoreI, k []byte) (value []byte) {
	value, err := db.Get(k)
	require.NoError(t, err)
	return
}

func testCompareIterators(t *testing.T, db types.RWStoreI, compare types.RWStoreI, keys []string) {
	var (
		it1, it2 types.IteratorI
		err      error
	)
	isReverse := math.Intn(2)
	prefix := getRandomBytes(t, math.Intn(4))
	require.NoError(t, err)
	switch isReverse {
	case 0:
		it1, err = db.Iterator(prefix)
		require.NoError(t, err)
		it2, err = compare.Iterator(prefix)
		require.NoError(t, err)
	case 1:
		it1, err = db.RevIterator(prefix)
		require.NoError(t, err)
		it2, err = compare.RevIterator(prefix)
		require.NoError(t, err)
	}
	defer func() { it1.Close(); it2.Close() }()
	for i := 0; func() bool { return it1.Valid() || it2.Valid() }(); func() { it1.Next(); it2.Next() }() {
		i++
		require.Equal(t, it1.Valid(), it2.Valid(), fmt.Sprintf("it1.valid=%t\ncompare.valid=%t\nisReverse=%d\nprefix=%s\n", it1.Valid(), it2.Valid(), isReverse, prefix))
		require.Equal(t, it1.Key(), it2.Key(), fmt.Sprintf("it1.key=%s\ncompare.key=%s\nisReverse=%d\nprefix=%s\n", it1.Key(), it2.Key(), isReverse, prefix))
		require.Equal(t, it1.Value(), it2.Value(), fmt.Sprintf("it1.value=%s\ncompare.value=%s\nisReverse=%d\nprefix=%s\n", it1.Value(), it2.Value(), isReverse, prefix))
	}
}
