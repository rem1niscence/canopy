package app

import (
	"bytes"
	"github.com/ginchuco/ginchu/bft"
	"github.com/ginchuco/ginchu/fsm"
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"math"
	"time"
)

// HandleTransaction accepts or rejects inbound txs based on the mempool state
// - pass through call checking indexer and mempool for duplicate
func (c *Controller) HandleTransaction(tx []byte) lib.ErrorI {
	c.Lock()
	defer c.Unlock()
	hash := crypto.Hash(tx)
	// indexer
	txResult, err := c.FSM.Store().(lib.StoreI).GetTxByHash(hash)
	if err != nil {
		return err
	}
	if txResult.TxHash != "" {
		return ErrDuplicateTx(hash)
	}
	// mempool
	if h := lib.BytesToString(hash); c.Mempool.Contains(h) {
		return lib.ErrTxFoundInMempool(h)
	}
	return c.Mempool.HandleTransaction(tx)
}

// CheckProposal checks the proposal for errors
func (c *Controller) CheckProposal(proposal []byte) (err lib.ErrorI) {
	block, err := c.proposalToBlock(proposal)
	if err != nil {
		return
	}
	return block.Check()
}

// ValidateProposal fully validates the proposal and resets back to begin block state
func (c *Controller) ValidateProposal(proposal []byte, evidence *bft.ByzantineEvidence) (err lib.ErrorI) {
	reset := c.ValidatorProposalConfig(c.FSM)
	defer func() { c.FSM.Reset(); reset() }()
	block, err := c.proposalToBlock(proposal)
	if err != nil {
		return
	}
	_, _, err = c.ApplyAndValidateBlock(block, evidence, true)
	return
}

func (c *Controller) ApplyAndValidateBlock(b *lib.Block, evidence *bft.ByzantineEvidence, isCandidateBlock bool) (*lib.BlockResult, *lib.ConsensusValidators, lib.ErrorI) {
	if err := b.Check(); err != nil {
		return nil, nil, err
	}
	blockHash, blockHeight := lib.BytesToString(b.BlockHeader.Hash), b.BlockHeader.Height
	c.log.Debugf("Applying block %s for height %d", blockHash, blockHeight)
	header, txResults, valSet, err := c.FSM.ApplyBlock(b)
	if err != nil {
		return nil, nil, err
	}
	c.log.Debugf("Validating block header %s for height %d", blockHash, blockHeight)
	if err = c.CheckBlockHeader(b.BlockHeader, header, evidence, isCandidateBlock); err != nil {
		return nil, nil, err
	}
	c.log.Infof("Block %s is valid for height %d ✅ ", blockHash, blockHeight)
	return &lib.BlockResult{
		BlockHeader:  b.BlockHeader,
		Transactions: txResults,
	}, valSet, nil
}

func (c *Controller) CheckBlockHeader(header *lib.BlockHeader, compare *lib.BlockHeader, evidence *bft.ByzantineEvidence, isCandidateBlock bool) lib.ErrorI {
	hash, e := compare.SetHash()
	if e != nil {
		return e
	}
	if !bytes.Equal(hash, header.Hash) {
		return lib.ErrUnequalBlockHash()
	}
	if header.Height > 2 {
		lastQCHeight := header.Height - 1
		vs, err := c.LoadValSet(lastQCHeight)
		if err != nil {
			return err
		}
		isPartialQC, err := header.LastQuorumCertificate.Check(vs, lastQCHeight)
		if err != nil {
			return err
		}
		if isPartialQC {
			return lib.ErrNoMaj23()
		}
	}
	if err := c.validateBlockTime(header); err != nil {
		return err
	}
	if isCandidateBlock {
		if err := c.Consensus.ValidateByzantineEvidence(header, evidence); err != nil {
			return err
		}
	}
	return nil
}

func (c *Controller) validateBlockTime(header *lib.BlockHeader) lib.ErrorI {
	now := time.Now()
	maxTime, minTime, t := now.Add(time.Hour), now.Add(-time.Hour), header.Time.AsTime()
	if minTime.Compare(t) > 0 || maxTime.Compare(t) < 0 {
		return lib.ErrInvalidBlockTime()
	}
	return nil
}

