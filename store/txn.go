package store

import (
	"bytes"
	lib "github.com/ginchuco/ginchu/types"
	"sort"
	"strings"
)

/*
	Txn acts like a database transaction
	It saves set/del operations in memory and allows the caller to Write() to the parent or Discard()
	When read from, it merges with the parent as if Write() had already been called

	CONTRACT:
	- only safe when writing to another memory store like a badger.Txn() as Write() is not atomic.
	- not thread safe
	- nil values are supported; deleted values are also set to nil
	- keys must be smaller than 128 bytes
	- Nested txns are theoretically supported, but iteration becomes increasingly inefficient
*/

var _ lib.StoreTxnI = &Txn{}

type Txn struct {
	parent lib.RWStoreI // memory store to Write() to
	txn
}

type txn struct {
	ops       map[string]op // [string(key)] -> set/del operations saved in memory
	sorted    []string      // ops keys sorted lexicographically; needed for iteration
	sortedLen int           // len(sorted)
}

type op struct {
	value  []byte // value of key value pair
	delete bool   // is operation delete
}

func NewTxn(parent lib.RWStoreI) *Txn {
	return &Txn{parent: parent, txn: txn{ops: make(map[string]op), sorted: make([]string, 0)}}
}

func (c *Txn) Get(key []byte) ([]byte, lib.ErrorI) {
	if v, found := c.ops[string(key)]; found {
		return v.value, nil
	}
	return c.parent.Get(key)
}

func (c *Txn) Set(key, value []byte) lib.ErrorI { c.update(string(key), value, false); return nil }
func (c *Txn) Delete(key []byte) lib.ErrorI     { c.update(string(key), nil, true); return nil }
func (c *Txn) update(key string, v []byte, delete bool) {
	if _, found := c.ops[key]; !found {
		c.addToSorted(key)
	}
	c.ops[key] = op{value: v, delete: delete}
}

func (c *Txn) addToSorted(key string) {
	i := sort.Search(c.sortedLen, func(i int) bool { return c.sorted[i] >= key })
	c.sorted = append(c.sorted, "")
	copy(c.sorted[i+1:], c.sorted[i:])
	c.sorted[i] = key
	c.sortedLen++
}

func (c *Txn) Iterator(prefix []byte) (lib.IteratorI, lib.ErrorI) {
	parent, err := c.parent.Iterator(prefix)
	if err != nil {
		return nil, err
	}
	return newTxnIterator(parent, c.txn, prefix, false), nil
}

func (c *Txn) RevIterator(prefix []byte) (lib.IteratorI, lib.ErrorI) {
	parent, err := c.parent.RevIterator(prefix)
	if err != nil {
		return nil, err
	}
	return newTxnIterator(parent, c.txn, prefix, true), nil
}

func (c *Txn) Discard() { c.ops, c.sorted, c.sortedLen = nil, nil, 0 }
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

var _ lib.IteratorI = &TxnIterator{}

type TxnIterator struct {
	parent lib.IteratorI
	txn
	prefix  string
	index   int
	reverse bool
	invalid bool
	useTxn  bool
}

func newTxnIterator(parent lib.IteratorI, t txn, prefix []byte, reverse bool) *TxnIterator {
	return (&TxnIterator{parent: parent, txn: t, prefix: string(prefix), reverse: reverse}).First()
}

func (c *TxnIterator) First() *TxnIterator {
	if c.reverse {
		return c.revSeek()
	}
	return c.seek()
}
func (c *TxnIterator) Close() { c.parent.Close() }
func (c *TxnIterator) Next() {
	if !c.parent.Valid() {
		c.txnNext()
		return
	}
	if c.txnInvalid() {
		c.parent.Next()
		return
	}
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

func (c *TxnIterator) Key() []byte {
	if c.useTxn {
		return c.txnKey()
	}
	return c.parent.Key()
}

func (c *TxnIterator) Value() []byte {
	if c.useTxn {
		return c.txnValue().value
	}
	return c.parent.Value()
}

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

// return when invalid or !deleted
func (c *TxnIterator) txnFastForward() {
	for {
		if c.txnInvalid() || !c.txnValue().delete {
			return
		}
		c.txnNext()
	}
}

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

func (c *TxnIterator) txnKey() []byte { return []byte(c.sorted[c.index]) }
func (c *TxnIterator) txnValue() op   { return c.ops[c.sorted[c.index]] }
func (c *TxnIterator) compare(a, b []byte) int {
	if c.reverse {
		return bytes.Compare(a, b) * -1
	}
	return bytes.Compare(a, b)
}

func (c *TxnIterator) txnNext() {
	if c.reverse {
		c.index--
	} else {
		c.index++
	}
}

func (c *TxnIterator) seek() *TxnIterator {
	c.index = sort.Search(c.sortedLen, func(i int) bool { return c.sorted[i] >= c.prefix })
	return c
}

func (c *TxnIterator) revSeek() *TxnIterator {
	endPrefix := string(prefixEnd([]byte(c.prefix)))
	c.index = sort.Search(c.sortedLen, func(i int) bool { return c.sorted[i] >= endPrefix }) - 1
	return c
}
