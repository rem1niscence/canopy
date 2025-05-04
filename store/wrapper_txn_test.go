package store

import (
	"crypto/rand"
	"encoding/hex"
	"github.com/canopy-network/canopy/lib"
	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/require"
	math "math/rand"
	"reflect"
	"testing"
	"unsafe"
)

func TestGetSetDelete(t *testing.T) {
	db, store, cleanup := newTestTxnWrapper(t)
	defer cleanup()
	bulkSetKV(t, store, "", "a", "b")
	got, err := store.Get([]byte("a"))
	require.NoError(t, err)
	require.Equal(t, "a", string(got))
	require.NoError(t, store.Delete([]byte("b")))
	got, err = store.Get([]byte("b"))
	require.NoError(t, err)
	require.Nil(t, got)
	reader := db.NewTransactionAt(0, false)
	_, er := reader.Get([]byte("a"))
	require.Contains(t, er.Error(), badger.ErrKeyNotFound.Error())
}

func TestIteratorBasic(t *testing.T) {
	_, parent, cleanup := newTestTxnWrapper(t)
	defer cleanup()
	expectedVals := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	expectedValsReverse := []string{"h", "g", "f", "e", "d", "c", "b", "a"}
	bulkSetKV(t, parent, "", expectedVals...)
	it, err := parent.Iterator(nil)
	require.NoError(t, err)
	defer it.Close()
	validateIterators(t, expectedVals, it)
	rIt, err := parent.RevIterator(nil)
	require.NoError(t, err)
	defer rIt.Close()
	validateIterators(t, expectedValsReverse, rIt)
}

func TestIteratorWithDelete(t *testing.T) {
	expectedVals := []string{"a", "b", "c", "d", "e", "f", "g"}
	_, parent, cleanup := newTestTxnWrapper(t)
	defer cleanup()
	bulkSetKV(t, parent, "", expectedVals...)
	for i := 0; i < 10; i++ {
		randomindex := math.Intn(len(expectedVals))
		require.NoError(t, parent.Delete([]byte(expectedVals[randomindex])))
		expectedVals = append(expectedVals[:randomindex], expectedVals[randomindex+1:]...)
		cIt, err := parent.Iterator(nil)
		require.NoError(t, err)
		validateIterators(t, expectedVals, cIt)
		cIt.Close()
		add := make([]byte, 1)
		_, er := rand.Read(add)
		require.NoError(t, er)
		expectedVals = append(expectedVals, hex.EncodeToString(add))
	}
}

func newTestTxnWrapper(t *testing.T) (*badger.DB, *TxnWrapper, func()) {
	db, err := badger.OpenManaged(badger.DefaultOptions("").WithInMemory(true).WithLoggingLevel(badger.ERROR))
	require.NoError(t, err)
	parent := NewTxnWrapper(db.NewTransactionAt(1, true), lib.NewDefaultLogger(), []byte(latestStatePrefix))
	return db, parent, func() {
		db.Close()
		parent.Close()
	}
}

func getMeta(e *badger.Item) (value byte) {
	v := reflect.ValueOf(e).Elem()
	f := v.FieldByName(badgerMetaFieldName)
	ptr := unsafe.Pointer(f.UnsafeAddr())
	return *(*byte)(ptr)
}
