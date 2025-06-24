package controller

import (
	"fmt"
	"github.com/canopy-network/canopy/fsm"
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
	"github.com/canopy-network/canopy/p2p"
	lru "github.com/hashicorp/golang-lru/v2"
	"math"
	"sync/atomic"
	"time"
)

/* This file implements logic for transaction sending and handling as well as memory pooling */

// SendTxMsg() routes a locally generated transaction message to the listener for processing + gossiping
func (c *Controller) SendTxMsg(tx []byte) lib.ErrorI {
	// create a transaction message object using the tx bytes and the chain id
	msg := &lib.TxMessage{ChainId: c.Config.ChainId, Tx: tx}
	// send the transaction message to the listener using internal routing
	return c.P2P.SelfSend(c.PublicKey, Tx, msg)
}

// ListenForTx() listen for inbound tx messages, internally route them, and gossip them to peers
func (c *Controller) ListenForTx() {
	// create a new message cache to filter out duplicate transaction messages
	cache := lib.NewMessageCache()
	// wait and execute for each inbound transaction message
	for msg := range c.P2P.Inbox(Tx) {
		// if the chain is syncing, just return without handling
		if c.isSyncing.Load() {
			// exit
			continue
		}
		func() {
			c.log.Debug("Handling transaction")
			//defer lib.TimeTrack(c.log, time.Now())
			// lock the controller for thread safety
			c.Lock()
			// unlock when this iteration completes
			defer c.Unlock()
			// check and add the message to the cache to prevent duplicates
			if ok := cache.Add(msg); !ok {
				// if duplicate, exit
				return
			}
			// create a convenience variable for the identity of the sender
			senderID := msg.Sender.Address.PublicKey
			// try to cast the p2p message as a tx message
			txMsg, ok := msg.Message.(*lib.TxMessage)
			// if the cast failed
			if !ok {
				// log the unexpected behavior
				c.log.Warnf("Non-Tx message from %s", lib.BytesToTruncatedString(senderID))
				// slash the peer's reputation score
				c.P2P.ChangeReputation(senderID, p2p.InvalidMsgRep)
				// exit
				return
			}
			// if the message is empty
			if txMsg == nil {
				// log the unexpected behavior
				c.log.Warnf("Empty tx message from %s", lib.BytesToTruncatedString(senderID))
				// slash the peers reputation score
				c.P2P.ChangeReputation(senderID, p2p.InvalidMsgRep)
				// exit
				return
			}
			// route the transaction to the mempool handler
			if err := c.HandleTransaction(txMsg.Tx); err != nil {
				// if the error is 'mempool already has it'
				if err.Error() == lib.ErrTxFoundInMempool(crypto.HashString(txMsg.Tx)).Error() {
					// exit
					return
				}
				// else - warn of the error
				c.log.Warnf("Handle tx from %s failed with err: %s", lib.BytesToTruncatedString(senderID), err.Error())
				// slash the peers reputation score
				c.P2P.ChangeReputation(senderID, p2p.InvalidTxRep)
				// exit
				return
			}
			// onto the next message
		}()
	}
}

// HandleTransaction() accepts or rejects inbound txs based on the mempool state
// - pass through call checking indexer and mempool for duplicate
func (c *Controller) HandleTransaction(tx []byte) lib.ErrorI {
	// generate hash bytes for the transaction
	hash := crypto.Hash(tx)
	// generate a hex encoded hash string for the transaction
	hashString := lib.BytesToString(hash)
	// get the transaction from the indexer using the hash bytes
	txResult, err := c.FSM.Store().(lib.StoreI).GetTxByHash(hash)
	// if an error occurred attempting to retrieve the tx
	if err != nil {
		// exit with the error
		return err
	}
	// if the transaction already exists
	if txResult.TxHash != "" {
		// exit with duplicate transaction error
		return lib.ErrDuplicateTx(hashString)
	}
	// ensure the mempool doesn't already contain the transaction
	if c.Mempool.Contains(hashString) {
		// if it does, exit with already found in the mempool
		return lib.ErrTxFoundInMempool(hashString)
	}
	// route the transaction to the mempool for handling
	return c.Mempool.HandleTransaction(tx)
}

