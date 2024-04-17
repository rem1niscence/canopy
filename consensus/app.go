package consensus

import (
	"bytes"
	"github.com/ginchuco/ginchu/state_machine"
	"github.com/ginchuco/ginchu/state_machine/types"
	lib "github.com/ginchuco/ginchu/types"
	"github.com/ginchuco/ginchu/types/crypto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"time"
)

type State struct {
	store  lib.StoreI
	params *lib.BeginBlockParams
	chain  *ChainParams
	fsm    *state_machine.StateMachine
	log    lib.LoggerI
}

// HandleTransaction accepts or rejects inbound txs based on the mempool state
// - pass through call checking indexer and mempool for duplicate
func (cs *ConsensusState) HandleTransaction(tx []byte) lib.ErrorI {
	hash := crypto.Hash(tx)
	if err := cs.checkForDuplicateTx(hash); err != nil {
		return err
	}
	return cs.Mempool.HandleTransaction(tx)
}

// CheckCandidateBlock checks the candidate block for errors and resets back to begin block state
func (cs *ConsensusState) CheckCandidateBlock(candidate *lib.Block, evidence *lib.ByzantineEvidence) (err lib.ErrorI) {
	defer cs.State.resetToBeginBlock()
	_, _, err = cs.applyAndValidateBlock(candidate, evidence, true)
	return
}

// ProduceCandidateBlock uses the mempool and state params to build a candidate block
func (cs *ConsensusState) ProduceCandidateBlock(badProposers, doubleSigners [][]byte) (*lib.Block, lib.ErrorI) {
	defer cs.State.resetToBeginBlock()
	qc, err := cs.GetBlockAndCertificate(cs.State.height())
	if err != nil {
		return nil, err
	}
	qc.Block = nil
	numTxs, transactions := cs.Mempool.GetTransactions(cs.State.chain.MaxBlockBytes)
	header := &lib.BlockHeader{
		Height:                cs.State.height() + 1,
		NetworkId:             cs.State.chain.NetworkID,
		Time:                  timestamppb.Now(),
		NumTxs:                uint64(numTxs),
		TotalTxs:              cs.State.params.BlockHeader.TotalTxs + uint64(numTxs),
		LastBlockHash:         cs.State.params.BlockHeader.Hash,
		ProposerAddress:       cs.State.chain.SelfAddress.Bytes(),
		LastDoubleSigners:     doubleSigners,
		BadProposers:          badProposers,
		LastQuorumCertificate: qc,
	}
	block := &lib.Block{
		BlockHeader:  header,
		Transactions: transactions,
	}
	block.BlockHeader, _, _, err = cs.applyBlock(block)
	return block, err
}

// CommitBlock used after consensus decides on a block
// - applies block against the fsm
// - indexes the block and its transactions
// - removes block transactions from mempool
// - re-checks all transactions in mempool
// - atomically writes all to the underlying db
// - sets up the app for the next height
func (cs *ConsensusState) CommitBlock(qc *lib.QuorumCertificate) lib.ErrorI {
	block := qc.Block
	blockResult, nextValidatorSet, err := cs.applyAndValidateBlock(block, nil, false)
	if err != nil {
		return err
	}
	if err = cs.State.store.IndexQC(qc); err != nil {
		return err
	}
	if err = cs.State.store.IndexBlock(blockResult); err != nil {
		return err
	}
	for _, tx := range block.Transactions {
		cs.Mempool.DeleteTransaction(tx)
	}
	if err = cs.Mempool.checkMempool(); err != nil {
		return err
	}
	if _, err = cs.State.store.Commit(); err != nil {
		return err
	}
	cs.State = NewState(cs.State.store, nextValidatorSet, blockResult.BlockHeader, cs.State.chain, cs.log) // next height
	cs.Mempool.State, err = cs.State.copy()
	if err != nil {
		return err
	}
	return nil
}

