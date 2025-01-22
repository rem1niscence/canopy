package controller

import (
	"bytes"
	"github.com/canopy-network/canopy/bft"
	"github.com/canopy-network/canopy/fsm"
	"github.com/canopy-network/canopy/fsm/types"
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
	"math"
	"slices"
	"time"
)

// HandleTransaction() accepts or rejects inbound txs based on the mempool state
// - pass through call checking indexer and mempool for duplicate
func (c *Controller) HandleTransaction(tx []byte) lib.ErrorI {
	// lock the controller for thread safety
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

// ValidateCertificate() fully validates the proposal and resets back to begin block state
func (c *Controller) ValidateCertificate(qc *lib.QuorumCertificate, evidence *bft.ByzantineEvidence) (err lib.ErrorI) {
	// the base chain has specific logic to approve or reject proposals
	reset := c.ValidatorProposalConfig()
	defer func() { reset(); c.FSM.Reset() }()
	// validate the byzantine evidence portion of the proposal (bft is canopy controlled)
	if err = c.Consensus.ValidateByzantineEvidence(qc.Results.SlashRecipients, evidence); err != nil {
		return err
	}
	// ensure the block is not empty
	if qc.Block == nil {
		return lib.ErrNilBlock()
	}
	// ensure the results aren't empty
	if qc.Results == nil && qc.Results.RewardRecipients != nil {
		return lib.ErrNilCertResults()
	}
	// convert the block bytes into a Canopy block structure
	blk := new(lib.Block)
	if err = lib.Unmarshal(qc.Block, blk); err != nil {
		return
	}
	// basic structural validations of the block
	if err = blk.Check(c.Config.NetworkID, c.Config.ChainId); err != nil {
		return
	}
	// ensure the Proposal.BlockHash corresponds to the actual hash of the block
	blockHash, e := blk.Hash()
	if e != nil {
		return e
	}
	if bytes.Equal(qc.BlockHash, blockHash) {
		return lib.ErrMismatchBlockHash()
	}
	// validate the reward recipients
	if len(qc.Results.RewardRecipients.PaymentPercents) != 2 {
		return types.ErrInvalidNumberOfRewardRecipients()
	}
	// get the proposer address
	proposerPub, er := crypto.NewPublicKeyFromBytes(qc.ProposerKey)
	if er != nil {
		return lib.ErrPubKeyFromBytes(er)
	}
	// get the delegate and their cut from the state machine
	delegate, delegateCut, err := c.GetDelegateToReward(proposerPub.Address().Bytes())
	if err != nil {
		return
	}
	// validate the reward amount for the proposer
	proposerPaymentPercent := qc.Results.RewardRecipients.PaymentPercents[0]
	if proposerPaymentPercent.Percent != 100-delegateCut {
		return types.ErrInvalidProposerRewardPercent()
	}
	// validate the reward amount for the delegate
	delegatorPaymentPercent := qc.Results.RewardRecipients.PaymentPercents[1]
	if !bytes.Equal(delegatorPaymentPercent.Address, delegate) || delegatorPaymentPercent.Percent != delegateCut {
		return types.ErrInvalidDelegateReward(delegatorPaymentPercent.Address, delegatorPaymentPercent.Percent)
	}
	// play the block against the state machine
	blockResult, err := c.ApplyAndValidateBlock(blk, false)
	if err != nil {
		return
	}
	// validate the orders
	if qc.Results.Orders == nil {
		return types.ErrInvalidOrders()
	}
	// validate the buy orders
	buyOrders := c.FSM.ParseBuyOrders(blockResult)
	if !slices.Equal(buyOrders, qc.Results.Orders.BuyOrders) {
		return types.ErrInvalidBuyOrder()
	}
	// process the base-chain order book against the state
	closeOrders, resetOrders := c.FSM.ProcessBaseChainOrderBook(types.OrderBook{}, blockResult)
	// validate the close orders
	if !slices.Equal(closeOrders, qc.Results.Orders.CloseOrders) {
		return types.ErrInvalidBuyOrder()
	}
	// validate the reset orders
	if !slices.Equal(resetOrders, qc.Results.Orders.ResetOrders) {
		return types.ErrInvalidBuyOrder()
	}
	return
}

// ProduceProposal() uses the associated `plugin` to create a Proposal with the candidate block and the `bft` to populate the byzantine evidence
func (c *Controller) ProduceProposal(be *bft.ByzantineEvidence, vdf *crypto.VDF) (blk []byte, results *lib.CertificateResult, err lib.ErrorI) {
	reset := c.ValidatorProposalConfig()
	defer func() { reset(); c.FSM.Reset() }()
	pubKey, _ := crypto.BytesToBLS12381Public(c.PublicKey)
	height := c.FSM.Height()
	// load the previous block from the store
	lastBlock, err := c.FSM.LoadBlock(height - 1)
	if err != nil {
		return
	}
	// get the last quorum certificate
	qc, err := c.FSM.LoadCertificateHashesOnly(height - 1)
	if err != nil {
		return
	}
	// get the maximum possible size of the block
	maxBlockSize, err := c.FSM.GetMaxBlockSize()
	if err != nil {
		return
	}
	// extract the vdf iterations if any
	var vdfIterations uint64
	if vdf != nil {
		vdfIterations = vdf.Iterations
	}
	// re-validate all transactions in the mempool as a fail-safe
	c.Mempool.checkMempool()
	transactions, numTxs := c.Mempool.GetTransactions(maxBlockSize)
	// calculate self address
	selfAddress := pubKey.Address().Bytes()
	// create a block header structure
	header := &lib.BlockHeader{
		Height:                height + 1,                                               // increment the height
		NetworkId:             c.FSM.NetworkID,                                          // ensure only applicable for the proper network
		Time:                  uint64(time.Now().UnixMicro()),                           // set the time of the block
		NumTxs:                uint64(numTxs),                                           // set the number of transactions
		TotalTxs:              lastBlock.BlockHeader.TotalTxs + uint64(numTxs),          // calculate the total transactions
		LastBlockHash:         lastBlock.BlockHeader.LastBlockHash,                      // use the last block hash to 'chain' the blocks
		ProposerAddress:       selfAddress,                                              // set self as proposer address
		LastQuorumCertificate: qc,                                                       // add last QC to lock-in a commit certificate
		TotalVdfIterations:    lastBlock.BlockHeader.TotalVdfIterations + vdfIterations, // add last total iterations to current iterations
		Vdf:                   vdf,                                                      // attach the vdf proof
	}
	// create a block structure
	block := &lib.Block{
		BlockHeader:  header,
		Transactions: transactions,
	}
	// capture the tentative block result here
	blockResult := new(lib.BlockResult)
	// apply the block against the state machine to fill in the merkle `roots` for the block header
	block.BlockHeader, blockResult.Transactions, err = c.FSM.ApplyBlock(block)
	if err != nil {
		return
	}
	// update the block results
	blockResult.BlockHeader = block.BlockHeader
	// marshal the block into bytes
	blk, err = lib.Marshal(block)
	if err != nil {
		return
	}
	// get the delegate and their cut from the state machine
	delegate, delegateCut, err := c.GetDelegateToReward(selfAddress)
	if err != nil {
		return
	}
	// parse the last block for buy orders and polling
	buyOrders := c.FSM.ParseBuyOrders(blockResult)
	// process the base-chain order book against the state
	closeOrders, resetOrders := c.FSM.ProcessBaseChainOrderBook(types.OrderBook{}, blockResult)
	// set block reward recipients
	results = &lib.CertificateResult{
		RewardRecipients: &lib.RewardRecipients{
			PaymentPercents: []*lib.PaymentPercents{{
				Address: header.ProposerAddress, // proposer is a recipient of the reward
				Percent: 100 - delegateCut,      // proposer gets what's left after the delegate's cut
			},
				{
					Address: delegate,    // delegate is a recipient of the reward
					Percent: delegateCut, // delegates cut is a governance parameter
				},
			}},
		SlashRecipients: new(lib.SlashRecipients),
		Orders: &lib.Orders{
			BuyOrders:   buyOrders,
			ResetOrders: resetOrders,
			CloseOrders: closeOrders,
		},
		Checkpoint: nil,
	}
	// use the bft object to fill in the Byzantine Evidence
	results.SlashRecipients.DoubleSigners, err = c.Consensus.ProcessDSE(be.DSE.Evidence...)
	if err != nil {
		c.log.Warn(err.Error()) // still produce proposal
	}
	return
}

// CommitCertificate used after consensus decides on a block
// - applies block against the fsm
// - indexes the block and its transactions
// - removes block transactions from mempool
// - re-checks all transactions in mempool
// - atomically writes all to the underlying db
// - sets up the app for the next height
func (c *Controller) CommitCertificate(qc *lib.QuorumCertificate) lib.ErrorI {
	// reset the store once this code finishes
	// NOTE: if code execution gets to `store.Commit()` - this will effectively be a noop
	defer func() { c.FSM.Reset() }()
	c.log.Debugf("TryCommit block %s", lib.BytesToString(qc.ResultsHash))
	// load the store (db) object
	store := c.FSM.Store().(lib.StoreI)
	// convert the proposal.block (bytes) into Block structure
	blk := new(lib.Block)
	if err := lib.Unmarshal(qc.Block, blk); err != nil {
		return err
	}
	// apply the block against the state machine
	blockResult, err := c.ApplyAndValidateBlock(blk, true)
	if err != nil {
		return err
	}
	// index the quorum certificate in the store
	c.log.Debugf("Indexing quorum certificate for height %d", qc.Header.Height)
	if err = store.IndexQC(qc); err != nil {
		return err
	}
	// update the LastQuorumCertificate in the store
	//- this is to ensure that each node has the same COMMIT QC for the last block as multiple valid versions (a few less signatures) of the same QC could exist
	if blk.BlockHeader.Height != 1 {
		c.log.Debugf("Indexing last quorum certificate for height %d", blk.BlockHeader.LastQuorumCertificate.Header.Height)
		if err = store.IndexQC(blk.BlockHeader.LastQuorumCertificate); err != nil {
			return err
		}
	}
	// index the block in the store
	c.log.Debugf("Indexing block %d", blk.BlockHeader.Height)
	if err = store.IndexBlock(blockResult); err != nil {
		return err
	}
	// delete the block transactions in the mempool
	for _, tx := range blk.Transactions {
		c.log.Debugf("Tx %s was included in a block so removing from mempool", crypto.HashString(tx))
		c.Mempool.DeleteTransaction(tx)
	}
	// rescan mempool to ensure validity of all transactions
	c.log.Debug("Checking mempool for newly invalid transactions")
	c.Mempool.checkMempool()
	// parse committed block for straw polls
	c.FSM.ParsePollTransactions(blockResult)
	// atomically write this to the store
	c.log.Debug("Committing to store")
	if _, err = store.Commit(); err != nil {
		return err
	}
	c.log.Infof("Committed block %s at H:%d 🔒", lib.BytesToString(qc.ResultsHash), blk.BlockHeader.Height)
	c.log.Debug("Setting up FSM for next height")
	c.FSM, err = fsm.New(c.Config, store, c.log)
	if err != nil {
		return err
	}
	c.log.Debug("Setting up Mempool for next height")
	if c.Mempool.FSM, err = c.FSM.Copy(); err != nil {
		return err
	}
	if err = c.Mempool.FSM.BeginBlock(); err != nil {
		return err
	}
	c.log.Debug("Commit done")
	return nil
}

// ApplyAndValidateBlock() plays the block against the State Machine and
// compares the block header against a results from the state machine
func (c *Controller) ApplyAndValidateBlock(b *lib.Block, commit bool) (*lib.BlockResult, lib.ErrorI) {
	// basic structural checks on the block
	if err := b.Check(c.Config.NetworkID, c.Config.ChainId); err != nil {
		return nil, err
	}
	// define convenience variables
	blockHash, blockHeight := lib.BytesToString(b.BlockHeader.Hash), b.BlockHeader.Height
	// apply the block against the state machine
	c.log.Debugf("Applying block %s for height %d", blockHash, blockHeight)
	header, txResults, err := c.FSM.ApplyBlock(b)
	if err != nil {
		return nil, err
	}
	// compare the resulting header against the block header (should be identical)
	c.log.Debugf("Validating block header %s for height %d", blockHash, blockHeight)
	if err = c.CompareBlockHeaders(b.BlockHeader, header, commit); err != nil {
		return nil, err
	}
	// return a valid block result
	c.log.Infof("Block %s with %d txs is valid for height %d ✅ ", blockHash, len(b.Transactions), blockHeight)
	return &lib.BlockResult{
		BlockHeader:  b.BlockHeader,
		Transactions: txResults,
	}, nil
}

// CompareBlockHeaders() compares two block headers for equality and validates the last quorum certificate and block time
func (c *Controller) CompareBlockHeaders(candidate *lib.BlockHeader, compare *lib.BlockHeader, commit bool) lib.ErrorI {
	// compare the block headers for equality
	hash, e := compare.SetHash()
	if e != nil {
		return e
	}
	if !bytes.Equal(hash, candidate.Hash) {
		return lib.ErrUnequalBlockHash()
	}
	// validate the last quorum certificate of the block header
	// by using the historical committee
	if candidate.Height > 2 {
		lastCertificate, err := c.FSM.LoadCertificateHashesOnly(candidate.Height - 1)
		if err != nil {
			return err
		}
		// validate the expected QC hash
		if candidate.LastQuorumCertificate == nil ||
			candidate.LastQuorumCertificate.Header == nil ||
			!bytes.Equal(candidate.LastQuorumCertificate.BlockHash, lastCertificate.BlockHash) ||
			!bytes.Equal(candidate.LastQuorumCertificate.ResultsHash, lastCertificate.ResultsHash) {
			return lib.ErrInvalidLastQuorumCertificate()
		}
		// load the committee for the last qc
		committeeHeight := candidate.LastQuorumCertificate.Header.CanopyHeight
		vs, err := c.LoadCommittee(committeeHeight)
		if err != nil {
			return err
		}
		// check the last QC
		isPartialQC, err := candidate.LastQuorumCertificate.Check(vs, 0, &lib.View{
			Height:       candidate.Height - 1,
			CanopyHeight: committeeHeight,
			NetworkId:    c.Config.NetworkID,
			CommitteeId:  c.Config.ChainId,
		}, true)
		if err != nil {
			return err
		}
		// ensure is a full +2/3rd maj QC
		if isPartialQC {
			return lib.ErrNoMaj23()
		}
	}
	// check the timestamp if actively in BFT - else it's been validated by the validator set
	if !commit {
		// validate the timestamp in the block header
		if err := c.validateBlockTime(candidate); err != nil {
			return err
		}
	}
	return nil
}

// CheckPeerQc() performs a basic checks on a block received from a peer and returns if the self is out of sync
func (c *Controller) CheckPeerQC(maxHeight uint64, qc *lib.QuorumCertificate) (stillCatchingUp bool, err lib.ErrorI) {
	// convert the proposal block bytes into a block structure
	blk := new(lib.Block)
	if err = lib.Unmarshal(qc.Block, blk); err != nil {
		return
	}
	// ensure the unmarshalled block is not nil
	if blk == nil || blk.BlockHeader == nil {
		return false, lib.ErrNilBlock()
	}
	// ensure the Proposal.BlockHash corresponds to the actual hash of the block
	if !bytes.Equal(qc.BlockHash, crypto.Hash(qc.Block)) {
		err = lib.ErrMismatchBlockHash()
		return
	}
	// check the height of the block
	h := c.FSM.Height()
	// don't accept any blocks below the local height
	if h > blk.BlockHeader.Height {
		err = lib.ErrWrongMaxHeight()
		return
	}
	// don't accept any blocks greater than 'max height'
	if blk.BlockHeader.Height > maxHeight {
		err = lib.ErrWrongMaxHeight()
		return
	}
	// height of this block is not local height, the node is still catching up
	stillCatchingUp = h != blk.BlockHeader.Height
	return
}

// GetDelegateToReward() gets the pseudorandomly selected delegate to reward and their cut
func (c *Controller) GetDelegateToReward(proposerAddress []byte) (address []byte, delegateCut uint64, err lib.ErrorI) {
	// get the validator params in order to have the reward percentage for the delegate
	valParams, err := c.FSM.GetParamsVal()
	if err != nil {
		return
	}
	// set the percentage the delegate receives
	delegateCut = valParams.ValidatorDelegateRewardPercentage
	// get the delegate pseudorandom delegate
	address = c.BaseChainInfo.DelegateLotteryWinner
	if bytes.Equal(address, crypto.MaxHash[:20]) {
		address = proposerAddress
	}
	return
}

func (c *Controller) LoadMaxBlockSize() int {
	params, _ := c.FSM.GetParamsCons()
	if params == nil {
		return 0
	}
	return int(params.BlockSize) // TODO add with max header size here... as this param is only enforced at the txn level in other places in the code
}

// validateBlockTime() validates the timestamp in the block header for safe pruning
// accepts +/- hour variance for clock drift
func (c *Controller) validateBlockTime(header *lib.BlockHeader) lib.ErrorI {
	now := time.Now()
	maxTime, minTime, t := now.Add(time.Hour), now.Add(-time.Hour), time.UnixMicro(int64(header.Time))
	if minTime.Compare(t) > 0 || maxTime.Compare(t) < 0 {
		return lib.ErrInvalidBlockTime()
	}
	return nil
}

// ValidatorProposalConfig() is how the Validator is configured for `base chain` specific parameter upgrades
func (c *Controller) ValidatorProposalConfig() (reset func()) {
	if c.Consensus.GetRound() < 3 {
		// if the node is not having 'consensus issues' refer to the approve list
		c.FSM.SetProposalVoteConfig(types.GovProposalVoteConfig_APPROVE_LIST)
	} else {
		// if the node is exhibiting 'chain halt' like behavior, reject all proposals
		c.FSM.SetProposalVoteConfig(types.GovProposalVoteConfig_REJECT_ALL)
	}
	// a callback that resets the configuration back to default
	reset = func() {
		// the default is to accept all except in 'Consensus mode'
		c.FSM.SetProposalVoteConfig(types.AcceptAllProposals)
	}
	return
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

// Mempool accepts or rejects incoming txs based on the mempool (ephemeral copy) state
// - recheck when
//   - mempool dropped some percent of the lowest fee txs
//   - new tx has higher fee than the lowest
//
// - notes:
//   - new tx added may also be evicted, this is expected behavior
type Mempool struct {
	log           lib.LoggerI
	FSM           *fsm.StateMachine
	cachedResults lib.TxResults
	lib.Mempool
}

// NewMempool() creates a new instance of a Mempool structure
func NewMempool(fsm *fsm.StateMachine, config lib.MempoolConfig, log lib.LoggerI) (m *Mempool, err lib.ErrorI) {
	// initialize the structure
	m = &Mempool{
		log:     log,
		Mempool: lib.NewMempool(config),
	}
	// make an 'mempool (ephemeral copy) state' so the mempool can maintain only 'valid' transactions
	// despite dependencies and conflicts
	m.FSM, err = fsm.Copy()
	if err != nil {
		return nil, err
	}
	return m, err
}

// HandleTransaction() attempts to add a transaction to the mempool by validating, adding, and evicting overfull or newly invalid txs
func (m *Mempool) HandleTransaction(tx []byte) lib.ErrorI {
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
