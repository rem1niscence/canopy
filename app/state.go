package app

import (
	"github.com/ginchuco/ginchu/crypto"
	"github.com/ginchuco/ginchu/state_machine"
	lib "github.com/ginchuco/ginchu/types"
)

type State struct {
	store      lib.StoreI
	lastBlock  *lib.BlockHeader
	beginBlock *lib.BeginBlockParams
	fsm        state_machine.StateMachine
}

func NewState(store lib.StoreI, beginBlock *lib.BeginBlockParams) State {
	return State{store: store, beginBlock: beginBlock, fsm: state_machine.StateMachine{}}
}

func (s *State) TxnWrap() (lib.StoreTxnI, func()) {
	txn := s.store.NewTxn()
	s.fsm.SetStore(txn)
	return txn, func() { s.fsm.SetStore(s.store); txn.Discard() }
}

func (s *State) BeginBlock() lib.ErrorI                      { return s.fsm.BeginBlock(s.beginBlock) }
func (s *State) EndBlock() (*lib.EndBlockParams, lib.ErrorI) { return s.fsm.EndBlock() }
func (s *State) ApplyTransaction(tx []byte, index int) (*lib.TxResult, lib.ErrorI) {
	return s.fsm.ApplyTransaction(uint64(index), tx, crypto.HashString(tx))
}
func (s *State) SetStore(store lib.StoreI) { s.store = store }
func (s *State) Reset()                    { s.store.Reset() }
func (s *State) ResetToBeginBlock() lib.ErrorI {
	s.Reset()
	return s.BeginBlock()
}
func (s *State) Height() uint64 { return s.store.Version() }

type ChainParams struct {
	SelfAddress   crypto.AddressI
	NetworkID     []byte
	MaxBlockBytes uint64
}
