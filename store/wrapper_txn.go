package store

import (
	"bytes"
	"github.com/dgraph-io/badger/v4"
	"github.com/ginchuco/ginchu/types"
)

type TxnWrapper struct {
	logger types.LoggerI
	parent *badger.Txn
	prefix string
}

func NewTxnWrapper(parent *badger.Txn, logger types.LoggerI, prefix string) *TxnWrapper {
	return &TxnWrapper{
		logger: logger,
		parent: parent,
		prefix: prefix,
	}
}

func (t *TxnWrapper) Get(k []byte) ([]byte, error) {
	item, err := t.parent.Get(append([]byte(t.prefix), k...))
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return nil, nil
		}
		return nil, err
	}
	return item.ValueCopy(nil)
}

func (t *TxnWrapper) Set(k, v []byte) error   { return t.parent.Set(append([]byte(t.prefix), k...), v) }
func (t *TxnWrapper) Delete(k []byte) error   { return t.parent.Delete(append([]byte(t.prefix), k...)) }
func (t *TxnWrapper) Close()                  { t.parent.Discard() }
func (t *TxnWrapper) SetParent(p *badger.Txn) { t.parent = p }

func (t *TxnWrapper) Iterator(prefix []byte) (types.IteratorI, error) {
	parent := t.parent.NewIterator(badger.IteratorOptions{
		Prefix: append([]byte(t.prefix), prefix...),
	})
	parent.Rewind()
	return &Iterator{
		logger: t.logger,
		parent: parent,
		prefix: t.prefix,
	}, nil
}

func (t *TxnWrapper) RevIterator(prefix []byte) (types.IteratorI, error) {
	newPrefix := append([]byte(t.prefix), prefix...)
	parent := t.parent.NewIterator(badger.IteratorOptions{
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

func seekLast(it *badger.Iterator, prefix []byte) {
	incPrefix := incrementPrefix(prefix)
	if len(incPrefix) > 0 {
		it.Seek(incPrefix) // Point to the first key <= the range prefix
		if !it.Valid() {
			return
		}
		if it.Item() != nil && bytes.Equal(it.Item().Key(), incPrefix) {
			// Exact match on incremented range prefix, need to move back one key for the last key matching the prefix
			it.Next()
		}
	} else {
		// Entire range prefix was just [0xFF, 0xFF, 0xFF, ..., 0xFF], use the last key in the database
		// There's currently a bug with it.Rewind() when in reverse and with a prefix, it does not correctly find the largest key
		// https://discuss.dgraph.io/t/iterator-rewind-invalid-with-reverse-true-and-prefix-option-set/15518
		it.Seek(bytes.Repeat([]byte{0xFF}, len(prefix)+1))
	}
}

type Iterator struct {
	logger types.LoggerI
	parent *badger.Iterator
	prefix string
	err    error
}

var _ types.IteratorI = &Iterator{}

func (i *Iterator) Valid() bool            { return i.parent.Valid() }
func (i *Iterator) Next()                  { i.parent.Next() }
func (i *Iterator) Error() error           { return i.err }
func (i *Iterator) Close()                 { i.parent.Close() }
func (i *Iterator) Key() (key []byte)      { return removePrefix(i.parent.Item().Key(), []byte(i.prefix)) }
func removePrefix(b, prefix []byte) []byte { return b[len(prefix):] }

func (i *Iterator) Value() (value []byte) {
	value, err := i.parent.Item().ValueCopy(nil)
	if err != nil {
		i.err = err
	}
	return
}

func incrementPrefix(prefix []byte) []byte {
	result := make([]byte, len(prefix))
	copy(result, prefix)
	var len = len(prefix)
	for len > 0 {
		if result[len-1] == 0xFF {
			len -= 1
		} else {
			result[len-1] += 1
			break
		}
	}
	return result[0:len]
}
