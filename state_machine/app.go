package state_machine

import (
	"bytes"
	"github.com/ginchuco/ginchu/crypto"
	"github.com/ginchuco/ginchu/state_machine/fsm"
	"github.com/ginchuco/ginchu/state_machine/types"
	lib "github.com/ginchuco/ginchu/types"
	"google.golang.org/protobuf/types/known/timestamppb"
	"time"
)

var _ lib.App = &App{}

type App struct {
	State
	mempool Mempool
}

// HandleTransaction accepts or rejects inbound txs based on the mempool state
// - pass through call checking indexer and mempool for duplicate
func (a *App) HandleTransaction(tx []byte) lib.ErrorI {
	hash := crypto.Hash(tx)
	if err := a.checkForDuplicateTx(hash); err != nil {
		return err
	}
	return a.mempool.HandleTransaction(tx)
}

// CheckCandidateBlock checks the candidate block for errors and resets back to begin block state
func (a *App) CheckCandidateBlock(candidate *lib.Block, evidence *lib.ByzantineEvidence) (err lib.ErrorI) {
	defer a.resetToBeginBlock()
	_, _, err = a.applyAndValidateBlock(candidate, evidence, true)
	return
}

// ProduceCandidateBlock uses the mempool and state params to build a candidate block
func (a *App) ProduceCandidateBlock(badProposers, doubleSigners [][]byte) (*lib.Block, lib.ErrorI) {
	defer a.resetToBeginBlock()
	qc, err := a.GetBlockAndCertificate(a.height())
	if err != nil {
		return nil, err
	}
	qc.Block = nil
	numTxs, transactions := a.mempool.GetTransactions(a.chain.MaxBlockBytes)
	header := &lib.BlockHeader{
		Height:                a.height() + 1,
		NetworkId:             a.chain.NetworkID,
		Time:                  timestamppb.Now(),
		NumTxs:                uint64(numTxs),
		TotalTxs:              a.params.BlockHeader.TotalTxs + uint64(numTxs),
		LastBlockHash:         a.params.BlockHeader.Hash,
		ProposerAddress:       a.chain.SelfAddress.Bytes(),
		LastDoubleSigners:     doubleSigners,
		BadProposers:          badProposers,
		LastQuorumCertificate: qc,
	}
	block := &lib.Block{
		BlockHeader:  header,
		Transactions: transactions,
	}
	block.BlockHeader, _, _, err = a.applyBlock(block)
	return block, err
}

// CommitBlock used after consensus decides on a block
// - applies block against the fsm
// - indexes the block and its transactions
// - removes block transactions from mempool
// - re-checks all transactions in mempool
// - atomically writes all to the underlying db
// - sets up the app for the next height
func (a *App) CommitBlock(qc *lib.QuorumCertificate) lib.ErrorI {
	block := qc.Block
	blockResult, nextValidatorSet, err := a.applyAndValidateBlock(block, nil, false)
	if err != nil {
		return err
	}
	if err = a.store.IndexQC(qc); err != nil {
		return err
	}
	if err = a.store.IndexBlock(blockResult); err != nil {
		return err
	}
	for _, tx := range block.Transactions {
		a.mempool.DeleteTransaction(tx)
	}
	if err = a.mempool.checkMempool(); err != nil {
		return err
	}
	if _, err = a.store.Commit(); err != nil {
		return err
	}
	a.State = NewState(a.store, nextValidatorSet, blockResult.BlockHeader, a.chain, a.log) // next height
	a.mempool.State, err = a.State.copy()
	if err != nil {
		return err
	}
	return nil
}

func (a *App) applyAndValidateBlock(b *lib.Block, evidence *lib.ByzantineEvidence, isCandidateBlock bool) (*lib.BlockResult, *lib.ValidatorSet, lib.ErrorI) {
	header, txResults, valSet, err := a.applyBlock(b)
	if err != nil {
		return nil, nil, err
	}
	if err = a.validateBlock(b.BlockHeader, header, evidence, isCandidateBlock); err != nil {
		return nil, nil, err
	}
	return &lib.BlockResult{
		BlockHeader:  b.BlockHeader,
		Transactions: txResults,
	}, valSet, nil
}

