package consensus

import (
	"github.com/ginchuco/ginchu/fsm"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// HandleTransaction accepts or rejects inbound txs based on the mempool state
// - pass through call checking indexer and mempool for duplicate
func (s *State) HandleTransaction(tx []byte) lib.ErrorI {
	hash := crypto.Hash(tx)
	// indexer
	txResult, err := s.FSM.Store().(lib.StoreI).GetTxByHash(hash)
	if err != nil {
		return err
	}
	if txResult != nil {
		return ErrDuplicateTx(hash)
	}
	// mempool
	if h := lib.BytesToString(hash); s.Mempool.Contains(h) {
		return lib.ErrTxFoundInMempool(h)
	}
	return s.Mempool.HandleTransaction(tx)
}

// CheckCandidateBlock checks the candidate block for errors and resets back to begin block state
func (s *State) CheckCandidateBlock(candidate *lib.Block, evidence *lib.ByzantineEvidence) (err lib.ErrorI) {
	defer s.FSM.ResetToBeginBlock()
	_, _, err = s.FSM.ApplyAndValidateBlock(candidate, evidence, true)
	return
}

// ProduceCandidateBlock uses the mempool and state params to build a candidate block
func (s *State) ProduceCandidateBlock(badProposers, doubleSigners [][]byte) (*lib.Block, lib.ErrorI) {
	defer s.FSM.ResetToBeginBlock()
	height := s.FSM.Height()
	qc, err := s.FSM.GetBlockAndCertificate(height)
	if err != nil {
		return nil, err
	}
	qc.Block = nil
	lastBlock := s.FSM.LastBlockHeader()
	numTxs, transactions := s.Mempool.GetTransactions(s.FSM.MaxBlockBytes)
	header := &lib.BlockHeader{
		Height:                height + 1,
		NetworkId:             s.FSM.NetworkID,
		Time:                  timestamppb.Now(),
		NumTxs:                uint64(numTxs),
		TotalTxs:              lastBlock.TotalTxs + uint64(numTxs),
		LastBlockHash:         lastBlock.Hash,
		ProposerAddress:       s.PublicKey.Bytes(),
		LastDoubleSigners:     doubleSigners,
		BadProposers:          badProposers,
		LastQuorumCertificate: qc,
	}
	block := &lib.Block{
		BlockHeader:  header,
		Transactions: transactions,
	}
	block.BlockHeader, _, _, err = s.FSM.ApplyBlock(block)
	return block, err
}

// CommitBlock used after consensus decides on a block
// - applies block against the fsm
// - indexes the block and its transactions
// - removes block transactions from mempool
// - re-checks all transactions in mempool
// - atomically writes all to the underlying db
// - sets up the app for the next height
func (s *State) CommitBlock(qc *lib.QuorumCertificate) lib.ErrorI {
	block := qc.Block
	blockResult, nextValidatorSet, err := s.FSM.ApplyAndValidateBlock(block, nil, false)
	if err != nil {
		return err
	}
	store := s.FSM.Store().(lib.StoreI)
	if err = store.IndexQC(qc); err != nil {
		return err
	}
	if err = store.IndexBlock(blockResult); err != nil {
		return err
	}
	for _, tx := range block.Transactions {
		s.Mempool.DeleteTransaction(tx)
	}
	if err = s.Mempool.checkMempool(); err != nil {
		return err
	}
	if _, err = store.Commit(); err != nil {
		return err
	}
	beginBlockParams := lib.BeginBlockParams{BlockHeader: block.BlockHeader, ValidatorSet: nextValidatorSet}
	s.FSM = fsm.NewWithBeginBlock(&beginBlockParams, s.FSM.ProtocolVersion, s.FSM.NetworkID, block.BlockHeader.Height+1, store)
	s.Mempool.FSM, err = s.FSM.Copy()
	if err != nil {
		return err
	}
	return nil
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

func NewMempool(fsm *fsm.StateMachine, config lib.MempoolConfig, log lib.LoggerI) *Mempool {
	return &Mempool{
		log:     log,
		FSM:     fsm,
		Mempool: lib.NewMempool(config),
	}
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

func (m *Mempool) applyAndWriteTx(tx []byte) (fee string, err lib.ErrorI) {
	store := m.FSM.Store()
	txn, err := m.FSM.TxnWrap()
	if err != nil {
		return "", err
	}
	defer func() { m.FSM.SetStore(store); txn.Discard() }()
	result, err := m.FSM.ApplyTransaction(uint64(m.Size()), tx, crypto.HashString(tx))
	if err != nil {
		return "", err
	}
	if err = txn.Write(); err != nil {
		return "", err
	}
	return result.Transaction.Fee, nil
}
