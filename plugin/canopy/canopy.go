package canopy

import (
	"bytes"
	"github.com/canopy-network/canopy/fsm"
	"github.com/canopy-network/canopy/fsm/types"
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
	"github.com/canopy-network/canopy/plugin"
	"math"
	"time"
)

/*
	This file implements the Canopy Plugin
	NOTE: Most plugins will simply be interfaces into the API of a third party software,
    but Canopy is unique as it's both the base blockchain and a Committee that requires a `Plugin`.
*/

// canopy.Plugin implements the CanopyPlugin interface
var _ plugin.CanopyPlugin = new(Plugin)

const (
	// BlockTimeMinusConsensus is the block time less the time it takes to complete the BFT phase
	BlockTimeMinusConsensus = 9*time.Minute + 40*time.Second
)

// canopy.Plugin is the Canopy implementation of the plugin.Plugin interface
type Plugin struct {
	FSM                      *fsm.StateMachine
	Mempool                  *Mempool
	PublicKey                []byte
	PrivateKey               crypto.PrivateKeyI
	Config                   lib.Config
	log                      lib.LoggerI
	bftTimerFinishedCallback plugin.BftTimerFinishedCallback
	gossipTxCallback         plugin.GossipTxCallback
	CommitteeId              uint64
	resetBFTTrigger          chan struct{}
}

// RegisterNew() registers a new instance of a canopy.Plugin with the RegisteredPlugins list
func RegisterNew(c lib.Config, committeeId uint64, valKey crypto.PrivateKeyI, db lib.StoreI, l lib.LoggerI) lib.ErrorI {
	sm, err := fsm.New(c, db, l)
	if err != nil {
		return err
	}
	mempool, err := NewMempool(sm, c.MempoolConfig, l)
	if err != nil {
		return err
	}
	plugin.RegisteredPlugins[lib.CanopyCommitteeId] = &Plugin{
		FSM:             sm,
		Mempool:         mempool,
		PublicKey:       valKey.PublicKey().Bytes(),
		PrivateKey:      valKey,
		Config:          c,
		log:             l,
		CommitteeId:     committeeId,
		resetBFTTrigger: make(chan struct{}, 1),
	}
	return nil
}

// WithCallbacks() allows the Controller to set the Plugin controlled callback functions
func (p *Plugin) WithCallbacks(bftTimerFinishedCallback plugin.BftTimerFinishedCallback, gossipTxCallback plugin.GossipTxCallback) {
	p.bftTimerFinishedCallback, p.gossipTxCallback = bftTimerFinishedCallback, gossipTxCallback
}

// Start() begins the plugin service
func (p *Plugin) Start() {
	for {
		timer := lib.NewTimer() // timer waiting for first reset trigger
		select {
		case <-p.resetBFTTrigger: // controller commands a reset and start of the bft trigger timer
			lib.ResetTimer(timer, BlockTimeMinusConsensus)
		case <-timer.C: // tell controller the bft trigger timer finished
			lib.StopTimer(timer)
			p.bftTimerFinishedCallback(lib.CanopyCommitteeId)
		}
	}
}

