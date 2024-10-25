package fsm

import (
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
)

// TODO ensure a 0 committee validator does not panic

// GetValidator() gets the validator from the store via the address
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

// GetValidatorExists() checks if the Validator already exists in the state
func (s *StateMachine) GetValidatorExists(address crypto.AddressI) (bool, lib.ErrorI) {
	bz, err := s.Get(types.KeyForValidator(address))
	if err != nil {
		return false, err
	}
	return bz != nil, nil
}

// GetValidators() returns a slice of all validators
func (s *StateMachine) GetValidators() ([]*types.Validator, lib.ErrorI) {
	it, err := s.Iterator(types.ValidatorPrefix())
	if err != nil {
		return nil, err
	}
	defer it.Close()
	var result []*types.Validator
	for ; it.Valid(); it.Next() {
		val, e := s.unmarshalValidator(it.Value())
		if e != nil {
			return nil, e
		}
		result = append(result, val)
	}
	return result, nil
}

// GetValidatorsPaginated() returns a page of filtered validators
func (s *StateMachine) GetValidatorsPaginated(p lib.PageParams, f lib.ValidatorFilters) (page *lib.Page, err lib.ErrorI) {
	return s.getValidatorsPaginated(p, f, types.ValidatorPrefix())
}

// getValidatorsPaginated() a helper function that returns a filtered page of Validators under a specific prefix key
func (s *StateMachine) getValidatorsPaginated(p lib.PageParams, f lib.ValidatorFilters, prefix []byte) (page *lib.Page, err lib.ErrorI) {
	// initialize a page and the results slice
	page, res := lib.NewPage(p, types.ValidatorsPageName), make(types.ValidatorPage, 0)
	if f.On() { // if request has filters
		// create a new iterator for the prefix key
		it, e := s.Iterator(prefix)
		if e != nil {
			return nil, e
		}
		defer it.Close()
		var filteredVals []*types.Validator
		// pre-filter the possible candidates
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
		// load the array (slice) into the page
		err = page.LoadArray(filteredVals, &res, func(i any) (e lib.ErrorI) {
			v, ok := i.(*types.Validator)
			if !ok {
				return lib.ErrInvalidArgument()
			}
			res = append(res, v)
			return
		})
	} else { // if no filters
		err = page.Load(prefix, false, &res, s.store, func(_, b []byte) (err lib.ErrorI) {
			val, err := s.unmarshalValidator(b)
			if err == nil {
				res = append(res, val)
			}
			return
		})
	}
	return
}

// SetValidators() upserts multiple Validators into the state and updates the supply
func (s *StateMachine) SetValidators(validators []*types.Validator, supply *types.Supply) lib.ErrorI {
	for _, val := range validators {
		supply.Total += val.StakedAmount
		supply.Staked += val.StakedAmount
		if err := s.SetValidator(val); err != nil {
			return err
		}
		if val.MaxPausedHeight == 0 && val.UnstakingHeight == 0 {
			switch val.Delegate {
			case false:
				if err := s.SetCommittees(crypto.NewAddressFromBytes(val.Address), val.StakedAmount, val.Committees); err != nil {
					return err
				}
				supplyFromState, err := s.GetSupply()
				if err != nil {
					return err
				}
				supply.CommitteesWithDelegations = supplyFromState.CommitteesWithDelegations
			case true:
				supply.Delegated += val.StakedAmount
				if err := s.SetDelegations(crypto.NewAddressFromBytes(val.Address), val.StakedAmount, val.Committees); err != nil {
					return err
				}
				supplyFromState, err := s.GetSupply()
				if err != nil {
					return err
				}
				supply.CommitteesWithDelegations = supplyFromState.CommitteesWithDelegations
				supply.DelegationsOnly = supplyFromState.DelegationsOnly
			}
		}
	}
	return nil
}

// SetValidator() upserts a Validator object into the state
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

