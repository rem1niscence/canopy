package store

import (
	"bytes"
	"sort"
	"strings"

	"github.com/canopy-network/canopy/lib"
	"github.com/dgraph-io/badger/v4"
)

// enforce the StoreTxnI interface
// var _ lib.StoreTxnI = &Txn{}
// enforce the Iterator Interface
var _ lib.IteratorI = &Iterator{}

/*
	Txn acts like a database transaction
	It saves set/del operations in memory and allows the caller to Write() to the parent or Discard()
	When read from, it merges with the parent as if Write() had already been called

	Txn abstraction is necessary due to the inability of BadgerDB to have nested transactions.
	Txns allow an easy rollback of write operations within a single Transaction object, which is necessary
	for ephemeral states and testing the validity of a proposal block / transactions.

	CONTRACT:
	- only safe when writing to another memory store like a badger.Txn() as Write() is not atomic.
	- not thread safe (can't use 1 txn across multiple threads)
	- nil values are supported; deleted values are also set to nil
	- keys must be smaller than 128 bytes
	- Nested txns are theoretically supported, but iteration becomes increasingly inefficient
*/

type Txn struct {
	reader *badger.Txn        // memory store to Read() from
	writer *badger.WriteBatch // memory store to Write() to
	prefix []byte             // prefix for keys in this txn
	logger lib.LoggerI        // logger for this txn
	cache  txn
}

// internal txn structure maintains the write operations sorted lexicographically by keys
type txn struct {
	ops       map[string]op // [string(key)] -> set/del operations saved in memory
	sorted    []string      // ops keys sorted lexicographically; needed for iteration
	sortedLen int           // len(sorted)
}

// op or Operation has the value portion of the operation and if it's a *delete* or a *set*
type op struct {
	value  []byte // value of key value pair
	delete bool   // is operation delete
}

// NewTxn() creates a new instance of a Txn with the specified readers/writers and prefix
func NewTxn(reader *badger.Txn, writer *badger.WriteBatch, prefix []byte) *Txn {
	return &Txn{
		reader: reader,
		writer: writer,
		prefix: prefix,
		cache: txn{
			ops:    make(map[string]op),
			sorted: make([]string, 0),
		},
	}
}

