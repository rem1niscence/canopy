package state_machine

import (
	"github.com/ginchuco/ginchu/crypto"
	"github.com/ginchuco/ginchu/state_machine/types"
	lib "github.com/ginchuco/ginchu/types"
)

func (s *StateMachine) GetValidator(address crypto.AddressI) (*types.Validator, lib.ErrorI) {
	bz, err := s.Get(types.KeyForValidator(address))
	if err != nil {
		return nil, err
	}
	if bz == nil {
		return nil, types.ErrValidatorNotExists()
	}
	return s.unmarshalValidator(bz)
}

func (s *StateMachine) GetValidatorExists(address crypto.AddressI) (bool, lib.ErrorI) {
	bz, err := s.Get(types.KeyForValidator(address))
	if err != nil {
		return false, err
	}
	return bz == nil, nil
}

func (s *StateMachine) GetValidators() ([]*types.Validator, lib.ErrorI) {
	it, err := s.Iterator(types.ValidatorPrefix())
	if err != nil {
		return nil, err
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
	bz, err := s.marshalValidator(validator)
	if err != nil {
		return err
	}
	address, er := crypto.NewAddressFromString(validator.Address)
	if er != nil {
		return types.ErrAddressFromString(er)
	}
	if err = s.Set(types.KeyForValidator(address), bz); err != nil {
		return err
	}
	return nil
}

func (s *StateMachine) SetValidatorUnstaking(address crypto.AddressI, validator *types.Validator, height uint64) lib.ErrorI {
	if err := s.Set(types.KeyForUnstaking(height, address), nil); err != nil {
		return err
	}
	validator.UnstakingHeight = height
	return s.SetValidator(validator)
}

func (s *StateMachine) SetValidatorPaused(validator *types.Validator, height uint64) lib.ErrorI {
	validator.PausedHeight = height
	return s.SetValidator(validator)
}

func (s *StateMachine) SetValidatorUnpaused(validator *types.Validator) lib.ErrorI {
	validator.PausedHeight = 0
	return s.SetValidator(validator)
}

func (s *StateMachine) DeleteUnstaking(height uint64) lib.ErrorI {
	it, err := s.Iterator(types.UnstakingPrefix(height))
	if err != nil {
		return err
	}
	defer it.Close()
	var keysToDelete [][]byte
	for ; it.Valid(); it.Next() {
		keysToDelete = append(keysToDelete, it.Key())
	}
	for _, key := range keysToDelete {
		if err = s.Delete(key); err != nil {
			return err
		}
	}
	return nil
}

func (s *StateMachine) DeleteValidator(address crypto.AddressI) lib.ErrorI {
	if err := s.Delete(types.KeyForValidator(address)); err != nil {
		return err
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
