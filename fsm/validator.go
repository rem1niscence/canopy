package fsm

import (
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"math"
)

func (s *StateMachine) GetValidator(address crypto.AddressI) (*types.Validator, lib.ErrorI) {
	bz, err := s.Get(types.KeyForValidator(address))
	if err != nil {
		return nil, err
	}
	if bz == nil {
		return nil, types.ErrValidatorNotExists()
	}
	val, err := s.unmarshalValidator(bz)
	if err != nil {
		return nil, err
	}
	val.Address = address.Bytes()
	return val, nil
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

func (s *StateMachine) GetValidatorsPaginated(p lib.PageParams, f lib.ValidatorFilters) (page *lib.Page, err lib.ErrorI) {
	return s.getValidatorsPaginated(p, f, types.ValidatorPrefix())
}

func (s *StateMachine) getValidatorsPaginated(p lib.PageParams, f lib.ValidatorFilters, prefix []byte) (page *lib.Page, err lib.ErrorI) {
	it, err := s.Iterator(prefix)
	if err != nil {
		return nil, err
	}
	defer it.Close()
	skipIdx, page := p.SkipToIndex(), lib.NewPage(p)
	page.Type = types.ValidatorsPageName
	res := make(types.ValidatorPage, 0)
	if f.On() {
		var filteredVals []*types.Validator
		for ; it.Valid(); it.Next() {
			var val *types.Validator
			val, err = s.unmarshalValidator(it.Value())
			if err != nil {
				return nil, err
			}
			if val.PassesFilter(f) {
				filteredVals = append(filteredVals, val)
			}
		}
		for i, countOnly := 0, false; i < len(filteredVals); i++ {
			page.TotalCount++
			switch {
			case i < skipIdx || countOnly:
				continue
			case i == skipIdx+page.PerPage:
				countOnly = true
				continue
			}
			res = append(res, filteredVals[i])
			page.Results = &res
			page.Count++
		}
	} else {
		for i, countOnly := 0, false; it.Valid(); func() { it.Next(); i++ }() {
			page.TotalCount++
			switch {
			case i < skipIdx || countOnly:
				continue
			case i == skipIdx+page.PerPage:
				countOnly = true
				continue
			}
			var val *types.Validator
			val, err = s.unmarshalValidator(it.Value())
			if err != nil {
				return nil, err
			}
			res = append(res, val)
			page.Results = &res
			page.Count++
		}
	}
	page.TotalPages = int(math.Ceil(float64(page.TotalCount) / float64(page.PerPage)))
	return
}

func (s *StateMachine) SetValidators(validators []*types.Validator, supply *types.Supply) lib.ErrorI {
	for _, val := range validators {
		supply.Total += val.StakedAmount
		supply.Staked += val.StakedAmount
		if err := s.SetValidator(val); err != nil {
			return err
		}
		if val.MaxPausedHeight == 0 && val.UnstakingHeight == 0 {
			if err := s.SetCommittees(crypto.NewAddressFromBytes(val.Address), val.StakedAmount, val.Committees); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *StateMachine) SetValidator(validator *types.Validator) lib.ErrorI {
	bz, err := s.marshalValidator(validator)
	if err != nil {
		return err
	}
	if err = s.Set(types.KeyForValidator(crypto.NewAddressFromBytes(validator.Address)), bz); err != nil {
		return err
	}
	return nil
}

func (s *StateMachine) DeleteValidator(address crypto.AddressI) lib.ErrorI {
	if err := s.Delete(types.KeyForValidator(address)); err != nil {
		return err
	}
	return nil
}

// UNSTAKING VALIDATORS BELOW

func (s *StateMachine) SetValidatorUnstaking(address crypto.AddressI, validator *types.Validator, height uint64) lib.ErrorI {
	if err := s.Set(types.KeyForUnstaking(height, address), nil); err != nil {
		return err
	}
	if validator.Delegate {
		if err := s.DeleteDelegations(address, validator.StakedAmount, validator.Committees); err != nil {
			return err
		}
	} else {
		if err := s.DeleteCommittees(address, validator.StakedAmount, validator.Committees); err != nil {
			return err
		}
	}
	validator.UnstakingHeight = height
	return s.SetValidator(validator)
}

func (s *StateMachine) DeleteUnstaking(height uint64) lib.ErrorI {
	var keys [][]byte
	callback := func(key, _ []byte) lib.ErrorI {
		keys = append(keys, key)
		addr, err := types.AddressFromKey(key)
		if err != nil {
			return err
		}
		validator, err := s.GetValidator(addr)
		if err != nil {
			return err
		}
		if err = s.AccountAdd(crypto.NewAddressFromBytes(validator.Output), validator.StakedAmount); err != nil {
			return err
		}
		if err = s.SubFromStakedSupply(validator.StakedAmount); err != nil {
			return err
		}
		return s.DeleteValidator(addr)
	}
	if err := s.IterateAndExecute(types.UnstakingPrefix(height), callback); err != nil {
		return err
	}
	return s.DeleteAll(keys)
}

// PAUSED VALIDATORS BELOW

func (s *StateMachine) SetValidatorsPaused(addresses [][]byte) lib.ErrorI {
	for _, addr := range addresses {
		if err := s.HandleMessagePause(&types.MessagePause{Address: addr}); err != nil {
			s.log.Debugf("can't pause validator %s with err %s", lib.BytesToString(addr), err.Error())
			continue
		}
	}
	return nil
}

func (s *StateMachine) SetValidatorPaused(address crypto.AddressI, validator *types.Validator, maxPausedHeight uint64) lib.ErrorI {
	if err := s.Set(types.KeyForPaused(maxPausedHeight, address), nil); err != nil {
		return err
	}
	if err := s.DeleteCommittees(address, validator.StakedAmount, validator.Committees); err != nil {
		return err
	}
	validator.MaxPausedHeight = maxPausedHeight
	return s.SetValidator(validator)
}

func (s *StateMachine) SetValidatorUnpaused(address crypto.AddressI, validator *types.Validator) lib.ErrorI {
	if err := s.Delete(types.KeyForPaused(validator.MaxPausedHeight, address)); err != nil {
		return err
	}
	if err := s.SetCommittees(address, validator.StakedAmount, validator.Committees); err != nil {
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

func (s *StateMachine) marshalValidator(validator *types.Validator) ([]byte, lib.ErrorI) {
	return lib.Marshal(validator)
}

func (s *StateMachine) unmarshalValidator(bz []byte) (*types.Validator, lib.ErrorI) {
	val := new(types.Validator)
	if err := lib.Unmarshal(bz, val); err != nil {
		return nil, err
	}
	return val, nil
}
