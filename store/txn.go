package store

import (
	"bytes"
	"github.com/ginchuco/ginchu/lib"
	"sort"
	"strings"
)

// enforce the StoreTxnI interface
var _ lib.StoreTxnI = &Txn{}

/*
	Txn acts like a database transaction
	It saves set/del operations in memory and allows the caller to Write() to the parent or Discard()
	When read from, it merges with the parent as if Write() had already been called

	Txn abstraction is necessary due to the inability of BadgerDB to have nested transactions.
	Txns allow an easy rollback of write operations within a single Transaction object, which is necessary
	for ephemeral states and testing the validity of a proposal block / transactions.

	CONTRACT:
	- only safe when writing to another memory store like a badger.Txn() as Write() is not atomic.
	- not thread safe
	- nil values are supported; deleted values are also set to nil
	- keys must be smaller than 128 bytes
	- Nested txns are theoretically supported, but iteration becomes increasingly inefficient
*/

type Txn struct {
	parent lib.RWStoreI // memory store to Write() to
	txn
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

// NewTxn() creates a new instance of a Txn with the specified parent store
func NewTxn(parent lib.RWStoreI) *Txn {
	return &Txn{parent: parent, txn: txn{ops: make(map[string]op), sorted: make([]string, 0)}}
}

// Get() retrieves the value for a given key from either the in-memory operations or the parent store
func (c *Txn) Get(key []byte) ([]byte, lib.ErrorI) {
	if v, found := c.ops[string(key)]; found {
		return v.value, nil
	}
	return c.parent.Get(key)
}

// Set() adds or updates the value for a key in the in-memory operations
func (c *Txn) Set(key, value []byte) lib.ErrorI { c.update(string(key), value, false); return nil }

// Delete() marks a key for deletion in the in-memory operations
func (c *Txn) Delete(key []byte) lib.ErrorI { c.update(string(key), nil, true); return nil }

// update() modifies or adds an operation for a key in the in-memory operations and maintains order
func (c *Txn) update(key string, v []byte, delete bool) {
	if _, found := c.ops[key]; !found {
		c.addToSorted(key)
	}
	c.ops[key] = op{value: v, delete: delete}
}

// addToSorted() inserts a key into the sorted list of operations maintaining lexicographical order
func (c *Txn) addToSorted(key string) {
	i := sort.Search(c.sortedLen, func(i int) bool { return c.sorted[i] >= key })
	c.sorted = append(c.sorted, "")
	copy(c.sorted[i+1:], c.sorted[i:])
	c.sorted[i] = key
	c.sortedLen++
}

// Iterator() returns a new iterator for merged iteration of both the in-memory operations and parent store with the given prefix
func (c *Txn) Iterator(prefix []byte) (lib.IteratorI, lib.ErrorI) {
	parent, err := c.parent.Iterator(prefix)
	if err != nil {
		return nil, err
	}
	return newTxnIterator(parent, c.txn, prefix, false), nil
}

// RevIterator() returns a new reverse iterator for merged iteration of both the in-memory operations and parent store with the given prefix
func (c *Txn) RevIterator(prefix []byte) (lib.IteratorI, lib.ErrorI) {
	parent, err := c.parent.RevIterator(prefix)
	if err != nil {
		return nil, err
	}
	return newTxnIterator(parent, c.txn, prefix, true), nil
}

// Discard() clears all in-memory operations and resets the sorted key list
func (c *Txn) Discard() { c.ops, c.sorted, c.sortedLen = nil, nil, 0 }

// Write() flushes the in-memory operations to the parent store and clears in-memory changes
func (c *Txn) Write() (err lib.ErrorI) {
	for k, v := range c.ops {
		if v.delete {
			if err = c.parent.Delete([]byte(k)); err != nil {
				return
			}
		} else {
			if err = c.parent.Set([]byte(k), v.value); err != nil {
				return
			}
		}
	}
	c.ops, c.sorted, c.sortedLen = make(map[string]op), make([]string, 0), 0
	return
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
	return (&TxnIterator{parent: parent, txn: t, prefix: string(prefix), reverse: reverse}).First()
}

// First() positions the iterator at the first valid entry based on the traversal direction
func (c *TxnIterator) First() *TxnIterator {
	if c.reverse {
		return c.revSeek() // seek to the end
	}
	return c.seek() // seek to the beginning
}

// Close() closes the merged iterator
func (c *TxnIterator) Close() { c.parent.Close() }

// Next() advances the iterator to the next entry, choosing between in-memory and parent store entries
func (c *TxnIterator) Next() {
	// if parent is not usable any more then txn.Next()
	// if txn is not usable any more then parent.Next()
	if !c.parent.Valid() {
		c.txnNext()
		return
	}
	if c.txnInvalid() {
		c.parent.Next()
		return
	}
	// compare the keys of the in memory option and the parent option
	switch c.compare(c.txnKey(), c.parent.Key()) {
	case 1: // use parent
		c.parent.Next()
	case 0: // use both
		c.parent.Next()
		c.txnNext()
	case -1: // use txn
		c.txnNext()
	}
}

// Key() returns the current key from either the in-memory operations or the parent store
func (c *TxnIterator) Key() []byte {
	if c.useTxn {
		return c.txnKey()
	}
	return c.parent.Key()
}

// Value() returns the current value from either the in-memory operations or the parent store
func (c *TxnIterator) Value() []byte {
	if c.useTxn {
		return c.txnValue().value
	}
	return c.parent.Value()
}

// Valid() checks if the current position of the iterator is valid, considering both the parent and in-memory entries
func (c *TxnIterator) Valid() bool {
	for {
		if !c.parent.Valid() {
			// only using cache; call txn.next until invalid or !deleted
			c.txnFastForward()
			c.useTxn = true
			break
		}
		if c.txnInvalid() {
			// parent is valid; txn is not
			c.useTxn = false
			break
		}
		// both are valid; key comparison matters
		cKey, pKey := c.txnKey(), c.parent.Key()
		switch c.compare(cKey, pKey) {
		case 1: // use parent
			c.useTxn = false
		case 0: // when equal txn shadows parent
			if c.txnValue().delete {
				c.parent.Next()
				c.txnNext()
				continue
			}
			c.useTxn = true
		case -1: // use txn
			if c.txnValue().delete {
				c.txnNext()
				continue
			}
			c.useTxn = true
		}
		break
	}
	return !c.txnInvalid() || c.parent.Valid()
}

// txnFastForward() skips over deleted entries in the in-memory operations
// return when invalid or !deleted
func (c *TxnIterator) txnFastForward() {
	for {
		if c.txnInvalid() || !c.txnValue().delete {
			return
		}
		c.txnNext()
	}
}

// txnInvalid() determines if the current in-memory entry is invalid
func (c *TxnIterator) txnInvalid() bool {
	if c.invalid {
		return c.invalid
	}
	c.invalid = true
	if c.reverse {
		if c.index < 0 {
			return c.invalid
		}
	} else {
		if c.index >= c.sortedLen {
			return c.invalid
		}
	}
	if !strings.HasPrefix(c.sorted[c.index], c.prefix) {
		return c.invalid
	}
	c.invalid = false
	return c.invalid
}

// txnKey() returns the key of the current in-memory operation
func (c *TxnIterator) txnKey() []byte { return []byte(c.sorted[c.index]) }

// txnValue() returns the value of the current in-memory operation
func (c *TxnIterator) txnValue() op { return c.ops[c.sorted[c.index]] }

// compare() compares two byte slices, adjusting for reverse iteration if needed
func (c *TxnIterator) compare(a, b []byte) int {
	if c.reverse {
		return bytes.Compare(a, b) * -1
	}
	return bytes.Compare(a, b)
}

// txnNext() advances the index of the in-memory operations based on the iteration direction
func (c *TxnIterator) txnNext() {
	if c.reverse {
		c.index--
	} else {
		c.index++
	}
}

// seek() positions the iterator at the first entry that matches or exceeds the prefix.
func (c *TxnIterator) seek() *TxnIterator {
	c.index = sort.Search(c.sortedLen, func(i int) bool { return c.sorted[i] >= c.prefix })
	return c
}

// revSeek() positions the iterator at the last entry that matches the prefix in reverse order.
func (c *TxnIterator) revSeek() *TxnIterator {
	endPrefix := string(prefixEnd([]byte(c.prefix)))
	c.index = sort.Search(c.sortedLen, func(i int) bool { return c.sorted[i] >= endPrefix }) - 1
	return c
}
