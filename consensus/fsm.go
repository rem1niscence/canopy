package consensus

import (
	"github.com/ginchuco/ginchu/fsm"
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"math"
)

// HandleTransaction accepts or rejects inbound txs based on the mempool state
// - pass through call checking indexer and mempool for duplicate
func (c *Consensus) HandleTransaction(tx []byte) lib.ErrorI {
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

// CheckCandidateBlock checks the candidate block for errors and resets back to begin block state
func (c *Consensus) CheckCandidateBlock(candidate *lib.Block, evidence *lib.ByzantineEvidence) (err lib.ErrorI) {
	reset := c.ValidatorProposalConfig(c.FSM)
	defer func() { c.FSM.Reset(); reset() }()
	_, _, err = c.FSM.ApplyAndValidateBlock(candidate, evidence, true)
	return
}

// ProduceCandidateBlock uses the mempool and state params to build a candidate block
func (c *Consensus) ProduceCandidateBlock(badProposers [][]byte, dse lib.DoubleSignEvidences) (*lib.Block, lib.ErrorI) {
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
	header := &lib.BlockHeader{
		Height:                height + 1,
		NetworkId:             c.FSM.NetworkID,
		Time:                  timestamppb.Now(),
		NumTxs:                uint64(numTxs),
		TotalTxs:              lastBlock.TotalTxs + uint64(numTxs),
		LastBlockHash:         lastBlock.Hash,
		ProposerAddress:       pubKey.Address().Bytes(),
		Evidence:              dse.DSE,
		BadProposers:          badProposers,
		LastQuorumCertificate: qc,
	}
	block := &lib.Block{
		BlockHeader:  header,
		Transactions: transactions,
	}
	block.BlockHeader, _, _, err = c.FSM.ApplyBlock(block)
	return block, err
}

// CommitBlock used after consensus decides on a block
// - applies block against the fsm
// - indexes the block and its transactions
// - removes block transactions from mempool
// - re-checks all transactions in mempool
// - atomically writes all to the underlying db
// - sets up the app for the next height
func (c *Consensus) CommitBlock(qc *lib.QuorumCertificate) lib.ErrorI {
	c.log.Debugf("TryCommit block %s", lib.BytesToString(qc.BlockHash))
	store, blk := c.FSM.Store().(lib.StoreI), qc.Block
	blockResult, nextVals, err := c.FSM.ApplyAndValidateBlock(blk, nil, false)
	if err != nil {
		return err
	}
	c.log.Debugf("Indexing quorum certificate for height %d", qc.Header.Height)
	if err = store.IndexQC(qc); err != nil {
		return err
	}
	if blk.BlockHeader.Height != 1 {
		c.log.Debugf("Indexing last quorum certificate for height %d", blk.BlockHeader.LastQuorumCertificate.Header.Height)
		if err = store.IndexQC(qc.Block.BlockHeader.LastQuorumCertificate); err != nil {
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
	c.log.Infof("Committed block %s at H:%d 🔒", lib.BytesToString(qc.BlockHash), blk.BlockHeader.Height)
	c.LastValidatorSet, c.Height = c.ValidatorSet, blk.BlockHeader.Height+1
	if c.ValidatorSet, err = lib.NewValidatorSet(nextVals); err != nil {
		return err
	}
	c.log.Debug("Setting up FSM for next height")
	c.FSM = fsm.NewWithBeginBlock(&lib.BeginBlockParams{BlockHeader: blk.BlockHeader, ValidatorSet: nextVals},
		c.Config, c.Height, store, c.log)
	c.log.Debug("Setting up Mempool for next height")
	if c.Mempool.FSM, err = c.FSM.Copy(); err != nil {
		return err
	}
	if err = c.Mempool.FSM.BeginBlock(); err != nil {
		return err
	}
	return nil
}

func (c *Consensus) ValidatorProposalConfig(fsm ...*fsm.StateMachine) (reset func()) {
	for _, f := range fsm {
		if c.Round < 3 {
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

func (c *Consensus) GetPendingPage(p lib.PageParams) (page *lib.Page, err lib.ErrorI) {
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
