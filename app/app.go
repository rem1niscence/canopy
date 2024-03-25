package app

import (
	"bytes"
	"github.com/ginchuco/ginchu/crypto"
	"github.com/ginchuco/ginchu/state_machine/types"
	lib "github.com/ginchuco/ginchu/types"
	"google.golang.org/protobuf/types/known/timestamppb"
)

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
func (a *App) CheckCandidateBlock(candidate *lib.Block) (err lib.ErrorI) {
	defer a.resetToBeginBlock()
	_, err = a.applyBlock(candidate)
	return
}

// ProduceCandidateBlock uses the mempool and state params to build a candidate block
func (a *App) ProduceCandidateBlock() (*lib.Block, lib.ErrorI) {
	numTxs, transactions := a.mempool.GetTransactions(a.chain.MaxBlockBytes)
	header := &lib.BlockHeader{
		Height:          a.height() + 1,
		NetworkId:       a.chain.NetworkID,
		Time:            timestamppb.Now(),
		NumTxs:          uint64(numTxs),
		TotalTxs:        a.lastBlock.TotalTxs + uint64(numTxs),
		LastBlockHash:   a.lastBlock.Hash,
		ProposerAddress: a.chain.SelfAddress.Bytes(),
	}
	return &lib.Block{
		BlockHeader:  header,
		Transactions: transactions,
	}, nil
}

// CommitBlock used after consensus decides on a block
// - applies block against the fsm
// - indexes the block and its transactions
// - removes block transactions from mempool
// - re-checks all transactions in mempool
// - atomically writes all to the underlying db
// - sets up the app for the next height
func (a *App) CommitBlock(block *lib.Block, params *lib.BeginBlockParams) lib.ErrorI {
	blockResult, err := a.applyBlock(block)
	if err != nil {
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
	a.State = NewState(a.store, params, block.BlockHeader, a.chain, a.log)
	a.mempool.State, err = a.State.copy()
	if err != nil {
		return err
	}
	return nil
}

func (a *App) applyBlock(b *lib.Block) (*lib.BlockResult, lib.ErrorI) {
	if err := a.beginBlock(); err != nil {
		return nil, err
	}
	txResults, txRoot, numTxs, err := a.applyTransactions(b)
	if err != nil {
		return nil, err
	}
	eb, err := a.endBlock()
	if err != nil {
		return nil, err
	}
	if err = a.validateBlock(b.BlockHeader, txRoot, uint64(numTxs), eb.ValidatorSet); err != nil {
		return nil, err
	}
	return &lib.BlockResult{
		BlockHeader:  b.BlockHeader,
		Transactions: txResults,
	}, nil
}

func (a *App) validateBlock(header *lib.BlockHeader, txRoot []byte, numTxs uint64, nvs *lib.ValidatorSet) lib.ErrorI {
	validatorRoot, err := a.params.ValidatorSet.Root()
	if err != nil {
		return err
	}
	nextValidatorRoot, err := nvs.Root()
	if err != nil {
		return err
	}
	stateRoot, err := a.store.Root()
	if err != nil {
		return err
	}
	// TODO validate block time, quorum certificate, and proposer address signature
	compare := &lib.BlockHeader{
		Height:            a.height(),
		NetworkId:         a.chain.NetworkID,
		Time:              header.Time,
		NumTxs:            numTxs,
		TotalTxs:          a.lastBlock.TotalTxs + numTxs,
		LastBlockHash:     a.lastBlock.Hash,
		StateRoot:         stateRoot,
		TransactionRoot:   txRoot,
		ValidatorRoot:     validatorRoot,
		NextValidatorRoot: nextValidatorRoot,
		ProposerAddress:   header.ProposerAddress,
		QuorumCertificate: header.QuorumCertificate,
	}
	hash, err := compare.SetHash()
	if err != nil {
		return err
	}
	if !bytes.Equal(hash, header.Hash) {
		return lib.ErrUnequalBlockHash()
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
