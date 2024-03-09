package state_machine

import (
	"github.com/ginchuco/ginchu/state_machine/types"
	lib "github.com/ginchuco/ginchu/types"
)

type StateMachine struct {
	store lib.StoreI
}

func (s *StateMachine) Store() lib.StoreI { return s.store }
func (s *StateMachine) Height() uint64    { return s.Store().Version() }

func (s *StateMachine) Set(k, v []byte) lib.ErrorI {
	store := s.Store()
	if err := store.Set(k, v); err != nil {
		return types.ErrStoreSet(err)
	}
	return nil
}

func (s *StateMachine) Get(key []byte) ([]byte, lib.ErrorI) {
	store := s.Store()
	bz, err := store.Get(key)
	if err != nil {
		return nil, types.ErrStoreGet(err)
	}
	return bz, nil
}

func (s *StateMachine) Delete(key []byte) lib.ErrorI {
	store := s.Store()
	if err := store.Delete(key); err != nil {
		return types.ErrStoreDelete(err)
	}
	return nil
}

func (s *StateMachine) Iterator(key []byte) (lib.IteratorI, lib.ErrorI) {
	store := s.Store()
	it, err := store.Iterator(key)
	if err != nil {
		return nil, types.ErrStoreIter(err)
	}
	return it, nil
}

func (s *StateMachine) RevIterator(key []byte) (lib.IteratorI, lib.ErrorI) {
	store := s.Store()
	it, err := store.RevIterator(key)
	if err != nil {
		return nil, types.ErrStoreRevIter(err)
	}
	return it, nil
}