// UpdateValidatorStake() updates the stake of the validator object in state - updating the corresponding committees and supply
func (s *StateMachine) UpdateValidatorStake(val *types.Validator, newCommittees []uint64, amountToAdd uint64) (err lib.ErrorI) {
	// create address object
	address := crypto.NewAddress(val.Address)
	// update staked supply
	if err = s.AddToStakedSupply(amountToAdd); err != nil {
		return err
	}
	// calculate the new stake amount
	newStakedAmount := val.StakedAmount + amountToAdd
	// if the validator is a delegate or not
	if val.Delegate {
		// track total delegated tokens
		if err = s.AddToDelegateSupply(amountToAdd); err != nil {
			return err
		}
		// update the delegations with the new committeeIds and stake amount
		if err = s.UpdateDelegations(address, val, newStakedAmount, newCommittees); err != nil {
			return err
		}
	} else {
		// update the committees with the new committeeIds and stake amount
		if err = s.UpdateCommittees(address, val, newStakedAmount, newCommittees); err != nil {
			return err
		}
	}
	// update the validator committees in the structure
	val.Committees = newCommittees
	// update the stake amount in the structure
	val.StakedAmount = newStakedAmount
	// set validator
	return s.SetValidator(val)
}

// DeleteValidator() completely removes a validator from the state
func (s *StateMachine) DeleteValidator(validator *types.Validator) lib.ErrorI {
	addr := crypto.NewAddress(validator.Address)
	// delete the validator committee information
	if validator.Delegate {
		if err := s.DeleteDelegations(addr, validator.StakedAmount, validator.Committees); err != nil {
			return err
		}
	} else {
		if err := s.DeleteCommittees(addr, validator.StakedAmount, validator.Committees); err != nil {
			return err
		}
	}
	// delete the validator from state
	return s.Delete(types.KeyForValidator(addr))
}

// UNSTAKING VALIDATORS BELOW

// SetValidatorUnstaking() updates a Validator as 'unstaking' and removes it from its respective committees
// NOTE: finish unstaking height is (likely) not current height, but one in the future when the validator will 'complete' unstaking and their
// funds be returned
func (s *StateMachine) SetValidatorUnstaking(address crypto.AddressI, validator *types.Validator, finishUnstakingHeight uint64) lib.ErrorI {
	if err := s.Set(types.KeyForUnstaking(finishUnstakingHeight, address), nil); err != nil {
		return err
	}
	validator.UnstakingHeight = finishUnstakingHeight // height at which the validator finishes unstaking
	return s.SetValidator(validator)
}

// DeleteFinishedUnstaking() deletes the Validator structure and unstaking keys for those who have finished unstaking
func (s *StateMachine) DeleteFinishedUnstaking() lib.ErrorI {
	var toDelete [][]byte
	// for each unstaking key at this height
	callback := func(key, _ []byte) lib.ErrorI {
		// add to the 'will delete' list
		toDelete = append(toDelete, key)
		// get the address from the key
		addr, err := types.AddressFromKey(key)
		if err != nil {
			return err
		}
		// get the validator associated with that address
		validator, err := s.GetValidator(addr)
		if err != nil {
			return err
		}
		// transfer the staked tokens to the designated output address
		if err = s.AccountAdd(crypto.NewAddressFromBytes(validator.Output), validator.StakedAmount); err != nil {
			return err
		}
		// subtract those tokens from the staked supply count
		if err = s.SubFromStakedSupply(validator.StakedAmount); err != nil {
			return err
		}
		if validator.Delegate {
			// subtract those tokens from the delegate supply count
			if err = s.SubFromDelegatedSupply(validator.StakedAmount); err != nil {
				return err
			}
		}
		// delete the validator structure
		return s.DeleteValidator(validator)
	}
	// for each unstaking key at this height
	if err := s.IterateAndExecute(types.UnstakingPrefix(s.Height()), callback); err != nil {
		return err
	}
	// delete all unstaking keys
	return s.DeleteAll(toDelete)
}