func (c *Controller) HashProposal(proposal []byte) (hash []byte) {
	block, err := c.proposalToBlock(proposal)
	if err != nil {
		c.log.Error(err.Error())
		return
	}
	if block.BlockHeader == nil {
		return
	}
	hash, err = block.BlockHeader.SetHash()
	if err != nil {
		c.log.Error(err.Error())
		return
	}
	return
}

// ProduceProposal uses the mempool and state params to build a proposal block
func (c *Controller) ProduceProposal(be *bft.ByzantineEvidence) ([]byte, lib.ErrorI) {
	reset := c.ValidatorProposalConfig(c.FSM, c.Mempool.FSM)
	defer func() { c.FSM.Reset(); reset() }()
	height, lastBlock := c.FSM.Height(), c.FSM.LastBlockHeader()
	qc, err := c.FSM.LoadCertificate(height - 1)
	if err != nil {
		return nil, err
	}
	maxBlockSize, err := c.FSM.GetMaxBlockSize()
	if err != nil {
		return nil, err
	}
	c.Mempool.checkMempool()
	pubKey, _ := crypto.NewBLSPublicKeyFromBytes(c.PublicKey)
	numTxs, transactions := c.Mempool.GetTransactions(maxBlockSize)
	doubleSigners, _ := c.Consensus.ProcessDSE(be.DSE.Evidence...)
	badProposers, _ := c.Consensus.ProcessBPE(be.BPE.Evidence...)
	header := &lib.BlockHeader{
		Height:                height + 1,
		NetworkId:             c.FSM.NetworkID,
		Time:                  timestamppb.Now(),
		NumTxs:                uint64(numTxs),
		TotalTxs:              lastBlock.TotalTxs + uint64(numTxs),
		LastBlockHash:         lastBlock.Hash,
		ProposerAddress:       pubKey.Address().Bytes(),
		DoubleSigners:         doubleSigners,
		BadProposers:          badProposers,
		LastQuorumCertificate: qc,
	}
	block := &lib.Block{
		BlockHeader:  header,
		Transactions: transactions,
	}
	block.BlockHeader, _, _, err = c.FSM.ApplyBlock(block)
	return lib.Marshal(block)
}

