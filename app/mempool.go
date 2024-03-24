package app

import (
	"encoding/hex"
	"fmt"
	"github.com/ginchuco/ginchu/crypto"
	"github.com/ginchuco/ginchu/state_machine"
	"github.com/ginchuco/ginchu/state_machine/types"
	lib "github.com/ginchuco/ginchu/types"
)

type Mempool struct {
	mempool      lib.Mempool
	mempoolStore lib.StoreI // ephemeral state to have safety within the mempool
	mempoolState state_machine.StateMachine
}

// HandleTransaction handles mempool acceptance from incoming transactions
// It alone decides if the transaction is valid based on the mempool state
func (m *Mempool) HandleTransaction(tx, h []byte) lib.ErrorI {
	if hash := hex.EncodeToString(h); m.mempool.Contains(hash) {
		return types.ErrTxFoundInMempool(hash)
	}
	fee, err := m.ApplyTransaction(tx)
	if err != nil {
		return err
	}
	recheck, err := m.mempool.AddTransaction(tx, fee)
	if err != nil {
		return err
	}
	if recheck {
		return m.CheckMempool()
	}
	return nil
}

func (m *Mempool) CheckMempool() lib.ErrorI {
	if err := m.ResetMempoolState(); err != nil {
		return err
	}
	var toDelete [][]byte
	it := m.mempool.Iterator()
	defer it.Close()
	for ; it.Valid(); it.Next() {
		tx := it.Key()
		if _, err := m.ApplyTransaction(tx); err != nil {
			fmt.Println(err)
			toDelete = append(toDelete, tx)
		}
	}
	for _, tx := range toDelete {
		if err := m.mempool.DeleteTransaction(tx); err != nil {
			return err
		}
	}
	return nil
}

func (m *Mempool) ApplyTransaction(tx []byte) (fee string, err lib.ErrorI) {
	txn, cleanup := m.TxnWrap()
	defer cleanup()
	result, err := m.mempoolState.ApplyTransaction(uint64(m.mempool.Size()), tx, crypto.HashString(tx))
	if err != nil {
		return "", err
	}
	if err = txn.Write(); err != nil {
		return "", err
	}
	return result.Transaction.Fee, nil
}

func (m *Mempool) TxnWrap() (txn lib.StoreTxnI, cleanup func()) {
	txn = m.mempoolStore.NewTxn()
	m.mempoolState.SetStore(txn)
	cleanup = func() {
		m.mempoolState.SetStore(m.mempoolStore)
		txn.Discard()
	}
	return
}

func (m *Mempool) ResetMempoolState() lib.ErrorI {
	return m.mempoolStore.Reset()
}

func (m *Mempool) SetStore(store lib.StoreI) { m.mempoolStore = store }