func (cs *ConsensusState) applyAndValidateBlock(b *lib.Block, evidence *lib.ByzantineEvidence, isCandidateBlock bool) (*lib.BlockResult, *lib.ValidatorSet, lib.ErrorI) {
	header, txResults, valSet, err := cs.applyBlock(b)
	if err != nil {
		return nil, nil, err
	}
	if err = cs.validateBlock(b.BlockHeader, header, evidence, isCandidateBlock); err != nil {
		return nil, nil, err
	}
	return &lib.BlockResult{
		BlockHeader:  b.BlockHeader,
		Transactions: txResults,
	}, valSet, nil
}

func (cs *ConsensusState) applyBlock(b *lib.Block) (*lib.BlockHeader, []*lib.TxResult, *lib.ValidatorSet, lib.ErrorI) {
	if err := cs.State.beginBlock(); err != nil {
		return nil, nil, nil, err
	}
	txResults, txRoot, numTxs, err := cs.applyTransactions(b)
	if err != nil {
		return nil, nil, nil, err
	}
	eb, err := cs.State.endBlock()
	if err != nil {
		return nil, nil, nil, err
	}
	validatorRoot, err := cs.State.params.ValidatorSet.Root()
	if err != nil {
		return nil, nil, nil, err
	}
	nextValidatorRoot, err := eb.ValidatorSet.Root()
	if err != nil {
		return nil, nil, nil, err
	}
	stateRoot, err := cs.State.store.Root()
	if err != nil {
		return nil, nil, nil, err
	}
	header := lib.BlockHeader{
		Height:                cs.State.height() + 1,
		Hash:                  nil,
		NetworkId:             cs.State.chain.NetworkID,
		Time:                  b.BlockHeader.Time,
		NumTxs:                uint64(numTxs),
		TotalTxs:              cs.State.params.BlockHeader.TotalTxs + uint64(numTxs),
		LastBlockHash:         cs.State.params.BlockHeader.Hash,
		StateRoot:             stateRoot,
		TransactionRoot:       txRoot,
		ValidatorRoot:         validatorRoot,
		NextValidatorRoot:     nextValidatorRoot,
		ProposerAddress:       b.BlockHeader.ProposerAddress,
		LastDoubleSigners:     b.BlockHeader.LastDoubleSigners,
		BadProposers:          b.BlockHeader.BadProposers,
		LastQuorumCertificate: b.BlockHeader.LastQuorumCertificate,
	}
	if _, err = header.SetHash(); err != nil {
		return nil, nil, nil, err
	}
	return &header, txResults, eb.ValidatorSet, nil
}

func (cs *ConsensusState) validateBlock(header *lib.BlockHeader, compare *lib.BlockHeader, evidence *lib.ByzantineEvidence, isCandidateBlock bool) lib.ErrorI {
	hash, err := compare.SetHash()
	if err != nil {
		return err
	}
	if !bytes.Equal(hash, header.Hash) {
		return lib.ErrUnequalBlockHash()
	}
	qcValidatorSet, err := cs.GetBeginStateValSet(header.Height - 1)
	if err != nil {
		return err
	}
	vs, err := lib.NewValidatorSet(qcValidatorSet)
	if err != nil {
		return err
	}
	isPartialQC, err := header.LastQuorumCertificate.Check(&lib.View{Height: header.Height - 1}, vs)
	if err != nil {
		return err
	}
	if isPartialQC {
		return lib.ErrNoMaj23()
	}
	if err = cs.validateBlockTime(header); err != nil {
		return err
	}
	if isCandidateBlock {
		if err = header.ValidateByzantineEvidence(cs, evidence); err != nil {
			return err
		}
	}
	return nil
}

func (cs *ConsensusState) applyTransactions(block *lib.Block) (results []*lib.TxResult, root []byte, n int, er lib.ErrorI) {
	var items [][]byte
	for index, tx := range block.Transactions {
		result, err := cs.State.applyTransaction(tx, index)
		if err != nil {
			return nil, nil, 0, err
		}
		bz, err := result.GetBytes()
		if err != nil {
			return nil, nil, 0, err
		}
		results = append(results, result)
		items = append(items, bz)
		n++
	}
	root, _, err := lib.MerkleTree(items)
	return results, root, n, err
}