// ProduceProposal() creates a new Proposal using the Mempool
func (p *Plugin) ProduceProposal(vdf *lib.VDF) (blockBytes []byte, rewardRecipients *lib.RewardRecipients, err lib.ErrorI) {
	// reset the state machine once this code completes
	defer func() { p.FSM.Reset() }()
	pubKey, _ := crypto.BytesToBLS12381Public(p.PublicKey)
	height := p.FSM.Height()
	// load the previous block from the store
	lastBlock, err := p.FSM.LoadBlock(height - 1)
	if err != nil {
		return
	}
	// get the Quorum Certificate without the Proposal
	qc, err := p.FSM.LoadCertificateHashesOnly(height - 1)
	if err != nil {
		return
	}
	// get the maximum possible size of the block
	maxBlockSize, err := p.FSM.GetMaxBlockSize()
	if err != nil {
		return
	}
	// re-validate all transactions in the mempool as a fail-safe
	p.Mempool.checkMempool()
	transactions, numTxs := p.Mempool.GetTransactions(maxBlockSize)
	// create a block header structure
	header := &lib.BlockHeader{
		Height:                height + 1,                                                // increment the height
		NetworkId:             p.FSM.NetworkID,                                           // ensure only applicable for the proper network
		Time:                  uint64(time.Now().UnixMicro()),                            // set the time of the block
		NumTxs:                uint64(numTxs),                                            // set the number of transactions
		TotalTxs:              lastBlock.BlockHeader.TotalTxs + uint64(numTxs),           // calculate the total transactions
		LastBlockHash:         lastBlock.BlockHeader.LastBlockHash,                       // use the last block hash to 'chain' the blocks
		ProposerAddress:       pubKey.Address().Bytes(),                                  // set self as proposer address
		LastQuorumCertificate: qc,                                                        // add last QC to lock-in a commit certificate
		TotalVdfIterations:    lastBlock.BlockHeader.TotalVdfIterations + vdf.Iterations, // add last total iterations to current iterations
		Vdf:                   vdf,                                                       // attach the vdf proof
	}
	// create a block structure
	block := &lib.Block{
		BlockHeader:  header,
		Transactions: transactions,
	}
	// apply the block against the state machine to fill in the merkle `roots` for the block header
	block.BlockHeader, _, err = p.FSM.ApplyBlock(block)
	if err != nil {
		return
	}
	// marshal the block into bytes
	blockBytes, err = lib.Marshal(block)
	if err != nil {
		return
	}
	// set block reward recipients
	rewardRecipients = &lib.RewardRecipients{
		PaymentPercents: []*lib.PaymentPercents{{
			Address: header.ProposerAddress, // self is recipient of the reward
			Percent: 100,                    // self gets 100% of the reward
		}}}
	return
}

