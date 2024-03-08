package state_machine

import (
	"github.com/ginchuco/ginchu/crypto"
	"github.com/ginchuco/ginchu/state_machine/types"
	lib "github.com/ginchuco/ginchu/types"
)

func (s *StateMachine) GetValidator(address crypto.AddressI) (*types.Validator, lib.ErrorI) {
	store := s.Store()
	bz, err := store.Get(types.KeyForValidator(address))
	if err != nil {
		return nil, types.ErrStoreGet(err)
	}
	return s.unmarshalValidator(bz)
}

func (s *StateMachine) GetValidators() ([]*types.Validator, lib.ErrorI) {
	store := s.Store()
	it, er := store.Iterator(types.ValidatorPrefix())
	if er != nil {
		return nil, types.ErrStoreIter(er)
	}
	defer it.Close()
	var result []*types.Validator
	for ; it.Valid(); it.Next() {
		val, err := s.unmarshalValidator(it.Value())
		if err != nil {
			return nil, err
		}
		result = append(result, val)
	}
	return result, nil
}

func (s *StateMachine) SetValidator(validator *types.Validator) lib.ErrorI {
	store := s.Store()
	bz, err := s.marshalValidator(validator)
	if err != nil {
		return err
	}
	address, er := crypto.NewAddressFromString(validator.Address)
	if er != nil {
		return types.ErrAddressFromString(er)
	}
	if er = store.Set(types.KeyForValidator(address), bz); err != nil {
		return types.ErrStoreSet(er)
	}
	return nil
}

func (s *StateMachine) DeleteValidator(address crypto.AddressI) lib.ErrorI {
	store := s.Store()
	if err := store.Delete(types.KeyForValidator(address)); err != nil {
		return types.ErrStoreDelete(err)
	}
	return nil
}

func (s *StateMachine) marshalValidator(validator *types.Validator) ([]byte, lib.ErrorI) {
	return types.Marshal(validator)
}

func (s *StateMachine) unmarshalValidator(bz []byte) (*types.Validator, lib.ErrorI) {
	val := new(types.Validator)
	if err := types.Unmarshal(bz, val); err != nil {
		return nil, err
	}
	return val, nil
}