// GetProposalBlockFromMempool() returns the cached proposal block
// if there's no cached block - call CheckMempool then return
func (c *Controller) GetProposalBlockFromMempool() (*lib.Block, *lib.BlockResult) {
	// if the cached proposal is nil
	if c.Mempool.cachedProposal == nil {
		// check the mempool 'on-demand'
		c.Mempool.CheckMempool()
	}
	// return the block
	return c.Mempool.cachedProposal.Block, c.Mempool.cachedProposal.BlockResult
}

// CheckMempool() periodically checks the mempool:
// - Validates all transactions in the mempool
// - Caches a proposal block based on the current state and the mempool transactions
// - P2P Gossip out any transactions that weren't previously gossiped
func (c *Controller) CheckMempool() {
	deDupe, _ := lru.New[string, struct{}](100_000)
	for range time.Tick(time.Second) {
		// if no-recheck needed
		if !c.Mempool.Recheck.Load() {
			continue
		}
		// lock the controller for thread safety
		c.Lock()
		// check the mempool to cache a proposal block and validate the mempool itself
		c.Mempool.CheckMempool()
		// get the transactions to gossip
		toGossip := c.Mempool.GetTransactions(math.MaxUint64)
		// unlock the controller
		c.Unlock()
		// for each transaction to gossip
		for _, tx := range toGossip {
			// get the key for the transaction
			key := crypto.HashString(tx)
			// if not already gossiped
			if _, found := deDupe.Get(key); !found {
				// add to the de-dupe list
				deDupe.Add(key, struct{}{})
				// gossip the transaction to peers
				if err := c.P2P.SendToPeers(Tx, &lib.TxMessage{ChainId: c.Config.ChainId, Tx: tx}); err != nil {
					// log the gossip error
					c.log.Error(fmt.Sprintf("unable to gossip tx with err: %s", err.Error()))
				}
			}
		}
		// set 're-check' to false
		c.Mempool.Recheck.Swap(false)
	}
}

// Mempool accepts or rejects incoming txs based on the mempool (ephemeral copy) state
// - recheck when
//   - mempool dropped some percent of the lowest fee txs
//   - new tx has higher fee than the lowest
//
// - notes:
//   - new tx added may also be evicted, this is expected behavior
type Mempool struct {
	lib.Mempool                        // the memory pool itself defined as an interface
	Recheck         atomic.Bool        // a signal to Recheck the mempool
	FSM             *fsm.StateMachine  // the ephemeral finite state machine used to validate inbound transactions
	cachedResults   lib.TxResults      // a memory cache of transaction results for efficient verification
	cachedFailedTxs *lib.FailedTxCache // a memory cache of failed transactions for tracking
	metrics         *lib.Metrics       // telemetry
	address         crypto.AddressI    // validator identity
	cachedProposal  *CachedProposal    // the cached block proposal
	log             lib.LoggerI        // the logger
}

type CachedProposal struct {
	Block       *lib.Block
	BlockResult *lib.BlockResult
}

// NewMempool() creates a new instance of a Mempool structure
func NewMempool(fsm *fsm.StateMachine, address crypto.AddressI, config lib.MempoolConfig, metrics *lib.Metrics, log lib.LoggerI) (m *Mempool, err lib.ErrorI) {
	// initialize the structure
	m = &Mempool{
		Mempool:         lib.NewMempool(config),
		Recheck:         atomic.Bool{},
		cachedFailedTxs: lib.NewFailedTxCache(),
		metrics:         metrics,
		address:         address,
		log:             log,
	}
	// make an 'mempool (ephemeral copy) state' so the mempool can maintain only 'valid' transactions despite dependencies and conflicts
	m.FSM, err = fsm.Copy()
	// if an error occurred copying the fsm
	if err != nil {
		return nil, err
	}
	// exit
	return m, err
}

// HandleTransaction() attempts to add a transaction to the mempool by validating, adding, and evicting overfull or newly invalid txs
func (m *Mempool) HandleTransaction(tx []byte) (err lib.ErrorI) {
	// upon completing this function
	defer func() {
		// in an error occurred while handling this transaction
		if err != nil {
			// cache failed txs for RPC display
			m.cachedFailedTxs.Add(tx, crypto.HashString(tx), err)
		}
	}()
	// create a new transaction object reference to ensure a non-nil transaction
	transaction := new(lib.Transaction)
	// populate the object ref with the bytes of the transaction
	if err = lib.Unmarshal(tx, transaction); err != nil {
		return
	}
	// perform basic validations against the tx object
	if err = transaction.CheckBasic(); err != nil {
		return
	}
	// extract the fee from the transaction result
	fee := transaction.Fee
	// if the transaction is a special type: 'certificate result'
	if transaction.MessageType == fsm.MessageCertificateResultsName {
		// prioritize certificate result transactions by artificially raising the fee 'stored fee'
		fee = math.MaxUint32
	}
	// signal a recheck
	m.Recheck.Swap(true)
	// add a transaction to the mempool
	if _, err = m.AddTransaction(tx, fee); err != nil {
		// exit with the error
		return
	}
	m.log.Debugf("Added tx %s to mempool for checking", crypto.HashString(tx))
	// exit
	return
}

