package types

import (
	"bytes"
	"container/list"
	"github.com/ginchuco/ginchu/crypto"
	lib "github.com/ginchuco/ginchu/types"
	"sync"
)

var _ lib.Mempool = &FIFOMempool{}

type FIFOMempool struct {
	l                    sync.RWMutex
	hashMap              map[string]struct{}
	pool                 *list.List
	size                 int
	transactionBytes     int
	maxTransactionsBytes uint64
	maxTransactions      uint32
}

func NewMempool(maxTransactionBytes uint64, maxTransactions uint32) lib.Mempool {
	return &FIFOMempool{
		l:                    sync.RWMutex{},
		hashMap:              make(map[string]struct{}),
		pool:                 list.New(),
		size:                 0,
		transactionBytes:     0,
		maxTransactionsBytes: maxTransactionBytes,
		maxTransactions:      maxTransactions,
	}
}

func (f *FIFOMempool) AddTransaction(tx []byte) lib.ErrorI {
	f.l.Lock()
	defer f.l.Unlock()
	hash := crypto.HashString(tx)
	if _, ok := f.hashMap[hash]; ok {
		return ErrTxFoundInMempool(hash)
	}
	f.pool.PushBack(tx)
	f.hashMap[hash] = struct{}{}
	f.size++
	f.transactionBytes += len(tx)
	for uint32(f.size) >= f.maxTransactions || uint64(f.transactionBytes) >= f.maxTransactionsBytes {
		_, err := popTransaction(f)
		if err != nil {
			return err
		}
	}
	return nil
}

func (f *FIFOMempool) Contains(hash string) bool {
	f.l.RLock()
	defer f.l.RUnlock()
	if _, has := f.hashMap[hash]; has {
		return true
	}
	return false
}

func (f *FIFOMempool) DeleteTransaction(tx []byte) lib.ErrorI {
	f.l.Lock()
	defer f.l.Unlock()
	var toRemove *list.Element
	for e := f.pool.Front(); e.Next() != nil; {
		if bytes.Equal(tx, e.Value.([]byte)) {
			toRemove = e
			break
		}
	}
	if toRemove != nil {
		_, err := removeTransaction(f, toRemove)
		if err != nil {
			return err
		}
	}
	return nil
}

func (f *FIFOMempool) PopTransaction() ([]byte, lib.ErrorI) {
	tx, err := popTransaction(f)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func (f *FIFOMempool) Clear() {
	f.l.Lock()
	defer f.l.Unlock()
	f.pool = list.New()
	f.hashMap = make(map[string]struct{})
	f.size = 0
	f.transactionBytes = 0
}

func (f *FIFOMempool) Size() int {
	f.l.RLock()
	defer f.l.RUnlock()
	return f.size
}

func (f *FIFOMempool) TxsBytes() int {
	f.l.RLock()
	defer f.l.RUnlock()
	return f.transactionBytes
}

func removeTransaction(f *FIFOMempool, e *list.Element) ([]byte, lib.ErrorI) {
	if f.size == 0 {
		return nil, nil
	}
	txBz := e.Value.([]byte)
	txBzLen := len(txBz)
	f.pool.Remove(e)
	hashString := crypto.HashString(txBz)
	delete(f.hashMap, hashString)
	f.size--
	f.transactionBytes -= txBzLen
	return txBz, nil
}

func popTransaction(f *FIFOMempool) ([]byte, lib.ErrorI) {
	return removeTransaction(f, f.pool.Front())
}
