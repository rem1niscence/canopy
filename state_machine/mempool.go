package state_machine

import (
	"github.com/ginchuco/ginchu/state_machine/types"
	lib "github.com/ginchuco/ginchu/types"
)

type Mempool struct {
	*State
	lib.Mempool
}

func NewMempool(state *State, config types.MempoolConfig) *Mempool {
	return &Mempool{
		Mempool: types.NewMempool(config),
		State:   state,
	}
}

// HandleTransaction accepts or rejects incoming txs based on the mempool state
// - recheck when
//   - mempool dropped some percent of the lowest fee txs
//   - new tx has higher fee than the lowest
//
// - notes:
//   - new tx added may also be evicted, this is expected behavior
func (m *Mempool) HandleTransaction(tx []byte) lib.ErrorI {
	fee, err := m.applyAndWriteTx(tx)
	if err != nil {
		return err
	}
	recheck, err := m.AddTransaction(tx, fee)
	if err != nil {
		return err
	}
	if recheck {
		return m.checkMempool()
	}
	return nil
}

func (m *Mempool) checkMempool() lib.ErrorI {
	m.resetToBeginBlock()
	var remove [][]byte
	m.recheckAll(func(tx []byte, err lib.ErrorI) {
		m.log.Error(err.Error())
		remove = append(remove, tx)
	})
	for _, tx := range remove {
		m.DeleteTransaction(tx)
	}
	return nil
}

func (m *Mempool) recheckAll(errorCallback func([]byte, lib.ErrorI)) {
	it := m.Iterator()
	defer it.Close()
	for ; it.Valid(); it.Next() {
		tx := it.Key()
		if _, err := m.applyAndWriteTx(tx); err != nil {
			errorCallback(tx, err)
		}
	}
}

func (m *Mempool) applyAndWriteTx(tx []byte) (fee string, err lib.ErrorI) {
	txn, cleanup := m.txnWrap()
	defer cleanup()
	result, err := m.applyTransaction(tx, m.Size())
	if err != nil {
		return "", err
	}
	if err = txn.Write(); err != nil {
		return "", err
	}
	return result.Transaction.Fee, nil
}
