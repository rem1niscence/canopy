package consensus

import (
	"github.com/ginchuco/ginchu/fsm"
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"google.golang.org/protobuf/types/known/timestamppb"
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
	if txResult != nil {
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
	defer func() { c.FSM.ResetToBeginBlock(); reset() }()
	_, _, err = c.FSM.ApplyAndValidateBlock(candidate, evidence, true)
	return
}

// ProduceCandidateBlock uses the mempool and state params to build a candidate block
func (c *Consensus) ProduceCandidateBlock(badProposers [][]byte, dse lib.DoubleSignEvidences) (*lib.Block, lib.ErrorI) {
	reset := c.ValidatorProposalConfig(c.FSM, c.Mempool.FSM)
	defer func() { c.FSM.ResetToBeginBlock(); reset() }()
	height, lastBlock := c.FSM.Height(), c.FSM.LastBlockHeader()
	qc, err := c.FSM.LoadCertificate(height)
	if err != nil {
		return nil, err
	}
	maxBlockSize, err := c.FSM.GetMaxBlockSize()
	if err != nil {
		return nil, err
	}
	if err = c.Mempool.checkMempool(); err != nil {
		return nil, err
	}
	numTxs, transactions := c.Mempool.GetTransactions(maxBlockSize)
	header := &lib.BlockHeader{
		Height:                height + 1,
		NetworkId:             c.FSM.NetworkID,
		Time:                  timestamppb.Now(),
		NumTxs:                uint64(numTxs),
		TotalTxs:              lastBlock.TotalTxs + uint64(numTxs),
		LastBlockHash:         lastBlock.Hash,
		ProposerAddress:       c.PublicKey,
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
	defer func() { c.FSM.ResetToBeginBlock() }()
	store, blk := c.FSM.Store().(lib.StoreI), qc.Block
	blockResult, nextVals, err := c.FSM.ApplyAndValidateBlock(blk, nil, false)
	if err != nil {
		return err
	}
	if err = store.IndexQC(qc); err != nil {
		return err
	}
	if err = store.IndexBlock(blockResult); err != nil {
		return err
	}
	for _, tx := range blk.Transactions {
		c.Mempool.DeleteTransaction(tx)
	}
	if err = c.Mempool.checkMempool(); err != nil {
		return err
	}
	if _, err = store.Commit(); err != nil {
		return err
	}
	c.LastValidatorSet, c.Height = c.ValidatorSet, blk.BlockHeader.Height+1
	if c.ValidatorSet, err = lib.NewValidatorSet(nextVals); err != nil {
		return err
	}
	c.FSM = fsm.NewWithBeginBlock(&lib.BeginBlockParams{BlockHeader: blk.BlockHeader, ValidatorSet: nextVals}, c.Config, c.Height, store)
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
	log lib.LoggerI
	FSM *fsm.StateMachine
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
	m.FSM.ResetToBeginBlock()
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

func (m *Mempool) applyAndWriteTx(tx []byte) (fee uint64, err lib.ErrorI) {
	store := m.FSM.Store()
	txn, err := m.FSM.TxnWrap()
	if err != nil {
		return 0, err
	}
	defer func() { m.FSM.SetStore(store); txn.Discard() }()
	result, err := m.FSM.ApplyTransaction(uint64(m.Size()), tx, crypto.HashString(tx))
	if err != nil {
		return 0, err
	}
	if err = txn.Write(); err != nil {
		return 0, err
	}
	return result.Transaction.Fee, nil
}