// commitBlock used after consensus decides on a block
// - applies block against the fsm
// - indexes the block and its transactions
// - removes block transactions from mempool
// - re-checks all transactions in mempool
// - atomically writes all to the underlying db
// - sets up the app for the next height
func (c *Controller) commitBlock(qc *lib.QuorumCertificate) lib.ErrorI {
	c.log.Debugf("TryCommit block %s", lib.BytesToString(qc.ProposalHash))
	store := c.FSM.Store().(lib.StoreI)
	blk, err := c.proposalToBlock(qc.Proposal)
	if err != nil {
		return err
	}
	blockResult, nextVals, err := c.ApplyAndValidateBlock(blk, nil, false)
	if err != nil {
		return err
	}
	c.log.Debugf("Indexing quorum certificate for height %d", qc.Header.Height)
	if err = store.IndexQC(qc); err != nil {
		return err
	}
	if blk.BlockHeader.Height != 1 {
		c.log.Debugf("Indexing last quorum certificate for height %d", blk.BlockHeader.LastQuorumCertificate.Header.Height)
		if err = store.IndexQC(blk.BlockHeader.LastQuorumCertificate); err != nil {
			return err
		}
	}
	c.log.Debugf("Indexing block %d", blk.BlockHeader.Height)
	if err = store.IndexBlock(blockResult); err != nil {
		return err
	}
	for _, tx := range blk.Transactions {
		c.log.Debugf("tx %s was included in a block so removing from mempool", crypto.HashString(tx))
		c.Mempool.DeleteTransaction(tx)
	}
	c.log.Debug("Checking mempool for newly invalid transactions")
	c.Mempool.checkMempool()
	c.log.Debug("Committing to store")
	if _, err = store.Commit(); err != nil {
		return err
	}
	c.log.Infof("Committed block %s at H:%d 🔒", lib.BytesToString(qc.ProposalHash), blk.BlockHeader.Height)
	c.Consensus.LastValidatorSet, c.Consensus.Height = c.Consensus.ValidatorSet, blk.BlockHeader.Height+1
	c.Consensus.ValidatorSet, err = lib.NewValidatorSet(nextVals)
	if err != nil {
		return err
	}
	c.log.Debug("Setting up FSM for next height")
	c.FSM = fsm.NewWithBeginBlock(&lib.BeginBlockParams{BlockHeader: blk.BlockHeader, ValidatorSet: nextVals},
		c.Config, c.Consensus.Height, store, c.log)
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

func (c *Controller) ValidatorProposalConfig(fsm ...*fsm.StateMachine) (reset func()) {
	for _, f := range fsm {
		if c.Consensus.GetRound() < 3 {
			f.SetProposalVoteConfig(types.ProposalVoteConfig_APPROVE_LIST)
		} else {
			f.SetProposalVoteConfig(types.ProposalVoteConfig_REJECT_ALL)
		}
	}
	reset = func() {
		for _, f := range fsm {
			f.SetProposalVoteConfig(types.AcceptAllProposals)
		}
	}
	return
}

// Mempool accepts or rejects incoming txs based on the mempool state
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

func NewMempool(fsm *fsm.StateMachine, config lib.MempoolConfig, log lib.LoggerI) (m *Mempool, err lib.ErrorI) {
	m = &Mempool{
		log:     log,
		Mempool: lib.NewMempool(config),
	}
	m.FSM, err = fsm.Copy()
	if err != nil {
		return nil, err
	}
	return m, err
}

func (m *Mempool) HandleTransaction(tx []byte) lib.ErrorI {
	result, err := m.applyAndWriteTx(tx)
	if err != nil {
		return err
	}
	recheck, err := m.AddTransaction(tx, result.Transaction.Fee)
	if err != nil {
		return err
	}
	m.log.Infof("added tx %s to mempool for checking", crypto.HashString(tx))
	m.cachedResults = append(m.cachedResults, result)
	if recheck {
		m.checkMempool()
	}
	return nil
}

func (m *Mempool) checkMempool() {
	m.FSM.ResetToBeginBlock()
	m.cachedResults = nil
	var remove [][]byte
	it := m.Iterator()
	defer it.Close()
	for ; it.Valid(); it.Next() {
		tx := it.Key()
		result, err := m.applyAndWriteTx(tx)
		if err != nil {
			m.log.Error(err.Error())
			remove = append(remove, tx)
			continue
		}
		m.cachedResults = append(m.cachedResults, result)
	}
	for _, tx := range remove {
		m.log.Infof("removed tx %s from mempool", crypto.HashString(tx))
		m.DeleteTransaction(tx)
	}
}

func (m *Mempool) applyAndWriteTx(tx []byte) (result *lib.TxResult, err lib.ErrorI) {
	store := m.FSM.Store()
	txn, err := m.FSM.TxnWrap()
	if err != nil {
		return nil, err
	}
	defer func() { m.FSM.SetStore(store); txn.Discard() }()
	result, err = m.FSM.ApplyTransaction(uint64(m.Size()), tx, crypto.HashString(tx))
	if err != nil {
		return nil, err
	}
	if err = txn.Write(); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Controller) GetPendingPage(p lib.PageParams) (page *lib.Page, err lib.ErrorI) {
	c.Lock()
	defer c.Unlock()
	return c.Mempool.pendingPageForRPC(p)
}

func (m *Mempool) pendingPageForRPC(p lib.PageParams) (page *lib.Page, err lib.ErrorI) {
	skipIdx, page := p.SkipToIndex(), lib.NewPage(p)
	page.Type = lib.PendingResultsPageName
	txResults := make(lib.TxResults, 0)
	for countOnly, i := false, 0; i < len(m.cachedResults); i++ {
		page.TotalCount++
		switch {
		case i < skipIdx || countOnly:
			continue
		case i == skipIdx+page.PerPage:
			countOnly = true
			continue
		}
		txResults = append(txResults, m.cachedResults[i])
		page.Results = &txResults
		page.Count++
	}
	page.TotalPages = int(math.Ceil(float64(page.TotalCount) / float64(page.PerPage)))
	return
}
