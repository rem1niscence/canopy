package app

import (
	"github.com/ginchuco/ginchu/state_machine"
	lib "github.com/ginchuco/ginchu/types"
)

type App struct {
	// state machine
	store lib.StoreI
	state state_machine.StateMachine
	Mempool
}

func (a *App) HandleBlock(block []byte) lib.ErrorI {

}

func (a *App) ProduceBlock() (lib.Block, lib.ErrorI) {

}
