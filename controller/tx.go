package controller

import (
	"fmt"
	"github.com/canopy-network/canopy/fsm"
	"github.com/canopy-network/canopy/fsm/types"
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
	"github.com/canopy-network/canopy/p2p"
	"math"
)

// SendTxMsg() gossips a Transaction through the P2P network for a specific chainId
func (c *Controller) SendTxMsg(tx []byte) lib.ErrorI {
	// create a transaction message object using the tx bytes and the chain id
	msg := &lib.TxMessage{ChainId: c.Config.ChainId, Tx: tx}

	// send transaction to controller for processing and gossip
	return c.P2P.SelfSend(c.PublicKey, Tx, msg)
}

// ListenForTx() listen for inbound tx messages, internally route them, and gossip them to peers
func (c *Controller) ListenForTx() {
	// create a new message cache to filter out duplicate transaction messages
	cache := lib.NewMessageCache()
	// wait and execute for each inbound transaction message
	for msg := range c.P2P.Inbox(Tx) {
		func() {
			// lock the controller for thread safety
			c.Lock()
			defer c.Unlock()
			// check and add the message to the cache to prevent duplicates
			if ok := cache.Add(msg); !ok {
				return
			}
			// create a convenience variable for the identity of the sender
			senderID := msg.Sender.Address.PublicKey
			// try to cast the p2p message as a tx message
			txMsg, ok := msg.Message.(*lib.TxMessage)
			// if the cast failed
			if !ok {
				c.log.Warnf("Non-Tx message from %s", lib.BytesToTruncatedString(senderID))
				c.P2P.ChangeReputation(senderID, p2p.InvalidMsgRep)
				return
			}
			// if the message is empty
			if txMsg == nil {
				c.log.Warnf("Empty tx message from %s", lib.BytesToTruncatedString(senderID))
				c.P2P.ChangeReputation(senderID, p2p.InvalidMsgRep)
				return
			}
			// if the chain is syncing, just return without handling
			if c.isSyncing.Load() {
				return
			}
			// handle the transaction under the plugin
			if err := c.HandleTransaction(txMsg.Tx); err != nil {
				if err.Error() == lib.ErrTxFoundInMempool(crypto.HashString(txMsg.Tx)).Error() {
					return
				}
				c.log.Warnf("Handle tx from %s failed with err: %s", lib.BytesToTruncatedString(senderID), err.Error())
				c.P2P.ChangeReputation(senderID, p2p.InvalidTxRep)
				return
			}
			c.log.Infof("Received valid transaction %s from %s for chain %d", crypto.ShortHashString(txMsg.Tx), lib.BytesToString(senderID)[:20], txMsg.ChainId)
			// bump peer reputation positively
			c.P2P.ChangeReputation(senderID, p2p.GoodTxRep)
			// gossip the transaction to peers
			if err := c.P2P.SendToPeers(Tx, msg.Message, lib.BytesToString(senderID)); err != nil {
				c.log.Error(fmt.Sprintf("unable to gossip tx with err: %s", err.Error()))
			}
		}()

	}
}

// HandleTransaction() accepts or rejects inbound txs based on the mempool state
// - pass through call checking indexer and mempool for duplicate
func (c *Controller) HandleTransaction(tx []byte) lib.ErrorI {
	hash := crypto.Hash(tx)
	hashString := lib.BytesToString(hash)
	// indexer
	txResult, err := c.FSM.Store().(lib.StoreI).GetTxByHash(hash)
	if err != nil {
		return err
	}
	if txResult.TxHash != "" {
		return lib.ErrDuplicateTx(hashString)
	}
	// mempool
	if c.Mempool.Contains(hashString) {
		return lib.ErrTxFoundInMempool(hashString)
	}
	return c.Mempool.HandleTransaction(tx)
}

// Mempool accepts or rejects incoming txs based on the mempool (ephemeral copy) state
// - recheck when
//   - mempool dropped some percent of the lowest fee txs
//   - new tx has higher fee than the lowest
//
// - notes:
//   - new tx added may also be evicted, this is expected behavior
type Mempool struct {
	log             lib.LoggerI
	FSM             *fsm.StateMachine
	cachedResults   lib.TxResults
	cachedFailedTxs *lib.FailedTxCache
	lib.Mempool
}

// NewMempool() creates a new instance of a Mempool structure
func NewMempool(fsm *fsm.StateMachine, config lib.MempoolConfig, log lib.LoggerI) (m *Mempool, err lib.ErrorI) {
	// initialize the structure
	m = &Mempool{
		log:             log,
		Mempool:         lib.NewMempool(config),
		cachedFailedTxs: lib.NewFailedTxCache(),
	}
	// make an 'mempool (ephemeral copy) state' so the mempool can maintain only 'valid' transactions
	// despite dependencies and conflicts
	m.FSM, err = fsm.Copy()
	if err != nil {
		return nil, err
	}
	m.FSM.ResetToBeginBlock()
	return m, err
}

