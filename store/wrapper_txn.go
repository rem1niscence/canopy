package store

import (
	"crypto/rand"
	"fmt"
	"math"
	"reflect"
	"unsafe"

	"github.com/dgraph-io/badger/v4/skl"

	"github.com/canopy-network/canopy/lib"
	"github.com/dgraph-io/badger/v4"
)

const (
	// ----------------------------------------------------------------------------------------------------------------
	// BadgerDB garbage collector behavior is not well documented leading to many open issues in their repository
	// However, here is our current understanding based on experimentation
	// ----------------------------------------------------------------------------------------------------------------
	// 1. Manual Keep (Protection)
	//    - `badgerNoDiscardBit` prevents automatic GC of a key version.
	//    - However, it can be manually superseded by a manual removal
	//
	// 2. Manual Remove (Explicit Deletion or Pruning)
	//    - Deleting a key at a higher ts removes earlier versions once `discardTs >= ts`.
	//    - Setting `badgerDiscardEarlierVersions` is similar, except it retains the current version.
	//
	// 3. Auto Remove – Tombstones
	//    - Deleted keys (tombstoned) <= `discardTs` are automatically purged unless protected by `badgerNoDiscardBit`
	//
	// 4. Auto Remove – Set Entries
	//    - For non-deleted (live) keys, Badger retains the number of versions to retain is defined by `KeepNumVersions`.
	//    - Older versions exceeding this count are automatically eligible for GC.
	//
	//   Note:
	// - The first GC pass after updating `discardTs` and flushing memtable is deterministic
	// - Subsequent GC runs are probabilistic, depending on reclaimable space and value log thresholds
	// ----------------------------------------------------------------------------------------------------------------
	// Bits source: https://github.com/hypermodeinc/badger/blob/85389e88bf308c1dc271383b77b67f4ef4a85194/value.go#L37
	badgerMetaFieldName                = "meta"  // badgerDB Entry 'meta' field name
	badgerDiscardEarlierVersions  byte = 1 << 2  // badgerDB 'discard earlier versions' flag
	badgerDeleteBit               byte = 1 << 0  // badgerDB 'tombstoned' flag
	badgerNoDiscardBit            byte = 1 << 3  // badgerDB 'never discard'  bit
	badgerGCRatio                      = .15     // the ratio when badgerDB will run the garbage collector
	badgerSizeFieldName                = "size"  // badgerDB Txn 'size' field name
	badgerCountFieldName               = "count" // badgerDB Txn 'count' field name
	badgerTxnFieldName                 = "txn"   // badgerDB WriteBatch 'txn' field name
	badgerDBMaxBatchScalingFactor      = 0.98425 // through experimentation badgerDB's max transaction scaling factor
)

// RWStoreI interface enforcement
var _ lib.RWStoreI = &TxnWrapper{}

// TxnWrapper is a wrapper over the badgerDB Txn object that conforms to the RWStoreI interface
type TxnWrapper struct {
	logger lib.LoggerI
	db     *badger.Txn
	size   uint64
	prefix []byte
}

// NewTxnWrapper() creates a new TxnWrapper with the provided params
func NewTxnWrapper(db *badger.Txn, logger lib.LoggerI, prefix []byte) *TxnWrapper {
	return &TxnWrapper{
		logger: logger,
		db:     db,
		prefix: prefix,
	}
}

// Get() retrieves the value associated with the key from the BadgerDB transaction
func (t *TxnWrapper) Get(k []byte) ([]byte, lib.ErrorI) {
	item, err := t.db.Get(lib.Append(t.prefix, k))
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return nil, nil
		}
		return nil, ErrStoreGet(err)
	}
	val, err := item.ValueCopy(nil)
	if err != nil {
		return nil, ErrStoreGet(err)
	}
	return val, nil
}

// Set() stores the key-value pair in the BadgerDB transaction
func (t *TxnWrapper) Set(k, v []byte) lib.ErrorI {
	if err := t.db.Set(lib.Append(t.prefix, k), v); err != nil {
		return ErrStoreSet(err)
	}
	return nil
}

// Delete() removes the key-value pair from the BadgerDB transaction
func (t *TxnWrapper) Delete(k []byte) lib.ErrorI {
	if err := t.db.Delete(lib.Append(t.prefix, k)); err != nil {
		return ErrStoreDelete(err)
	}
	return nil
}

// Tombstone() removes the key-value pair from the BadgerDB transaction but prevents it from being garbage collected
func (t *TxnWrapper) Tombstone(k []byte) lib.ErrorI {
	// set an entry with a bit that marks it as deleted and prevents it from being discarded
	if err := t.db.SetEntry(newEntry(lib.Append(t.prefix, k), nil, badgerDeleteBit|badgerNoDiscardBit)); err != nil {
		return ErrStoreDelete(err)
	}
	return nil
}

// Close() discards the current transaction
func (t *TxnWrapper) Close()              { t.db.Discard() }
func (t *TxnWrapper) setDB(p *badger.Txn) { t.db = p }

// Iterator() creates a new iterator for the given prefix in the BadgerDB transaction
func (t *TxnWrapper) Iterator(prefix []byte) (lib.IteratorI, lib.ErrorI) {
	parent := t.db.NewIterator(badger.IteratorOptions{
		Prefix: lib.Append(t.prefix, prefix),
	})
	parent.Rewind()
	return &Iterator{
		logger: t.logger,
		parent: parent,
		prefix: t.prefix,
	}, nil
}

