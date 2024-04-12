package types

import (
	"bytes"
	lib "github.com/ginchuco/ginchu/types"
	"github.com/ginchuco/ginchu/types/crypto"
	"sort"
	"sync"
)

var _ lib.Mempool = &FeeMempool{}

type FeeMempool struct {
	l                    sync.RWMutex
	hashMap              map[string]struct{}
	pool                 Transactions
	size                 int
	transactionBytes     int
	dropPercentage       int
	maxTransactionsBytes uint64
	maxTransactions      uint32
}

type Transaction struct {
	Tx  []byte
	Fee string
}

func NewMempool(config MempoolConfig) lib.Mempool {
	return &FeeMempool{
		l:                    sync.RWMutex{},
		hashMap:              make(map[string]struct{}),
		pool:                 Transactions{s: make([]Transaction, 0)},
		size:                 0,
		transactionBytes:     0,
		dropPercentage:       config.DropPercentage,
		maxTransactionsBytes: config.MaxTransactionBytes,
		maxTransactions:      config.MaxTransactions,
	}
}

type MempoolConfig struct {
	MaxTransactionBytes uint64
	MaxTransactions     uint32
	DropPercentage      int
}

func DefaultMempoolConfig() MempoolConfig {
	return MempoolConfig{
		MaxTransactionBytes: 512000000,
		MaxTransactions:     1000000,
		DropPercentage:      35,
	}
}

func (f *FeeMempool) AddTransaction(tx []byte, fee string) (recheck bool, err lib.ErrorI) {
	f.l.Lock()
	defer f.l.Unlock()
	hash := crypto.HashString(tx)
	if _, ok := f.hashMap[hash]; ok {
		return false, ErrTxFoundInMempool(hash)
	}
	recheck = f.pool.Insert(Transaction{
		Tx:  tx,
		Fee: fee,
	})
	f.hashMap[hash] = struct{}{}
	f.size++
	f.transactionBytes += len(tx)
	var dropped []Transaction
	if uint32(f.size) >= f.maxTransactions || uint64(f.transactionBytes) >= f.maxTransactionsBytes {
		dropped = f.pool.Drop(f.dropPercentage)
	}
	return len(dropped) != 0 || recheck, nil
}

func (f *FeeMempool) GetTransactions(maxBytes uint64) (totalTxs int, txs [][]byte) {
	totalBytes := uint64(0)
	for _, tx := range f.pool.s {
		txLen := len(tx.Tx)
		if totalBytes+uint64(txLen) > maxBytes {
			return
		}
		bz := make([]byte, txLen)
		copy(bz, tx.Tx)
		txs = append(txs, bz)
		totalBytes += uint64(txLen)
		totalTxs++
	}
	return
}

func (f *FeeMempool) Contains(hash string) bool {
	f.l.RLock()
	defer f.l.RUnlock()
	if _, has := f.hashMap[hash]; has {
		return true
	}
	return false
}

func (f *FeeMempool) DeleteTransaction(tx []byte) {
	f.l.Lock()
	defer f.l.Unlock()
	deleted := f.pool.Delete(tx)
	delete(f.hashMap, crypto.HashString(deleted.Tx))
	f.size--
	f.transactionBytes -= len(deleted.Tx)
}

func (f *FeeMempool) Clear() {
	f.l.Lock()
	defer f.l.Unlock()
	f.pool = Transactions{s: make([]Transaction, 0)}
	f.hashMap = make(map[string]struct{})
	f.size = 0
	f.transactionBytes = 0
}

func (f *FeeMempool) Size() int {
	f.l.RLock()
	defer f.l.RUnlock()
	return f.size
}

func (f *FeeMempool) TxsBytes() int {
	f.l.RLock()
	defer f.l.RUnlock()
	return f.transactionBytes
}

func (f *FeeMempool) Iterator() lib.IteratorI {
	return NewMempoolIterator(f.pool)
}

var _ lib.IteratorI = &mempoolIterator{}

type mempoolIterator struct {
	pool  *Transactions
	index int
	valid bool
}

func NewMempoolIterator(p Transactions) *mempoolIterator {
	pool := p.Copy()
	return &mempoolIterator{pool: pool, valid: pool.n != 0}
}

func (m *mempoolIterator) Valid() bool           { return m.index < m.pool.n }
func (m *mempoolIterator) Next()                 { m.index++ }
func (m *mempoolIterator) Key() (key []byte)     { return m.pool.s[m.index].Tx }
func (m *mempoolIterator) Value() (value []byte) { return m.Key() }
func (m *mempoolIterator) Error() error          { return nil }
func (m *mempoolIterator) Close()                {}

type Transactions struct {
	n int
	s []Transaction
}

func (t *Transactions) Insert(tr Transaction) (recheck bool) {
	i := sort.Search(t.n, func(i int) bool {
		less, _ := lib.StringsLess(t.s[i].Fee, tr.Fee)
		return less
	})
	if i != t.n {
		recheck = true
	}
	t.s = append(t.s, Transaction{})
	copy(t.s[i+1:], t.s[i:])
	t.s[i] = tr
	t.n++
	return
}

func (t *Transactions) Delete(tx []byte) (deleted Transaction) {
	i := sort.Search(t.n, func(i int) bool {
		return bytes.Equal(t.s[i].Tx, tx)
	})
	if i == t.n {
		return
	}
	deleted = t.s[i]
	t.s = append(t.s[:i], t.s[i+1:]...)
	t.n--
	return
}

func (t *Transactions) Drop(percent int) (dropped []Transaction) {
	numDrop := (t.n * percent) / 100
	t.n -= numDrop
	dropped = t.s[t.n:]
	t.s = t.s[:t.n]
	return
}

func (t *Transactions) Copy() *Transactions {
	dst := make([]Transaction, t.n)
	copy(dst, t.s)
	return &Transactions{
		n: t.n,
		s: dst,
	}
}
