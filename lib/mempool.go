package lib

import (
	"bytes"
	"github.com/canopy-network/canopy/lib/crypto"
	"sort"
	"sync"
)

var _ Mempool = &FeeMempool{} // Mempool interface enforcement for FeeMempool implementation

// Mempool interface is a model for a pre-block, in-memory, Transaction store
type Mempool interface {
	Contains(hash string) bool                                       // whether the mempool has this transaction already (de-duplicated by hash)
	AddTransaction(tx []byte, fee uint64) (recheck bool, err ErrorI) // insert new unconfirmed transaction
	DeleteTransaction(tx []byte)                                     // delete unconfirmed transaction
	GetTransactions(maxBytes uint64) ([][]byte, int)                 // retrieve transactions from the highest fee to lowest

	Clear()              // reset the entire store
	TxCount() int        // number of Transactions in the pool
	TxsBytes() int       // collective number of bytes in the pool
	Iterator() IteratorI // loop through each transaction in the pool
}

// FeeMempool is a Mempool implementation that prioritizes transactions with the highest fees
type FeeMempool struct {
	l        sync.RWMutex        // for thread safety
	hashMap  map[string]struct{} // O(1) de-duplication
	pool     MempoolTxs          // the actual pool of transactions
	count    int                 // the number of Transactions in the pool
	txsBytes int                 // collective number of bytes in the pool
	config   MempoolConfig       // user configuration of the pool
}

// MempoolTx is a wrapper over Transaction bytes that maintains the fee associated with the bytes
type MempoolTx struct {
	Tx  []byte // transaction bytes
	Fee uint64 // fee associated with the transaction
}

// NewMempool() creates a new FeeMempool instance of a Mempool
func NewMempool(config MempoolConfig) Mempool {
	if config.DropPercentage == 0 {
		config.DropPercentage = DefaultMempoolConfig().DropPercentage
	}
	return &FeeMempool{
		l:        sync.RWMutex{},
		hashMap:  make(map[string]struct{}),
		pool:     MempoolTxs{s: make([]MempoolTx, 0)},
		count:    0,
		txsBytes: 0,
		config:   config,
	}
}

// AddTransaction() inserts a new unconfirmed Transaction to the Pool and returns if this addition
// requires a recheck of the Mempool due to dropping or re-ordering of the Transactions
func (f *FeeMempool) AddTransaction(tx []byte, fee uint64) (recheck bool, err ErrorI) {
	f.l.Lock()
	defer f.l.Unlock()
	// create quick hash of the transaction for de-duplication;
	// note that hash may not equal Transaction Hash based on the implementation
	hash := crypto.HashString(tx)
	// check for a duplicate
	if _, ok := f.hashMap[hash]; ok {
		return false, ErrTxFoundInMempool(hash)
	}
	// ensure the size of the Transaction doesn't exceed the individual limit
	txBytes := len(tx)
	if uint32(txBytes) >= f.config.IndividualMaxTxSize {
		return false, ErrMaxTxSize()
	}
	// insert the transaction into the pool
	recheck = f.pool.insert(MempoolTx{
		Tx:  tx,
		Fee: fee,
	})
	// insert into de-duplication hash map
	f.hashMap[hash] = struct{}{}
	// increment the count
	f.count++
	// update the number of bytes
	f.txsBytes += txBytes
	// assess if limits are exceeded - if so, drop from the bottom
	var dropped []MempoolTx
	// loop until the conditions are satisfied
	for uint32(f.count) > f.config.MaxTransactionCount || uint64(f.txsBytes) > f.config.MaxTotalBytes {
		// drop percentage is configurable
		dropped = f.pool.drop(f.config.DropPercentage)
		// for each dropped transaction
		for _, d := range dropped {
			// decrement count
			f.count--
			// subtract the txsBytes
			f.txsBytes -= len(d.Tx)
			// delete from teh de-duplication hash map
			delete(f.hashMap, crypto.HashString(d.Tx))
		}
	}
	// if any are dropped or re-order happened
	return len(dropped) != 0 || recheck, nil
}

// GetTransactions() returns a list of the Transactions from the pool up to 'max collective Transaction bytes'
func (f *FeeMempool) GetTransactions(maxBytes uint64) (txs [][]byte, count int) {
	totalBytes := uint64(0)
	for _, tx := range f.pool.s {
		txBytes := len(tx.Tx)
		totalBytes += uint64(txBytes)
		// check to see if the addition of this transaction
		// exceeds the maxBytes limit
		if totalBytes > maxBytes {
			return // return without adding the tx
		}
		// add the tx to the list and increment totalTxs
		txs = append(txs, tx.Tx)
		count++
	}
	return
}

// Contains() checks if a transaction with the given hash exists in the mempool
func (f *FeeMempool) Contains(hash string) bool {
	f.l.RLock()
	defer f.l.RUnlock()
	if _, contains := f.hashMap[hash]; contains {
		return true
	}
	return false
}