// HandleTransaction() attempts to add a transaction to the mempool by validating, adding, and evicting overfull or newly invalid txs
func (m *Mempool) HandleTransaction(tx []byte) (err lib.ErrorI) {
	defer func() {
		// cache failed txs for RPC display
		if err != nil {
			m.cachedFailedTxs.Add(tx, crypto.HashString(tx), err)
		}
	}()

	// validate the transaction against the mempool (ephemeral copy) state
	result, err := m.applyAndWriteTx(tx)
	if err != nil {
		return err
	}
	fee := result.Transaction.Fee
	// prioritize certificate result transactions by artificially raising the fee 'stored fee'
	if result.MessageType == types.MessageCertificateResultsName {
		fee = math.MaxUint32
	}
	// add a transaction to the mempool
	recheck, err := m.AddTransaction(tx, fee)
	if err != nil {
		return err
	}
	// cache the results for RPC display
	m.log.Infof("Added tx %s to mempool for checking", crypto.HashString(tx))
	m.cachedResults = append(m.cachedResults, result)
	// recheck the mempool if necessary
	if recheck {
		m.checkMempool()
	}
	return nil
}

// checkMempool() validates all transactions the mempool using the mempool (ephemeral copy) state and evicts any that are invalid
func (m *Mempool) checkMempool() {
	// reset the mempool (ephemeral copy) state to just after the automatic 'begin block' phase
	m.FSM.ResetToBeginBlock()
	// reset the RPC cached results
	m.cachedResults = nil
	// define convenience variables
	var remove [][]byte
	// create an iterator for the mempool
	it := m.Iterator()
	defer it.Close()
	// for each mempool transaction
	for ; it.Valid(); it.Next() {
		// write the transaction to the state machine
		tx := it.Key()
		result, err := m.applyAndWriteTx(tx)
		if err != nil {
			// if invalid, add to the remove list
			m.log.Error(err.Error())
			remove = append(remove, tx)
			// and cache it
			m.cachedFailedTxs.Add(tx, crypto.HashString(tx), err)
			continue
		}
		// cache the results
		m.cachedResults = append(m.cachedResults, result)
	}
	// evict all 'newly' invalid transactions from the mempool
	for _, tx := range remove {
		m.log.Infof("removed tx %s from mempool", crypto.HashString(tx))
		m.DeleteTransaction(tx)
	}
}

// applyAndWriteTx() checks the validity of a transaction by playing it against the mempool (ephemeral copy) state machine
func (m *Mempool) applyAndWriteTx(tx []byte) (result *lib.TxResult, err lib.ErrorI) {
	store := m.FSM.Store()
	// wrap the store in a transaction in case a rollback to the previous valid transaction is needed
	txn, err := m.FSM.TxnWrap()
	if err != nil {
		return nil, err
	}
	// at the end of this code, reset the state machine store and discard the transaction
	defer func() { m.FSM.SetStore(store); txn.Discard() }()
	// apply the transaction to the mempool (ephemeral copy) state machine
	result, err = m.FSM.ApplyTransaction(uint64(m.TxCount()), tx, crypto.HashString(tx))
	if err != nil {
		// if invalid return error
		return nil, err
	}
	// write the transaction to the mempool store
	if err = txn.Write(); err != nil {
		return nil, err
	}
	// return the result
	return result, nil
}

// GetPendingPage() returns a page of unconfirmed mempool transactions
func (c *Controller) GetPendingPage(p lib.PageParams) (page *lib.Page, err lib.ErrorI) {
	// lock the controller for thread safety
	c.Lock()
	defer c.Unlock()
	page, txResults := lib.NewPage(p, lib.PendingResultsPageName), make(lib.TxResults, 0)
	err = page.LoadArray(c.Mempool.cachedResults, &txResults, func(i any) lib.ErrorI {
		v, ok := i.(*lib.TxResult)
		if !ok {
			return lib.ErrInvalidArgument()
		}
		txResults = append(txResults, v)
		return nil
	})
	return
}

// GetFailedTxsPage() returns a list of failed mempool transactions
func (c *Controller) GetFailedTxsPage(address string, p lib.PageParams) (page *lib.Page, err lib.ErrorI) {
	// lock the controller for thread safety
	c.Lock()
	defer c.Unlock()
	page, failedTxs := lib.NewPage(p, lib.FailedTxsPageName), make(lib.FailedTxs, 0)
	err = page.LoadArray(c.Mempool.cachedFailedTxs.GetAddr(address), &failedTxs, func(i any) lib.ErrorI {
		v, ok := i.(*lib.FailedTx)
		if !ok {
			return lib.ErrInvalidArgument()
		}
		failedTxs = append(failedTxs, v)
		return nil
	})
	return
}