// PAUSED VALIDATORS BELOW

// SetValidatorsPaused() automatically updates all validators as if they'd submitted a MessagePause
func (s *StateMachine) SetValidatorsPaused(addresses [][]byte) lib.ErrorI {
	for _, addr := range addresses {
		if err := s.HandleMessagePause(&types.MessagePause{Address: addr}); err != nil {
			s.log.Debugf("can't pause validator %s with err %s", lib.BytesToString(addr), err.Error())
			continue
		}
	}
	return nil
}

// SetValidatorPaused() updates a Validator as 'paused' with a MaxPausedHeight (height at which the Validator is force-unstaked for being paused too long)
func (s *StateMachine) SetValidatorPaused(address crypto.AddressI, validator *types.Validator, maxPausedHeight uint64) lib.ErrorI {
	if err := s.Set(types.KeyForPaused(maxPausedHeight, address), nil); err != nil {
		return err
	}
	validator.MaxPausedHeight = maxPausedHeight
	return s.SetValidator(validator)
}

// SetValidatorUnpaused() updates a Validator as 'unpaused'
func (s *StateMachine) SetValidatorUnpaused(address crypto.AddressI, validator *types.Validator) lib.ErrorI {
	if err := s.Delete(types.KeyForPaused(validator.MaxPausedHeight, address)); err != nil {
		return err
	}
	validator.MaxPausedHeight = 0
	return s.SetValidator(validator)
}

// ForceUnstakeMaxPaused() force unstakes validators who have reached MaxPauseHeight and deletes their 'paused' key
// EXPLAINER: addresses under the (max) paused prefix for the latest height indicate the Validator
// reached their 'max paused height' as this key was set some heights ago when the validators were initially paused.
// Note that these Validators still remain paused or else the key would have been deleted upon 'un-pausing'
func (s *StateMachine) ForceUnstakeMaxPaused() lib.ErrorI {
	var keys [][]byte
	// this callback force unstakes all addresses under the prefix
	forceUnstakeCallback := func(key, _ []byte) lib.ErrorI {
		keys = append(keys, key)
		addr, err := types.AddressFromKey(key)
		if err != nil {
			return err
		}
		return s.ForceUnstakeValidator(addr)
	}
	// force unstake all addresses under the (max) paused prefix for the latest height
	if err := s.IterateAndExecute(types.PausedPrefix(s.Height()), forceUnstakeCallback); err != nil {
		return err
	}
	return s.DeleteAll(keys)
}

// GetAuthorizedSignersForValidator() returns the addresses that are able to sign messages on behalf of the validator
func (s *StateMachine) GetAuthorizedSignersForValidator(address []byte) (signers [][]byte, err lib.ErrorI) {
	// retrieve the validator from state
	validator, err := s.GetValidator(crypto.NewAddressFromBytes(address))
	if err != nil {
		return nil, err
	}
	// ensure not nil
	if validator == nil {
		return nil, types.ErrValidatorNotExists()
	}
	// return the operator and output
	return [][]byte{validator.Address, validator.Output}, nil
}

// validatorPubToAddr() is a convenience function that converts a BLS validator key to an address
func (s *StateMachine) validatorPubToAddr(public []byte) ([]byte, lib.ErrorI) {
	pk, er := crypto.NewPublicKeyFromBytes(public)
	if er != nil {
		return nil, types.ErrInvalidPublicKey(er)
	}
	return pk.Address().Bytes(), nil
}

// marshalValidator() converts the Validator object to bytes
func (s *StateMachine) marshalValidator(validator *types.Validator) ([]byte, lib.ErrorI) {
	return lib.Marshal(validator)
}

// unmarshalValidator() converts bytes into a Validator object
func (s *StateMachine) unmarshalValidator(bz []byte) (*types.Validator, lib.ErrorI) {
	val := new(types.Validator)
	if err := lib.Unmarshal(bz, val); err != nil {
		return nil, err
	}
	return val, nil
}
