package state_machine

import (
	"github.com/ginchuco/ginchu/state_machine/fsm"
	lib "github.com/ginchuco/ginchu/types"
	"github.com/ginchuco/ginchu/types/crypto"
)

type State struct {
	store  lib.StoreI
	params *lib.BeginBlockParams
	chain  *ChainParams
	fsm    *fsm.StateMachine
	log    lib.Logger
}

type ChainParams struct {
	SelfAddress     crypto.AddressI
	NetworkID       []byte
	MaxBlockBytes   uint64
	ProtocolVersion int
}

func NewState(store lib.StoreI, vs *lib.ValidatorSet, lastBlock *lib.BlockHeader, chain *ChainParams, log lib.Logger) State {
	return State{
		store: store,
		params: &lib.BeginBlockParams{
			BlockHeader:  lastBlock,
			ValidatorSet: vs,
		},
		chain: chain,
		fsm:   fsm.NewStateMachine(chain.ProtocolVersion, store.Version(), store),
		log:   log,
	}
}

func (s *State) txnWrap() (lib.StoreTxnI, func()) {
	txn := s.store.NewTxn()
	s.fsm.SetStore(txn)
	return txn, func() { s.fsm.SetStore(s.store); txn.Discard() }
}

func (s *State) beginBlock() lib.ErrorI {
	return s.fsm.BeginBlock(s.params)
}
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
		fsm:    fsm.NewStateMachine(s.chain.ProtocolVersion, storeCopy.Version(), storeCopy),
		log:    s.log,
	}, nil
}
