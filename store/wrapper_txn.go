package store

import (
	"bytes"
	"github.com/dgraph-io/badger/v4"
	"github.com/ginchuco/ginchu/lib"
)

var _ lib.RWStoreI = &TxnWrapper{}

type TxnWrapper struct {
	logger lib.LoggerI
	db     *badger.Txn
	prefix string
}

func NewTxnWrapper(db *badger.Txn, logger lib.LoggerI, prefix string) *TxnWrapper {
	return &TxnWrapper{
		logger: logger,
		db:     db,
		prefix: prefix,
	}
}

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

func (t *TxnWrapper) Set(k, v []byte) lib.ErrorI {
	if err := t.db.Set(append([]byte(t.prefix), k...), v); err != nil {
		return ErrStoreSet(err)
	}
	return nil
}

func (t *TxnWrapper) Delete(k []byte) lib.ErrorI {
	if err := t.db.Delete(append([]byte(t.prefix), k...)); err != nil {
		return ErrStoreDelete(err)
	}
	return nil
}

func (t *TxnWrapper) Close()              { t.db.Discard() }
func (t *TxnWrapper) setDB(p *badger.Txn) { t.db = p }

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

func seekLast(it *badger.Iterator, prefix []byte) {
	it.Seek(prefixEnd(prefix))
}

type Iterator struct {
	logger lib.LoggerI
	parent *badger.Iterator
	prefix string
	err    error
}

var _ lib.IteratorI = &Iterator{}

func (i *Iterator) Valid() bool            { return i.parent.Valid() }
func (i *Iterator) Next()                  { i.parent.Next() }
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

var (
	endBytes = bytes.Repeat([]byte{0xFF}, maxKeyBytes+1)
)

func prefixEnd(prefix []byte) []byte {
	return append(prefix, endBytes...)
}
