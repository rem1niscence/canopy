package store

import (
	"bytes"
	"github.com/ginchuco/ginchu/types"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/util"
)

/*
	This file wraps LevelDB for the KVStore interface
	In doing so, it also implements the IteratorI
	A lot of the iterator code is from Tendermint's leveldb wrapper
*/

var _ types.KVStoreI = &LevelDBWrapper{}

type LevelDBWrapper struct {
	db *leveldb.DB
}

func (l *LevelDBWrapper) Get(key []byte) ([]byte, error) {
	if key == nil {
		return nil, newNilKeyError()
	}
	bz, err := l.db.Get(key, nil)
	if err == errors.ErrNotFound {
		return nil, nil
	}
	return bz, err
}

func (l *LevelDBWrapper) Set(key, value []byte) error {
	if key == nil {
		return newNilKeyError()
	}
	if value == nil {
		return newNilValueError()
	}
	return l.db.Put(key, value, nil)
}

func (l *LevelDBWrapper) Delete(key []byte) error {
	if key == nil {
		return newNilKeyError()
	}
	return l.db.Delete(key, nil)
}

func (l *LevelDBWrapper) Close() error { return l.db.Close() }

func (l *LevelDBWrapper) Iterator(start, end []byte) (types.IteratorI, error) {
	itr := l.db.NewIterator(&util.Range{Start: start, Limit: end}, nil)
	return newLevelDBIt(itr, start, end, false), nil
}

func (l *LevelDBWrapper) ReverseIterator(start, end []byte) (types.IteratorI, error) {
	itr := l.db.NewIterator(&util.Range{Start: start, Limit: end}, nil)
	return newLevelDBIt(itr, start, end, true), nil
}

type levelDBIt struct {
	source  iterator.Iterator
	start   []byte
	end     []byte
	reverse bool
	invalid bool
}

var _ types.IteratorI = (*levelDBIt)(nil)

func newLevelDBIt(source iterator.Iterator, start, end []byte, isReverse bool) *levelDBIt {
	if isReverse {
		if end == nil {
			source.Last()
		} else {
			valid := source.Seek(end)
			if valid {
				endOrAfterKey := source.Key()
				if bytes.Compare(end, endOrAfterKey) <= 0 {
					source.Prev()
				}
			} else {
				source.Last()
			}
		}
	} else {
		if start == nil {
			source.First()
		} else {
			source.Seek(start)
		}
	}
	return &levelDBIt{
		source:  source,
		start:   start,
		end:     end,
		reverse: isReverse,
		invalid: false,
	}
}

func (itr *levelDBIt) Valid() bool {
	if itr.invalid {
		return false
	}
	itr.assertNoError()
	if !itr.source.Valid() {
		itr.invalid = true
		return false
	}
	start, end, key := itr.start, itr.end, itr.source.Key()
	if itr.reverse {
		if start != nil && bytes.Compare(key, start) < 0 {
			itr.invalid = true
			return false
		}
	} else {
		if end != nil && bytes.Compare(end, key) <= 0 {
			itr.invalid = true
			return false
		}
	}
	return true
}

func (itr *levelDBIt) Key() []byte {
	itr.assertNoError()
	itr.assertIsValid()
	return cp(itr.source.Key())
}

func (itr *levelDBIt) Value() []byte {
	itr.assertNoError()
	itr.assertIsValid()
	return cp(itr.source.Value())
}

func (itr *levelDBIt) Next() {
	itr.assertNoError()
	itr.assertIsValid()
	if itr.reverse {
		itr.source.Prev()
	} else {
		itr.source.Next()
	}
}

func (itr *levelDBIt) Error() error {
	return itr.source.Error()
}

func (itr *levelDBIt) Close() {
	itr.source.Release()
}

func (itr *levelDBIt) assertNoError() {
	err := itr.source.Error()
	if err != nil {
		panic(err)
	}
}

func (itr levelDBIt) assertIsValid() {
	if !itr.Valid() {
		panic("iterator is invalid")
	}
}

func cp(bz []byte) (ret []byte) {
	ret = make([]byte, len(bz))
	copy(ret, bz)
	return ret
}