// Get() retrieves the value for a given key from either the cache operations or the reader store
func (t *Txn) Get(key []byte) ([]byte, lib.ErrorI) {
	// append the prefix to the key
	prefixedKey := lib.Append(t.prefix, key)
	// first retrieve from the in-memory cache
	if v, found := t.cache.ops[lib.BytesToString(prefixedKey)]; found {
		return v.value, nil
	}
	// if not found, retrieve from the parent reader
	item, err := t.reader.Get(prefixedKey)
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

// Set() adds or updates the value for a key in the cache operations
func (t *Txn) Set(key, value []byte) lib.ErrorI {
	t.update(lib.BytesToString(lib.Append(t.prefix, key)), value, false)
	return nil
}

// Delete() marks a key for deletion in the in-memory operations
func (t *Txn) Delete(key []byte) lib.ErrorI {
	t.update(lib.BytesToString(lib.Append(t.prefix, key)), nil, true)
	return nil
}

// update() modifies or adds an operation for a key in the in-memory operations and maintains the
// lexicographical order.
// NOTE: update() won't modify the key itself, any key prefixing must be done before calling this
func (t *Txn) update(key string, v []byte, delete bool) {
	if _, found := t.cache.ops[key]; !found {
		t.addToSorted(key)
	}
	t.cache.ops[key] = op{value: v, delete: delete}
}

// addToSorted() inserts a key into the sorted list of operations maintaining lexicographical order
func (t *Txn) addToSorted(key string) {
	i := sort.Search(t.cache.sortedLen, func(i int) bool { return t.cache.sorted[i] >= key })
	t.cache.sorted = append(t.cache.sorted, "")
	copy(t.cache.sorted[i+1:], t.cache.sorted[i:])
	t.cache.sorted[i] = key
	t.cache.sortedLen++
}

// Iterator() returns a new iterator for merged iteration of both the in-memory operations and parent store with the given prefix
func (t *Txn) Iterator(prefix []byte) (lib.IteratorI, lib.ErrorI) {
	parent := t.reader.NewIterator(badger.IteratorOptions{
		Prefix: lib.Append(t.prefix, prefix),
	})
	parent.Rewind()
	it := &Iterator{
		logger: t.logger,
		parent: parent,
		prefix: prefix,
	}
	return newTxnIterator(it, t.cache, prefix, false), nil
}

// RevIterator() returns a new reverse iterator for merged iteration of both the in-memory operations and parent store with the given prefix
func (t *Txn) RevIterator(prefix []byte) (lib.IteratorI, lib.ErrorI) {
	newPrefix := lib.Append(t.prefix, prefix)
	parent := t.reader.NewIterator(badger.IteratorOptions{
		Reverse: true,
		Prefix:  newPrefix,
	})
	seekLast(parent, newPrefix)
	it := &Iterator{
		logger: t.logger,
		parent: parent,
		prefix: t.prefix,
	}
	return newTxnIterator(it, t.cache, prefix, true), nil
}

// Discard() clears all in-memory operations and resets the sorted key list
func (t *Txn) Discard() { t.cache.ops, t.cache.sorted, t.cache.sortedLen = nil, nil, 0 }

// Write() flushes the in-memory operations to the batch writer and clears in-memory changes
func (t *Txn) Write() lib.ErrorI {
	for k, v := range t.cache.ops {
		sk, er := lib.StringToBytes(k)
		if er != nil {
			return er
		}
		if v.delete {
			if err := t.writer.Delete(sk); err != nil {
				return ErrStoreDelete(err)
			}
		} else {
			if err := t.writer.Set(sk, v.value); err != nil {
				return ErrStoreSet(err)
			}
		}
	}
	t.cache.ops, t.cache.sorted, t.cache.sortedLen = make(map[string]op), make([]string, 0), 0
	return nil
}

// enforce the Iterator interface
var _ lib.IteratorI = &TxnIterator{}

// TxnIterator is a reversible, merged iterator of the parent and the in-memory operations
type TxnIterator struct {
	parent lib.IteratorI
	txn
	prefix  string
	index   int
	reverse bool
	invalid bool
	useTxn  bool
}

// newTxnIterator() initializes a new merged iterator for traversing both the in-memory operations and parent store
func newTxnIterator(parent lib.IteratorI, t txn, prefix []byte, reverse bool) *TxnIterator {
	return (&TxnIterator{parent: parent, txn: t, prefix: lib.BytesToString(prefix), reverse: reverse}).First()
}

// First() positions the iterator at the first valid entry based on the traversal direction
func (ti *TxnIterator) First() *TxnIterator {
	if ti.reverse {
		return ti.revSeek() // seek to the end
	}
	return ti.seek() // seek to the beginning
}

// Close() closes the merged iterator
func (ti *TxnIterator) Close() { ti.parent.Close() }

// Next() advances the iterator to the next entry, choosing between in-memory and parent store entries
func (ti *TxnIterator) Next() {
	// if parent is not usable any more then txn.Next()
	// if txn is not usable any more then parent.Next()
	if !ti.parent.Valid() {
		ti.txnNext()
		return
	}
	if ti.txnInvalid() {
		ti.parent.Next()
		return
	}
	// compare the keys of the in memory option and the parent option
	switch ti.compare(ti.txnKey(), ti.parent.Key()) {
	case 1: // use parent
		ti.parent.Next()
	case 0: // use both
		ti.parent.Next()
		ti.txnNext()
	case -1: // use txn
		ti.txnNext()
	}
}

// Key() returns the current key from either the in-memory operations or the parent store
func (ti *TxnIterator) Key() []byte {
	if ti.useTxn {
		return ti.txnKey()
	}
	return ti.parent.Key()
}

// Value() returns the current value from either the in-memory operations or the parent store
func (ti *TxnIterator) Value() []byte {
	if ti.useTxn {
		return ti.txnValue().value
	}
	return ti.parent.Value()
}

// Valid() checks if the current position of the iterator is valid, considering both the parent and in-memory entries
func (ti *TxnIterator) Valid() bool {
	for {
		if !ti.parent.Valid() {
			// only using cache; call txn.next until invalid or !deleted
			ti.txnFastForward()
			ti.useTxn = true
			break
		}
		if ti.txnInvalid() {
			// parent is valid; txn is not
			ti.useTxn = false
			break
		}
		// both are valid; key comparison matters
		cKey, pKey := ti.txnKey(), ti.parent.Key()
		switch ti.compare(cKey, pKey) {
		case 1: // use parent
			ti.useTxn = false
		case 0: // when equal txn shadows parent
			if ti.txnValue().delete {
				ti.parent.Next()
				ti.txnNext()
				continue
			}
			ti.useTxn = true
		case -1: // use txn
			if ti.txnValue().delete {
				ti.txnNext()
				continue
			}
			ti.useTxn = true
		}
		break
	}
	return !ti.txnInvalid() || ti.parent.Valid()
}

// txnFastForward() skips over deleted entries in the in-memory operations
// return when invalid or !deleted
func (ti *TxnIterator) txnFastForward() {
	for {
		if ti.txnInvalid() || !ti.txnValue().delete {
			return
		}
		ti.txnNext()
	}
}

// txnInvalid() determines if the current in-memory entry is invalid
func (ti *TxnIterator) txnInvalid() bool {
	if ti.invalid {
		return ti.invalid
	}
	ti.invalid = true
	if ti.reverse {
		if ti.index < 0 {
			return ti.invalid
		}
	} else {
		if ti.index >= ti.sortedLen {
			return ti.invalid
		}
	}
	if !strings.HasPrefix(ti.sorted[ti.index], ti.prefix) {
		return ti.invalid
	}
	ti.invalid = false
	return ti.invalid
}

// txnKey() returns the key of the current in-memory operation
func (ti *TxnIterator) txnKey() []byte {
	bz, _ := lib.StringToBytes(ti.sorted[ti.index])
	return bz
}

// txnValue() returns the value of the current in-memory operation
func (ti *TxnIterator) txnValue() op { return ti.ops[ti.sorted[ti.index]] }

// compare() compares two byte slices, adjusting for reverse iteration if needed
func (ti *TxnIterator) compare(a, b []byte) int {
	if ti.reverse {
		return bytes.Compare(a, b) * -1
	}
	return bytes.Compare(a, b)
}

// txnNext() advances the index of the in-memory operations based on the iteration direction
func (ti *TxnIterator) txnNext() {
	if ti.reverse {
		ti.index--
	} else {
		ti.index++
	}
}

// seek() positions the iterator at the first entry that matches or exceeds the prefix.
func (ti *TxnIterator) seek() *TxnIterator {
	ti.index = sort.Search(ti.sortedLen, func(i int) bool { return ti.sorted[i] >= ti.prefix })
	return ti
}

// revSeek() positions the iterator at the last entry that matches the prefix in reverse order.
func (ti *TxnIterator) revSeek() *TxnIterator {
	bz, _ := lib.StringToBytes(ti.prefix)
	endPrefix := lib.BytesToString(prefixEnd(bz))
	ti.index = sort.Search(ti.sortedLen, func(i int) bool { return ti.sorted[i] >= endPrefix }) - 1
	return ti
}

type Iterator struct {
	logger lib.LoggerI
	parent *badger.Iterator
	prefix []byte
	err    error
}

func (i *Iterator) Valid() bool     { return i.parent.Valid() }
func (i *Iterator) Next()           { i.parent.Next() }
func (i *Iterator) Close()          { i.parent.Close() }
func (i *Iterator) Version() uint64 { return i.parent.Item().Version() }
func (i *Iterator) Deleted() bool   { return i.parent.Item().IsDeletedOrExpired() }
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
	return lib.Append(prefix, endBytes)
}