// ValidateCertificate() validates a Quorum Certificate from the Leader using the Canopy state machine
func (p *Plugin) ValidateCertificate(_ uint64, qc *lib.QuorumCertificate) (err lib.ErrorI) {
	// reset the state machine once this code completes
	defer func() { p.FSM.Reset() }()
	// ensure the block is not empty
	if qc.Block == nil {
		return lib.ErrNilBlock()
	}
	// convert the block bytes into a Canopy block structure
	blk := new(lib.Block)
	if err = lib.Unmarshal(qc.Block, blk); err != nil {
		return
	}
	// basic structural validations of the block
	if err = blk.Check(p.Config.NetworkID, p.CommitteeId); err != nil {
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
	// play the block against the state machine
	_, err = p.ApplyAndValidateBlock(blk)
	return
}

func (p *Plugin) LoadMaxBlockSize() int {
	params, _ := p.FSM.GetParamsCons()
	if params == nil {
		return 0
	}
	return int(params.BlockSize) // TODO add with max header size here... as this param is only enforced at the txn level in other places in the code
}

// CheckPeerQc() performs a basic checks on a block received from a peer and returns if the self is out of sync
func (p *Plugin) CheckPeerQC(maxHeight uint64, qc *lib.QuorumCertificate) (stillCatchingUp bool, err lib.ErrorI) {
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
	h := p.FSM.Height()
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

// ApplyAndValidateBlock() plays the block against the State Machine and
// compares the block header against a results from the state machine
func (p *Plugin) ApplyAndValidateBlock(b *lib.Block) (*lib.BlockResult, lib.ErrorI) {
	// basic structural checks on the block
	if err := b.Check(p.Config.NetworkID, p.CommitteeId); err != nil {
		return nil, err
	}
	// define convenience variables
	blockHash, blockHeight := lib.BytesToString(b.BlockHeader.Hash), b.BlockHeader.Height
	// apply the block against the state machine
	p.log.Debugf("Applying block %s for height %d", blockHash, blockHeight)
	header, txResults, err := p.FSM.ApplyBlock(b)
	if err != nil {
		return nil, err
	}
	// compare the resulting header against the block header (should be identical)
	p.log.Debugf("Validating block header %s for height %d", blockHash, blockHeight)
	if err = p.CompareBlockHeaders(b.BlockHeader, header); err != nil {
		return nil, err
	}
	// return a valid block result
	p.log.Infof("Block %s is valid for height %d ✅ ", blockHash, blockHeight)
	return &lib.BlockResult{
		BlockHeader:  b.BlockHeader,
		Transactions: txResults,
	}, nil
}

// CompareBlockHeaders() compares two block headers for equality and validates the last quorum certificate and block time
func (p *Plugin) CompareBlockHeaders(candidate *lib.BlockHeader, compare *lib.BlockHeader) lib.ErrorI {
	// compare the block headers for equality
	hash, e := compare.SetHash()
	if e != nil {
		return e
	}
	if !bytes.Equal(hash, candidate.Hash) {
		return lib.ErrUnequalBlockHash()
	}
	// validate the last quorum certificate of the block header
	// by using the historical canopy committee
	if candidate.Height > 2 {
		lastBlockHeight := candidate.Height - 1
		vs, err := p.FSM.LoadCanopyCommittee(lastBlockHeight)
		if err != nil {
			return err
		}
		// check the last QC
		isPartialQC, err := candidate.LastQuorumCertificate.Check(vs, 0, &lib.View{
			Height:       lastBlockHeight,
			CanopyHeight: lastBlockHeight,
			NetworkId:    p.Config.NetworkID,
			CommitteeId:  p.CommitteeId,
		}, true)
		if err != nil {
			return err
		}
		// ensure is a full +2/3rd maj QC
		if isPartialQC {
			return lib.ErrNoMaj23()
		}
	}
	// validate the timestamp in the block header
	if err := p.validateBlockTime(candidate); err != nil {
		return err
	}
	return nil
}

// validateBlockTime() validates the timestamp in the block header for safe pruning
// accepts +/- hour variance for clock drift
func (p *Plugin) validateBlockTime(header *lib.BlockHeader) lib.ErrorI {
	now := time.Now()
	maxTime, minTime, t := now.Add(time.Hour), now.Add(-time.Hour), time.UnixMicro(int64(header.Time))
	if minTime.Compare(t) > 0 || maxTime.Compare(t) < 0 {
		return lib.ErrInvalidBlockTime()
	}
	return nil
}

// IntegratedChain() returns if the plugin is for an 'integrated' or 'external' chain
func (p *Plugin) IntegratedChain() bool { return true }

// HandleTx() routes an inbound transaction from the Controller
func (p *Plugin) HandleTx(tx []byte) lib.ErrorI {
	hash := crypto.Hash(tx)
	hashString := lib.BytesToString(hash)
	// indexer
	txResult, err := p.FSM.Store().(lib.StoreI).GetTxByHash(hash)
	if err != nil {
		return err
	}
	if txResult.TxHash != "" {
		return lib.ErrDuplicateTx(hashString)
	}
	// mempool
	if p.Mempool.Contains(hashString) {
		return lib.ErrTxFoundInMempool(hashString)
	}
	return p.Mempool.HandleTransaction(tx)
}

// CommitCertificate used after consensus decides on a block
// - applies block against the fsm
// - indexes the block and its transactions
// - removes block transactions from mempool
// - re-checks all transactions in mempool
// - atomically writes all to the underlying db
// - sets up the app for the next height
func (p *Plugin) CommitCertificate(qc *lib.QuorumCertificate) lib.ErrorI {
	// reset the store once this code finishes
	// NOTE: if code execution gets to `store.Commit()` - this will effectively be a noop
	defer func() { p.FSM.Reset() }()
	p.log.Debugf("TryCommit block %s", lib.BytesToString(qc.ResultsHash))
	// load the store (db) object
	store := p.FSM.Store().(lib.StoreI)
	// convert the proposal.block (bytes) into Block structure
	blk := new(lib.Block)
	if err := lib.Unmarshal(qc.Block, blk); err != nil {
		return err
	}
	// apply the block against the state machine
	blockResult, err := p.ApplyAndValidateBlock(blk)
	if err != nil {
		return err
	}
	// index the quorum certificate in the store
	p.log.Debugf("Indexing quorum certificate for height %d", qc.Header.Height)
	if err = store.IndexQC(qc); err != nil {
		return err
	}
	// update the LastQuorumCertificate in the store
	//- this is to ensure that each node has the same COMMIT QC for the last block as multiple valid versions (a few less signatures) of the same QC could exist
	if blk.BlockHeader.Height != 1 {
		p.log.Debugf("Indexing last quorum certificate for height %d", blk.BlockHeader.LastQuorumCertificate.Header.Height)
		if err = store.IndexQC(blk.BlockHeader.LastQuorumCertificate); err != nil {
			return err
		}
	}
	// index the block in the store
	p.log.Debugf("Indexing block %d", blk.BlockHeader.Height)
	if err = store.IndexBlock(blockResult); err != nil {
		return err
	}
	// delete the block transactions in the mempool
	for _, tx := range blk.Transactions {
		p.log.Debugf("tx %s was included in a block so removing from mempool", crypto.HashString(tx))
		p.Mempool.DeleteTransaction(tx)
	}
	// rescan mempool to ensure validity of all transactions
	p.log.Debug("Checking mempool for newly invalid transactions")
	p.Mempool.checkMempool()
	// atomically write this to the store
	p.log.Debug("Committing to store")
	if _, err = store.Commit(); err != nil {
		return err
	}
	p.log.Infof("Committed block %s at H:%d 🔒", lib.BytesToString(qc.ResultsHash), blk.BlockHeader.Height)
	p.log.Debug("Setting up FSM for next height")
	p.FSM, err = fsm.New(p.Config, store, p.log)
	if err != nil {
		return err
	}
	p.log.Debug("Setting up Mempool for next height")
	if p.Mempool.FSM, err = p.FSM.Copy(); err != nil {
		return err
	}
	if err = p.Mempool.FSM.BeginBlock(); err != nil {
		return err
	}
	p.log.Debug("Commit done")
	return nil
}

// LoadCertificate() loads the quorum certificate for a specific height
func (p *Plugin) LoadCertificate(height uint64) (*lib.QuorumCertificate, lib.ErrorI) {
	return p.FSM.LoadCertificateHashesOnly(height)
}

// GetFSM() is a Canopy specific plugin call - returns the state machine object for Canopy
func (p *Plugin) GetFSM() *fsm.StateMachine { return p.FSM }

// ResetAndStartBFTTimer() allows the Controller to signal a reset and start of the bft trigger timer
func (p *Plugin) ResetAndStartBFTTimer() { p.resetBFTTrigger <- struct{}{} }

// Height() returns the height of the Canopy software
func (p *Plugin) Height() uint64 { return p.FSM.Height() }

// TotalVDFIterations() returns the total number of VDF iterations in the chain
func (p *Plugin) TotalVDFIterations() uint64 { return p.FSM.TotalVDFIterations() }

// LoadLastCommitTime() returns the last time the Canopy software committed
func (p *Plugin) LoadLastCommitTime(height uint64) time.Time {
	cert, err := p.LoadCertificate(height)
	if err != nil {
		p.log.Error(err.Error())
		return time.Time{}
	}
	block := new(lib.Block)
	if err = lib.Unmarshal(cert.Block, block); err != nil {
		p.log.Error(err.Error())
		return time.Time{}
	}
	if block.BlockHeader == nil {
		return time.Time{}
	}
	return time.UnixMicro(int64(block.BlockHeader.Time))
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
	m.log.Infof("added tx %s to mempool for checking", crypto.HashString(tx))
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

// PendingPageForRPC() returns a page of Mempool transactions and their 'ephemeral' results for the RPC
func (p *Plugin) PendingPageForRPC(params lib.PageParams) (page *lib.Page, err lib.ErrorI) {
	page, txResults := lib.NewPage(params, lib.PendingResultsPageName), make(lib.TxResults, 0)
	err = page.LoadArray(p.Mempool.cachedResults, &txResults, func(i any) lib.ErrorI {
		v, ok := i.(*lib.TxResult)
		if !ok {
			return lib.ErrInvalidArgument()
		}
		txResults = append(txResults, v)
		return nil
	})
	return
}
