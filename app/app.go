package app

import (
	"github.com/ginchuco/ginchu/crypto"
	"github.com/ginchuco/ginchu/state_machine"
	"github.com/ginchuco/ginchu/state_machine/types"
	lib "github.com/ginchuco/ginchu/types"
)

type App struct {
	// state machine
	store   lib.StoreI
	state   state_machine.StateMachine
	mempool Mempool
}

func (a *App) HandleTransaction(tx []byte) lib.ErrorI {
	hash := crypto.Hash(tx)
	txResult, err := a.store.GetByHash(hash)
	if err != nil {
		return err
	}
	if txResult != nil {
		return types.ErrDuplicateTx(hash)
	}
	return a.mempool.HandleTransaction(tx, hash)
}

func (a *App) HandleBlock(block []byte) lib.ErrorI {
	return nil
}

func (a *App) ProduceBlock() (lib.Block, lib.ErrorI) {
	return lib.Block{}, nil
}
