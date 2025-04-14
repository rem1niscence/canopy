package store

import (
	"bytes"
	"github.com/canopy-network/canopy/lib"
	"github.com/dgraph-io/badger/v4"
)

// RWStoreI interface enforcement
var _ lib.RWStoreI = &TxnWrapper{}

// TxnWrapper is a wrapper over the badgerDB Txn object that conforms to the RWStoreI interface
type TxnWrapper struct {
	logger lib.LoggerI
	db     *badger.Txn
	prefix string
}

// NewTxnWrapper() creates a new TxnWrapper with the provided params
func NewTxnWrapper(db *badger.Txn, logger lib.LoggerI, prefix string) *TxnWrapper {
	return &TxnWrapper{
		logger: logger,
		db:     db,
		prefix: prefix,
	}
}

// Get() retrieves the value associated with the key from the BadgerDB transaction
func (t *TxnWrapper) Get(k []byte) ([]byte, lib.ErrorI) {
	item, err := t.db.Get(append([]byte(t.prefix), k...))
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
	if err := t.db.Set(append([]byte(t.prefix), k...), v); err != nil {
		return ErrStoreSet(err)
	}
	return nil
}

// Delete() removes the key-value pair from the BadgerDB transaction
func (t *TxnWrapper) Delete(k []byte) lib.ErrorI {
	if err := t.db.Delete(append([]byte(t.prefix), k...)); err != nil {
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
		Prefix: append([]byte(t.prefix), prefix...),
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
	newPrefix := append([]byte(t.prefix), prefix...)
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
		Prefix:      append([]byte(t.prefix), prefix...),
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
type Iterator struct {
	logger lib.LoggerI
	parent *badger.Iterator
	prefix string
	err    error
}

func (i *Iterator) Valid() bool     { return i.parent.Valid() }
func (i *Iterator) Next()           { i.parent.Next() }
func (i *Iterator) Close()          { i.parent.Close() }
func (i *Iterator) Version() uint64 { return i.parent.Item().Version() }
func (i *Iterator) Key() (key []byte) {
	// get the key from the parent
	key = i.parent.Item().Key()
	// make a copy of the key
	c := make([]byte, len(key))
	copy(c, key)
	// remove the prefix and return
	return removePrefix(c, []byte(i.prefix))
}

// removePrefix() removes the prefix from the key
func removePrefix(b, prefix []byte) []byte { return b[len(prefix):] }

// Value() retrieves the current value from the iterator
func (i *Iterator) Value() (value []byte) {
	value, err := i.parent.Item().ValueCopy(nil)
	if err != nil {
		i.err = err
	}
	return
}

var (
	endBytes = bytes.Repeat([]byte{0xFF}, maxKeyBytes+1)
)

// prefixEnd() returns the end key for a given prefix by appending max possible bytes
func prefixEnd(prefix []byte) []byte {
	return append(prefix, endBytes...)
}
