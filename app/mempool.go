package app

import (
	"encoding/hex"
	"github.com/ginchuco/ginchu/crypto"
	"github.com/ginchuco/ginchu/state_machine"
	"github.com/ginchuco/ginchu/state_machine/types"
	"github.com/ginchuco/ginchu/store"
	lib "github.com/ginchuco/ginchu/types"
)

type Mempool struct {
	mempool      lib.Mempool
	mempoolStore lib.StoreI // ephemeral state to have safety within the mempool
	mempoolState state_machine.StateMachine

	feeLimit string
}

// HandleTransaction handles mempool acceptance from incoming transactions
// It alone decides if the transaction is valid based on the mempool state
//
// Mempool gracefully handles
// - If mempool is full it will drop
func (m *Mempool) HandleTransaction(tx []byte) lib.ErrorI {
	hash := crypto.Hash(tx)
	hashString := hex.EncodeToString(hash)
	txResult, er := m.mempoolStore.GetByHash(hash)
	if er != nil {
		return types.ErrGetTransaction(er)
	}
	if txResult != nil {
		return types.ErrDuplicateTx(hash)
	}
	if m.mempool.Contains(hashString) {
		return types.ErrTxFoundInMempool(hashString)
	}
	txn := m.mempoolStore.NewTxn()
	m.mempoolState.SetStore(txn)
	defer func() {
		m.mempoolState.SetStore(m.mempoolStore)
		txn.Discard()
	}()
	txResult, err := m.mempoolState.ApplyTransaction(uint64(m.mempool.Size()), tx, hashString, m.feeLimit)
	if err != nil {
		return err
	}
	if er = txn.Write(); er != nil {
		return store.NewTxWriteError(er)
	}
	return m.mempool.AddTransaction(tx)
}

func (m *Mempool) HandleFullMempool() lib.ErrorI {

}

func (m *Mempool) SetStore(store lib.StoreI) { m.mempoolStore = store }
