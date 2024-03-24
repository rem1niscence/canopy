package app

import (
	"github.com/ginchuco/ginchu/crypto"
	"github.com/ginchuco/ginchu/state_machine/types"
	lib "github.com/ginchuco/ginchu/types"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type App struct {
	State
	mempool Mempool

	params ChainParams
}

func (a *App) HandleTransaction(tx []byte) lib.ErrorI {
	hash := crypto.Hash(tx)
	if err := a.checkForDuplicateTx(hash); err != nil {
		return err
	}
	return a.mempool.HandleTransaction(tx)
}

func (a *App) ApplyBlock(block *lib.Block) lib.ErrorI {
	_, cleanup := a.TxnWrap()
	defer cleanup()
	if err := a.BeginBlock(); err != nil {
		return err
	}
	for index, tx := range block.Transactions {
		if _, err := a.ApplyTransaction(tx, index); err != nil {
			return err
		}
	}
	if _, err := a.EndBlock(); err != nil {
		return err
	}
	return nil
}

func (a *App) ApplyAndWriteBlock(block *lib.Block) lib.ErrorI {
	txn, cleanup := a.TxnWrap()
	defer cleanup()
	if err := a.ApplyBlock(block); err != nil {
		return err
	}
	for _, tx := range block.Transactions {
		a.mempool.DeleteTransaction(tx)
	}
	if err := a.mempool.CheckMempool(); err != nil {
		return err
	}
	return txn.Write()
}

func (a *App) ProduceBlock() (*lib.Block, lib.ErrorI) {
	numTxs, transactions := a.mempool.GetTransactions(a.params.MaxBlockBytes)
	header := &lib.BlockHeader{
		Height:          a.Height() + 1,
		NetworkId:       a.params.NetworkID,
		Time:            timestamppb.Now(),
		NumTxs:          uint64(numTxs),
		TotalTxs:        a.lastBlock.TotalTxs + uint64(numTxs),
		LastBlockHash:   a.lastBlock.Hash,
		ProposerAddress: a.params.SelfAddress.Bytes(),
	}
	return &lib.Block{
		BlockHeader:  header,
		Transactions: transactions,
	}, nil
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
