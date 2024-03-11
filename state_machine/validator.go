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
	address := crypto.NewAddressFromBytes(validator.Address)
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

func (s *StateMachine) SetValidatorsPaused(params *types.ValidatorParams, addresses []crypto.AddressI) lib.ErrorI {
	maxPausedHeight := s.Height() + params.ValidatorMaxPauseBlocks.Value
	for _, address := range addresses {
		validator, err := s.GetValidator(address)
		if err != nil {
			return err
		}
		if validator.MaxPausedHeight != 0 {
			return types.ErrValidatorPaused()
		}
		if err = s.SetValidatorPaused(address, validator, maxPausedHeight); err != nil {
			return err
		}
	}
	return nil
}

func (s *StateMachine) SetValidatorPaused(address crypto.AddressI, validator *types.Validator, maxPausedHeight uint64) lib.ErrorI {
	if err := s.Set(types.KeyForPaused(maxPausedHeight, address), nil); err != nil {
		return err
	}
	validator.MaxPausedHeight = maxPausedHeight
	return s.SetValidator(validator)
}

func (s *StateMachine) SetValidatorUnpaused(address crypto.AddressI, validator *types.Validator) lib.ErrorI {
	if err := s.Delete(types.KeyForPaused(validator.MaxPausedHeight, address)); err != nil {
		return err
	}
	validator.MaxPausedHeight = 0
	return s.SetValidator(validator)
}

func (s *StateMachine) DeletePaused(height uint64) lib.ErrorI {
	var keys [][]byte
	setValidatorUnstakingCallback := func(key, _ []byte) lib.ErrorI {
		keys = append(keys, key)
		addr, err := types.AddressFromKey(key)
		if err != nil {
			return err
		}
		return s.ForceUnstakeValidator(addr)
	}
	if err := s.IterateAndExecute(types.PausedPrefix(height), setValidatorUnstakingCallback); err != nil {
		return err
	}
	return s.DeleteAll(keys)
}

func (s *StateMachine) DeleteUnstaking(height uint64) lib.ErrorI {
	var keys [][]byte
	deleteValidatorCallback := func(key, _ []byte) lib.ErrorI {
		keys = append(keys, key)
		addr, err := types.AddressFromKey(key)
		if err != nil {
			return err
		}
		return s.DeleteValidator(addr)
	}
	if err := s.IterateAndExecute(types.UnstakingPrefix(height), deleteValidatorCallback); err != nil {
		return err
	}
	return s.DeleteAll(keys)
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
