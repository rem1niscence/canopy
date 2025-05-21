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
	return &Iterator2{
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
	seekLast2(parent, newPrefix)
	return &Iterator2{
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
	return &Iterator2{
		logger: t.logger,
		parent: parent,
		prefix: t.prefix,
	}, nil
}

// seekLast() positions the iterator at the last key for the given prefix
func seekLast2(it *badger.Iterator, prefix []byte) {
	it.Seek(prefixEnd(prefix))
}

// IteratorI interface enforcement
var _ lib.IteratorI = &Iterator{}

// Iterator implements a wrapper around BadgerDB's iterator but satisfies the IteratorI interface
type Iterator2 struct {
	logger lib.LoggerI
	parent *badger.Iterator
	prefix []byte
	err    error
}

func (i *Iterator2) Valid() bool {
	valid := i.parent.Valid()
	return valid
}
func (i *Iterator2) Next()           { i.parent.Next() }
func (i *Iterator2) Close()          { i.parent.Close() }
func (i *Iterator2) Version() uint64 { return i.parent.Item().Version() }
func (i *Iterator2) Deleted() bool   { return i.parent.Item().IsDeletedOrExpired() }
func (i *Iterator2) Key() (key []byte) {
	// get the key from the parent
	key = i.parent.Item().Key()
	// make a copy of the key
	c := make([]byte, len(key))
	copy(c, key)
	// remove the prefix and return
	return removePrefix(c, []byte(i.prefix))
}

// Value() retrieves the current value from the iterator
func (i *Iterator2) Value() (value []byte) {
	value, err := i.parent.Item().ValueCopy(nil)
	if err != nil {
		i.err = err
	}
	return
}
