package store

import (
	"github.com/canopy-network/canopy/lib"
	"github.com/dgraph-io/badger/v4"
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

// IteratorI interface enforcement
var _ lib.IteratorI = &Iterator{}