// DeleteTransaction() removes the specified transaction from the mempool
func (f *FeeMempool) DeleteTransaction(tx []byte) {
	f.l.Lock()
	defer f.l.Unlock()
	deleted := f.pool.delete(tx)
	if deleted.Tx == nil {
		return
	}
	delete(f.hashMap, crypto.HashString(deleted.Tx))
	f.count--
	f.txsBytes -= len(deleted.Tx)
}

// Clear() empties the mempool and resets its state
func (f *FeeMempool) Clear() {
	f.l.Lock()
	defer f.l.Unlock()
	f.pool = MempoolTxs{s: make([]MempoolTx, 0)}
	f.hashMap = make(map[string]struct{})
	f.count = 0
	f.txsBytes = 0
}

// TxCount() returns the current number of transactions in the mempool
func (f *FeeMempool) TxCount() int {
	f.l.RLock()
	defer f.l.RUnlock()
	return f.count
}

// TxsBytes() returns the total size in bytes of all transactions in the mempool
func (f *FeeMempool) TxsBytes() int {
	f.l.RLock()
	defer f.l.RUnlock()
	return f.txsBytes
}

// Iterator() creates a new iterator for traversing the transactions in the mempool
func (f *FeeMempool) Iterator() IteratorI {
	return NewMempoolIterator(f.pool)
}

var _ IteratorI = &mempoolIterator{} // enforce

// mempoolIterator implements IteratorI using the list of Transactions the index and if the position is valid
type mempoolIterator struct {
	pool  *MempoolTxs // reference to list of Transactions
	index int         // index position
	valid bool        // is the position valid
}

// NewMempoolIterator() initializes a new iterator for the mempool transactions
func NewMempoolIterator(p MempoolTxs) *mempoolIterator {
	pool := p.copy() // copy the pool for safe iteration during a parallel
	return &mempoolIterator{pool: pool, valid: pool.count != 0}
}

// Valid() checks if the iterator is positioned on a valid element
func (m *mempoolIterator) Valid() bool { return m.index < m.pool.count }

// Next() advances the iterator to the next transaction in the pool
func (m *mempoolIterator) Next() { m.index++ }

// Key() returns the transaction at the current iterator position
func (m *mempoolIterator) Key() (key []byte) { return m.pool.s[m.index].Tx }

// Value() returns same as key
func (m *mempoolIterator) Value() (value []byte) { return m.Key() }

// Error() always returns nil, as no errors are tracked by this iterator
func (m *mempoolIterator) Error() error { return nil }

// Close() is a no-op in this iterator, as no resources need to be released
func (m *mempoolIterator) Close() {}

// MempoolTxs is a list of MempoolTxs with a count
type MempoolTxs struct {
	count int
	s     []MempoolTx
}

// insert() inserts a new tx into the list sorted by the highest fee to the lowest fee
func (t *MempoolTxs) insert(tx MempoolTx) (recheck bool) {
	// The comparison t.s[i].Fee < tr.Fee ensures that the search returns the first position
	// where the fee is less than the transaction being inserted. This places transactions with
	// higher fees at the beginning of the slice
	i := sort.Search(t.count, func(i int) bool {
		return t.s[i].Fee < tx.Fee
	})
	// if insert position isn't at the end
	// there is a re-org which requires rechecking
	// of all Transactions in the list
	if i != t.count {
		recheck = true
	}
	// add an empty slot to the slice
	t.s = append(t.s, MempoolTx{})
	// move everything to the right of
	// the insert point one over
	copy(t.s[i+1:], t.s[i:])
	// insert the new tx
	t.s[i] = tx
	// increment the count
	t.count++
	return
}

// delete() evicts a transaction from the list and re-order based on the fee
func (t *MempoolTxs) delete(tx []byte) (deleted MempoolTx) {
	index := t.count
	for i := 0; i < t.count; i++ {
		// if candidate == target
		if bytes.Equal(t.s[i].Tx, tx) {
			index = i
			break
		}
	}
	// transaction not found
	if index == t.count {
		return
	}
	// set the evicted
	deleted = t.s[index]
	// remove it from the list
	t.s = append(t.s[:index], t.s[index+1:]...)
	// decrement the count
	t.count--
	return
}

// drop() removes the bottom (the lowest fee) X percent of Transactions
func (t *MempoolTxs) drop(percent int) (dropped []MempoolTx) {
	// calculate the percent using integer division
	numDrop := (t.count*percent)/100 + 1
	// decrement count by number evicted
	t.count -= numDrop
	// save the evicted list
	dropped = t.s[t.count:]
	// update the list with what's not evicted
	t.s = t.s[:t.count]
	return
}

// copy() returns a shallow copy of the MempoolTxs
func (t *MempoolTxs) copy() *MempoolTxs {
	dst := make([]MempoolTx, t.count)
	copy(dst, t.s)
	return &MempoolTxs{
		count: t.count,
		s:     dst,
	}
}