func (a *App) applyBlock(b *lib.Block) (*lib.BlockHeader, []*lib.TxResult, *lib.ValidatorSet, lib.ErrorI) {
	if err := a.beginBlock(); err != nil {
		return nil, nil, nil, err
	}
	txResults, txRoot, numTxs, err := a.applyTransactions(b)
	if err != nil {
		return nil, nil, nil, err
	}
	eb, err := a.endBlock()
	if err != nil {
		return nil, nil, nil, err
	}
	validatorRoot, err := a.params.ValidatorSet.Root()
	if err != nil {
		return nil, nil, nil, err
	}
	nextValidatorRoot, err := eb.ValidatorSet.Root()
	if err != nil {
		return nil, nil, nil, err
	}
	stateRoot, err := a.store.Root()
	if err != nil {
		return nil, nil, nil, err
	}
	header := lib.BlockHeader{
		Height:                a.height() + 1,
		Hash:                  nil,
		NetworkId:             a.chain.NetworkID,
		Time:                  b.BlockHeader.Time,
		NumTxs:                uint64(numTxs),
		TotalTxs:              a.params.BlockHeader.TotalTxs + uint64(numTxs),
		LastBlockHash:         a.params.BlockHeader.Hash,
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

func (a *App) validateBlock(header *lib.BlockHeader, compare *lib.BlockHeader, evidence *lib.ByzantineEvidence, isCandidateBlock bool) lib.ErrorI {
	hash, err := compare.SetHash()
	if err != nil {
		return err
	}
	if !bytes.Equal(hash, header.Hash) {
		return lib.ErrUnequalBlockHash()
	}
	qcValidatorSet, err := a.GetBeginStateValSet(header.Height - 1)
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
	if err = a.validateBlockTime(header); err != nil {
		return err
	}
	if isCandidateBlock {
		if err = header.ValidateByzantineEvidence(a, evidence); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) applyTransactions(block *lib.Block) (results []*lib.TxResult, root []byte, n int, er lib.ErrorI) {
	var items [][]byte
	for index, tx := range block.Transactions {
		result, err := a.applyTransaction(tx, index)
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

func (a *App) checkForDuplicateTx(hash []byte) lib.ErrorI {
	// indexer
	txResult, err := a.store.GetTxByHash(hash)
	if err != nil {
		return err
	}
	if txResult != nil {
		return types.ErrDuplicateTx(hash)
	}
	// mempool
	if h := lib.BytesToString(hash); a.mempool.Contains(h) {
		return types.ErrTxFoundInMempool(h)
	}
	return nil
}

func (a *App) GetBeginStateValSet(height uint64) (*lib.ValidatorSet, lib.ErrorI) {
	height -= 1 // begin state is the end state of the previous height
	newStore, err := a.store.NewReadOnly(height)
	if err != nil {
		return nil, err
	}
	return fsm.NewStateMachine(a.chain.ProtocolVersion, height, newStore).GetConsensusValidators()
}

func (a *App) EvidenceExists(e *lib.DoubleSignEvidence) (bool, lib.ErrorI) {
	bz, err := lib.Marshal(e)
	if err != nil {
		return false, err
	}
	evidence, err := a.store.GetEvidenceByHash(crypto.Hash(bz))
	if err != nil {
		return false, err
	}
	return evidence != nil, nil
}

func (a *App) LatestHeight() uint64                       { return a.store.Version() }
func (a *App) GetBeginBlockParams() *lib.BeginBlockParams { return a.params }
func (a *App) GetProducerPubKeys() [][]byte {
	keys, err := a.fsm.GetProducerKeys()
	if err != nil {
		return nil
	}
	return keys.ProducerKeys
}
func (a *App) GetBlockAndCertificate(height uint64) (*lib.QuorumCertificate, lib.ErrorI) {
	return a.store.GetQCByHeight(height)
}

func (a *App) validateBlockTime(header *lib.BlockHeader) lib.ErrorI {
	now := time.Now()
	t := header.Time.AsTime()
	minTime := now.Add(30 * time.Minute)
	maxTime := now.Add(30 * time.Minute)
	if minTime.Compare(t) > 0 || maxTime.Compare(t) < 0 {
		return lib.ErrInvalidBlockTime()
	}
	return nil
}