func (cs *ConsensusState) checkForDuplicateTx(hash []byte) lib.ErrorI {
	// indexer
	txResult, err := cs.State.store.GetTxByHash(hash)
	if err != nil {
		return err
	}
	if txResult != nil {
		return types.ErrDuplicateTx(hash)
	}
	// mempool
	if h := lib.BytesToString(hash); cs.Mempool.Contains(h) {
		return types.ErrTxFoundInMempool(h)
	}
	return nil
}

func (cs *ConsensusState) GetBeginStateValSet(height uint64) (*lib.ValidatorSet, lib.ErrorI) {
	height -= 1 // begin state is the end state of the previous height
	newStore, err := cs.State.store.NewReadOnly(height)
	if err != nil {
		return nil, err
	}
	return state_machine.NewStateMachine(cs.State.chain.ProtocolVersion, height, newStore).GetConsensusValidators()
}

func (cs *ConsensusState) LatestHeight() uint64                       { return cs.State.store.Version() }
func (cs *ConsensusState) GetBeginBlockParams() *lib.BeginBlockParams { return cs.State.params }
func (cs *ConsensusState) GetProducerPubKeys() [][]byte {
	keys, err := cs.State.fsm.GetProducerKeys()
	if err != nil {
		return nil
	}
	return keys.ProducerKeys
}
func (cs *ConsensusState) GetBlockAndCertificate(height uint64) (*lib.QuorumCertificate, lib.ErrorI) {
	return cs.State.store.GetQCByHeight(height)
}

func (cs *ConsensusState) validateBlockTime(header *lib.BlockHeader) lib.ErrorI {
	now := time.Now()
	t := header.Time.AsTime()
	minTime := now.Add(30 * time.Minute)
	maxTime := now.Add(30 * time.Minute)
	if minTime.Compare(t) > 0 || maxTime.Compare(t) < 0 {
		return lib.ErrInvalidBlockTime()
	}
	return nil
}

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

type ChainParams struct {
	SelfAddress     crypto.AddressI
	NetworkID       []byte
	MaxBlockBytes   uint64
	ProtocolVersion int
}

func NewState(store lib.StoreI, vs *lib.ValidatorSet, lastBlock *lib.BlockHeader, chain *ChainParams, log lib.LoggerI) State {
	return State{
		store: store,
		params: &lib.BeginBlockParams{
			BlockHeader:  lastBlock,
			ValidatorSet: vs,
		},
		chain: chain,
		fsm:   state_machine.NewStateMachine(chain.ProtocolVersion, store.Version(), store),
		log:   log,
	}
}

func (s *State) txnWrap() (lib.StoreTxnI, func()) {
	txn := s.store.NewTxn()
	s.fsm.SetStore(txn)
	return txn, func() { s.fsm.SetStore(s.store); txn.Discard() }
}

func (s *State) beginBlock() lib.ErrorI                      { return s.fsm.BeginBlock(s.params) }
func (s *State) endBlock() (*lib.EndBlockParams, lib.ErrorI) { return s.fsm.EndBlock() }
func (s *State) reset()                                      { s.store.Reset() }
func (s *State) height() uint64                              { return s.store.Version() }
func (s *State) applyTransaction(tx []byte, index int) (*lib.TxResult, lib.ErrorI) {
	return s.fsm.ApplyTransaction(uint64(index), tx, crypto.HashString(tx))
}

func (s *State) resetToBeginBlock() {
	s.reset()
	if err := s.beginBlock(); err != nil {
		s.log.Error(err.Error())
	}
}

func (s *State) copy() (*State, lib.ErrorI) {
	storeCopy, err := s.store.Copy()
	if err != nil {
		return nil, err
	}
	return &State{
		store:  storeCopy,
		params: s.params,
		fsm:    state_machine.NewStateMachine(s.chain.ProtocolVersion, storeCopy.Version(), storeCopy),
		log:    s.log,
	}, nil
}