// RevIterator() creates a new reverse iterator for the given prefix in the BadgerDB transaction
func (t *TxnWrapper) RevIterator(prefix []byte) (lib.IteratorI, lib.ErrorI) {
	newPrefix := lib.Append(t.prefix, prefix)
	parent := t.db.NewIterator(badger.IteratorOptions{
		Reverse: true,
		Prefix:  newPrefix,
	})
	seekLast(parent, newPrefix)
	return &Iterator{
		logger: t.logger,
		parent: parent,
		prefix: t.prefix,
	}, nil
}

// ArchiveIterator() creates a new iterator for all versions under the given prefix in the BadgerDB transaction
func (t *TxnWrapper) ArchiveIterator(prefix []byte) (lib.IteratorI, lib.ErrorI) {
	parent := t.db.NewIterator(badger.IteratorOptions{
		Prefix:      lib.Append(t.prefix, prefix),
		AllVersions: true,
	})
	parent.Rewind()
	return &Iterator{
		logger: t.logger,
		parent: parent,
		prefix: t.prefix,
	}, nil
}

// seekLast() positions the iterator at the last key for the given prefix
func seekLast(it *badger.Iterator, prefix []byte) {
	it.Seek(prefixEnd(prefix))
}

// IteratorI interface enforcement
var _ lib.IteratorI = &Iterator{}

// Iterator implements a wrapper around BadgerDB's iterator but satisfies the IteratorI interface

// newEntry() creates a new badgerDB entry
func newEntry(key, value []byte, meta byte) (e *badger.Entry) {
	e = &badger.Entry{Key: key, Value: value}
	setMeta(e, meta)
	return
}

// FlushMemTable() ensures badgerDB is flushing its mem table before running flatten
// IMPORTANT - discardTs must be set before this
func FlushMemTable(db *badger.DB) lib.ErrorI {
	// get random 32 bytes
	randomPrefix := make([]byte, 32)
	if _, err := rand.Read(randomPrefix); err != nil {
		return ErrReadBytes(err)
	}
	// create a new transaction to write to the database
	tx := db.NewTransactionAt(math.MaxUint64, true)
	// write the random prefix to the database
	if err := tx.Set(randomPrefix, nil); err != nil {
		return ErrSetEntry(err)
	}
	// commit the transaction
	if err := tx.CommitAt(math.MaxUint64, nil); err != nil {
		return ErrCommitDB(err)
	}
	// call drop prefix which triggers the mempool flush
	// NOTE: this only works if an actual prefix exists
	if err := db.DropPrefix(randomPrefix); err != nil {
		return ErrFlushMemTable(err)
	}
	return nil
}

// setBatchOptions() updates unexported badgerDB batch options
func setBatchOptions(db *badger.DB, batchSize int64) error {
	// Access the DB struct's Options field using reflection
	v := reflect.ValueOf(db).Elem()

	// Access the 'opt' field of DB struct using unsafe
	optField := v.FieldByName("opt")
	if !optField.IsValid() {
		return fmt.Errorf("unable to access 'opt' field")
	}

	// Use unsafe.Pointer to get a pointer to the unexported field
	optPtr := unsafe.Pointer(optField.UnsafeAddr())
	optStruct := reflect.NewAt(optField.Type(), optPtr).Elem()

	maxBatchSizeField := optStruct.FieldByName("maxBatchSize")
	maxBatchCountField := optStruct.FieldByName("maxBatchCount")
	// Now, use unsafe.Pointer to modify the unexported fields directly
	batchSizePtr := unsafe.Pointer(maxBatchSizeField.UnsafeAddr())
	batchCountPtr := unsafe.Pointer(maxBatchCountField.UnsafeAddr())

	// Set values using unsafe pointer manipulation
	*(*int64)(batchSizePtr) = batchSize
	*(*int64)(batchCountPtr) = batchSize / int64(skl.MaxNodeSize)

	return nil
}

// setMeta() accesses the private field 'meta' of badgerDB's `Entry`
// badger doesn't yet allow users to explicitly set keys as *do not discard*
// https://github.com/hypermodeinc/badger/issues/2192
func setMeta(e *badger.Entry, value byte) {
	v := reflect.ValueOf(e).Elem()
	f := v.FieldByName(badgerMetaFieldName)
	ptr := unsafe.Pointer(f.UnsafeAddr())
	*(*byte)(ptr) = value
}

// getTxnFromBatch() accesses the private field 'size/count' of badgerDB's `Txn` inside a 'WriteBatch'
// badger doesn't yet allow users to access this info - though it allows users to avoid
// TxnTooBig errors
func getSizeAndCountFromBatch(batch *badger.WriteBatch) (size, count int64) {
	v := reflect.ValueOf(batch).Elem()
	f := v.FieldByName(badgerTxnFieldName)
	if f.Kind() != reflect.Ptr || f.IsNil() {
		return 0, 0
	}
	// f.Pointer() is the uintptr of the actual *Txn
	txPtr := (*badger.Txn)(unsafe.Pointer(f.Pointer()))
	return getSizeAndCount(txPtr)
}

// getSizeAndCount() accesses the private field 'size/count' of badgerDB's `Txn`
// badger doesn't yet allow users to access this info - though it allows users to avoid
// TxnTooBig errors
func getSizeAndCount(txn *badger.Txn) (size, count int64) {
	v := reflect.ValueOf(txn).Elem()
	sizeF, countF := v.FieldByName(badgerSizeFieldName), v.FieldByName(badgerCountFieldName)
	if !sizeF.IsValid() || !countF.IsValid() {
		return 0, 0
	}
	sizePtr, countPtr := unsafe.Pointer(sizeF.UnsafeAddr()), unsafe.Pointer(countF.UnsafeAddr())
	size, count = *(*int64)(sizePtr), *(*int64)(countPtr)
	return
}