// CheckMempool() Checks each transaction in the mempool and caches a block proposal
func (m *Mempool) CheckMempool() {
	defer lib.TimeTrack(m.log, time.Now())
	var err lib.ErrorI
	// reset the mempool (ephemeral copy) state to just after the automatic 'begin block' phase
	m.FSM.Reset()
	// create the actual block structure with the maximum amount of transactions allowed or available in the mempool
	block := &lib.Block{
		BlockHeader:  &lib.BlockHeader{Time: uint64(time.Now().UnixMicro()), ProposerAddress: m.address.Bytes()},
		Transactions: m.GetTransactions(math.MaxUint64), // get all transactions in mempool - but apply block will only keep 'max-block' amount
	}
	// capture the tentative block result using a new object reference
	blockResult, failed := new(lib.BlockResult), make([][]byte, 0)
	// apply the block against the state machine and populate the resulting merkle `roots` in the block header
	block.BlockHeader, blockResult.Transactions, failed, err = m.FSM.ApplyBlock(block, true)
	if err != nil {
		m.log.Errorf("Check Mempool error: %s", err.Error())
		return
	}
	// cache the proposal
	m.cachedProposal = &CachedProposal{
		Block:       block,
		BlockResult: blockResult,
	}
	// evict all invalid transactions from the mempool
	for _, tx := range failed {
		m.log.Infof("Removed tx %s from mempool", crypto.HashString(tx))
		// delete the transaction
		m.DeleteTransaction(tx)
	}
	// reset the RPC cached results
	m.cachedResults = nil
	// add results to cache
	for _, result := range blockResult.Transactions {
		// cache the results
		m.cachedResults = append(m.cachedResults, result)
	}
	// update the mempool metrics
	m.metrics.UpdateMempoolMetrics(m.Mempool.TxCount(), m.Mempool.TxsBytes())
}

// GetPendingPage() returns a page of unconfirmed mempool transactions
func (c *Controller) GetPendingPage(p lib.PageParams) (page *lib.Page, err lib.ErrorI) {
	// lock the controller for thread safety
	c.Lock()
	// unlock the controller when the function completes
	defer c.Unlock()
	// create a new page and transaction results list to populate
	page, txResults := lib.NewPage(p, lib.PendingResultsPageName), make(lib.TxResults, 0)
	// define a callback to execute when loading the page
	callback := func(item any) (e lib.ErrorI) {
		// cast the item to a transaction result pointer
		v, ok := item.(*lib.TxResult)
		// if the cast failed
		if !ok {
			// exit with error
			return lib.ErrInvalidMessageCast()
		}
		// add to the list
		txResults = append(txResults, v)
		// exit callback
		return
	}
	// populate the page using the 'cached results'
	err = page.LoadArray(c.Mempool.cachedResults, &txResults, callback)
	// exit
	return
}

// GetFailedTxsPage() returns a list of failed mempool transactions
func (c *Controller) GetFailedTxsPage(address string, p lib.PageParams) (page *lib.Page, err lib.ErrorI) {
	// lock the controller for thread safety
	c.Lock()
	// unlock the controller when the function completes
	defer c.Unlock()
	// create a new page and failed transaction results list to populate
	page, failedTxs := lib.NewPage(p, lib.FailedTxsPageName), make(lib.FailedTxs, 0)
	// define a callback to execute when loading the page
	callback := func(item any) (e lib.ErrorI) {
		// cast the item to a 'failed transaction' object
		v, ok := item.(*lib.FailedTx)
		// if the cast failed
		if !ok {
			// exit with error
			return lib.ErrInvalidMessageCast()
		}
		// add to the failed list
		failedTxs = append(failedTxs, v)
		// exit callback
		return
	}
	// populate the page using the 'failed cache'
	err = page.LoadArray(c.Mempool.cachedFailedTxs.GetFailedForAddress(address), &failedTxs, callback)
	// exit
	return
}
