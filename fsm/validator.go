package fsm

import (
	"bytes"
	"github.com/canopy-network/canopy/fsm/types"
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
	"slices"
)

/* This file implements state actions on Validators and Delegators*/

// GetValidator() gets the validator from the store via the address
func (s *StateMachine) GetValidator(address crypto.AddressI) (*types.Validator, lib.ErrorI) {
	// get the bytes from state using the key for a validator at a specific address
	bz, err := s.Get(types.KeyForValidator(address))
	if err != nil {
		return nil, err
	}
	// if the bytes are empty, return 'validator doesn't exist'
	if bz == nil {
		return nil, types.ErrValidatorNotExists()
	}
	// convert the bytes into a validator object reference
	val, err := s.unmarshalValidator(bz)
	if err != nil {
		return nil, err
	}
	// update the validator structure address
	val.Address = address.Bytes()
	// return the validator
	return val, nil
}

// GetValidatorExists() checks if the Validator already exists in the state
func (s *StateMachine) GetValidatorExists(address crypto.AddressI) (bool, lib.ErrorI) {
	// get the bytes from state using the key for a validator at a specific address
	bz, err := s.Get(types.KeyForValidator(address))
	if err != nil {
		return false, err
	}
	// return true if validator bytes are non-nil
	return bz != nil, nil
}

// GetValidators() returns a slice of all validators
func (s *StateMachine) GetValidators() (result []*types.Validator, err lib.ErrorI) {
	// create an iterator to traverse all keys under the 'ValidatorPrefix'
	it, err := s.Iterator(types.ValidatorPrefix())
	if err != nil {
		return nil, err
	}
	// ensure memory cleanup
	defer it.Close()
	// for each item of the iterator
	for ; it.Valid(); it.Next() {
		// convert the bytes into a validator object reference
		val, e := s.unmarshalValidator(it.Value())
		if e != nil {
			return nil, e
		}
		// add it to the list
		result = append(result, val)
	}
	// exit
	return
}

// GetValidatorsPaginated() returns a page of filtered validators
func (s *StateMachine) GetValidatorsPaginated(p lib.PageParams, f lib.ValidatorFilters) (page *lib.Page, err lib.ErrorI) {
	// initialize a page and the results slice
	page, res := lib.NewPage(p, types.ValidatorsPageName), make(types.ValidatorPage, 0)
	// if the request has no filters
	if !f.On() {
		// populate the page using the validators prefix (validators are stored lexicographically not ordered stake)
		err = page.Load(types.ValidatorPrefix(), false, &res, s.store, func(_, value []byte) (err lib.ErrorI) {
			// convert the value into a validator object reference
			val, err := s.unmarshalValidator(value)
			// if there's no error
			if err == nil {
				// add to the list
				res = append(res, val)
			}
			// exit
			return
		})
		// exit
		return
	}
	// create a new iterator for the prefix key
	it, e := s.Iterator(types.ValidatorPrefix())
	if e != nil {
		return nil, e
	}
	// ensure memory cleanup
	defer it.Close()
	// create a variable to hold the list of filtered validators
	var filteredVals []*types.Validator
	// for each item in the iterator
	for ; it.Valid(); it.Next() {
		// convert the bytes into a validator object reference
		val, er := s.unmarshalValidator(it.Value())
		if er != nil {
			return nil, er
		}
		// pre-filter the possible candidates
		if val.PassesFilter(f) {
			// add to the list
			filteredVals = append(filteredVals, val)
		}
	}
	// populate the page with the list of filtered validators
	err = page.LoadArray(filteredVals, &res, func(i any) (e lib.ErrorI) {
		// cast to validator
		v, ok := i.(*types.Validator)
		// ensure the cast was successful
		if !ok {
			return lib.ErrInvalidArgument()
		}
		// add to the resulting page
		res = append(res, v)
		// exit
		return
	})
	return
}

// SetValidators() upserts multiple Validators into the state and updates the supply tracker
func (s *StateMachine) SetValidators(validators []*types.Validator, supply *types.Supply) lib.ErrorI {
	// for each validator in the list
	for _, val := range validators {
		// if the unstaking height or the max paused height is set
		if val.UnstakingHeight != 0 {
			// if the validator is unstaking - update it accordingly in state
			if err := s.SetValidatorUnstaking(crypto.NewAddress(val.Address), val, val.UnstakingHeight); err != nil {
				return err
			}
		} else if val.MaxPausedHeight != 0 {
			// if validator is paused - update it accordingly
			if err := s.SetValidatorPaused(crypto.NewAddress(val.Address), val, val.MaxPausedHeight); err != nil {
				return err
			}
		}
		// add to 'total supply' in the supply tracker
		supply.Total += val.StakedAmount
		// add to 'staked supply' in the supply tracker
		supply.Staked += val.StakedAmount
		// set the validator structure in state
		if err := s.SetValidator(val); err != nil {
			return err
		}
		// if the validator is a 'delegate'
		if val.Delegate {
			// add to the delegation supply
			supply.DelegatedOnly += val.StakedAmount
			// set the delegations in state
			if err := s.SetDelegations(crypto.NewAddressFromBytes(val.Address), val.StakedAmount, val.Committees); err != nil {
				return err
			}
		} else {
			if err := s.SetCommittees(crypto.NewAddressFromBytes(val.Address), val.StakedAmount, val.Committees); err != nil {
				return err
			}
		}
	}
	//
	// get the 'supply tracker' from state to update the local 'supply tracker' with the automatically populated committee/delegate staked pool
	supplyFromState, err := s.GetSupply()
	if err != nil {
		return err
	}
	// update the committee staked pool
	supply.CommitteeStaked = supplyFromState.CommitteeStaked
	// update the delegate staked pool
	supply.CommitteeDelegatedOnly = supplyFromState.CommitteeDelegatedOnly
	return nil
}

// SetValidator() upserts a Validator object into the state
func (s *StateMachine) SetValidator(validator *types.Validator) (err lib.ErrorI) {
	// covert the validator object to bytes
	bz, err := s.marshalValidator(validator)
	if err != nil {
		return
	}
	// set the bytes under a key for validator using a specific 'validator address'
	if err = s.Set(types.KeyForValidator(crypto.NewAddressFromBytes(validator.Address)), bz); err != nil {
		return
	}
	// exit
	return
}

// UpdateValidatorStake() updates the stake of the validator object in state - updating the corresponding committees and supply
// NOTE: new stake amount must be GTE the previous stake amount
func (s *StateMachine) UpdateValidatorStake(val *types.Validator, newCommittees []uint64, amountToAdd uint64) (err lib.ErrorI) {
	// convert the validator address bytes into an object reference
	address := crypto.NewAddress(val.Address)
	// update staked supply accordingly (validator stake amount can never go down except for slashing and unstaking)
	if err = s.AddToStakedSupply(amountToAdd); err != nil {
		return err
	}
	// calculate the new stake amount
	newStakedAmount := val.StakedAmount + amountToAdd
	// if the validator is a delegate or not
	if val.Delegate {
		// update the new 'total delegated tokens' amount by adding to the staked supply
		if err = s.AddToDelegateSupply(amountToAdd); err != nil {
			return err
		}
		// update the delegations with the new chainIds and stake amount
		if err = s.UpdateDelegations(address, val, newStakedAmount, newCommittees); err != nil {
			return err
		}
	} else {
		// update the committees with the new chainIds and stake amount
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
	// convert the validator address bytes into an object reference
	addr := crypto.NewAddress(validator.Address)
	// subtract from staked supply
	if err := s.SubFromStakedSupply(validator.StakedAmount); err != nil {
		return err
	}
	// delete the validator committee information
	if validator.Delegate {
		// subtract those tokens from the delegate supply count
		if err := s.SubFromDelegateSupply(validator.StakedAmount); err != nil {
			return err
		}
		// remove the delegations for the validator
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
// NOTE: finish unstaking height is the height in the future when the validator will be deleted and their
// funds be returned
func (s *StateMachine) SetValidatorUnstaking(address crypto.AddressI, validator *types.Validator, finishUnstakingHeight uint64) lib.ErrorI {
	// set an entry in the database to mark this validator as unstaking, a single byte is used to allow 'get' calls to differentiate between non-existing keys
	if err := s.Set(types.KeyForUnstaking(finishUnstakingHeight, address), []byte{0x1}); err != nil {
		return err
	}
	// if validator is 'paused' (only happens if validator is max paused)
	if validator.MaxPausedHeight != 0 {
		// update the validator as unpaused
		if err := s.SetValidatorUnpaused(address, validator); err != nil {
			return err
		}
	}
	// update the validator structure with the finishUnstakingHeight
	validator.UnstakingHeight = finishUnstakingHeight
	// update the validator structure
	return s.SetValidator(validator)
}

// DeleteFinishedUnstaking() deletes the Validator structure and unstaking keys for those who have finished unstaking
func (s *StateMachine) DeleteFinishedUnstaking() lib.ErrorI {
	// create a variable to maintain a list of the unstaking keys 'to delete'
	var toDelete [][]byte
	// for each unstaking key at this height
	callback := func(unstakingKey, _ []byte) lib.ErrorI {
		// add to the 'will delete' list
		toDelete = append(toDelete, unstakingKey)
		// get the address from the key
		addr, err := types.AddressFromKey(unstakingKey)
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
func (s *StateMachine) SetValidatorsPaused(chainId uint64, addresses [][]byte) {
	// for each validator in the list
	for _, addr := range addresses {
		// get the validator
		val, err := s.GetValidator(crypto.NewAddress(addr))
		if err != nil {
			// log error
			s.log.Debugf("can't pause validator %s not found", lib.BytesToString(addr))
			// move on to the next iteration
			continue
		}
		// ensure no unauthorized auto-pauses
		if !slices.Contains(val.Committees, chainId) {
			// NOTE: expected - this can happen during a race between edit-stake and pause
			s.log.Warnf("unauthorized pause from %d, this can happen occasionally", chainId)
			// exit
			return
		}
		// handle pausing the validator
		if err = s.HandleMessagePause(&types.MessagePause{Address: addr}); err != nil {
			// log error
			s.log.Debugf("can't pause validator %s with err %s", lib.BytesToString(addr), err.Error())
			// move on to the next iteration
			continue
		}
	}
}

// SetValidatorPaused() updates a Validator as 'paused' with a MaxPausedHeight (height at which the Validator is force-unstaked for being paused too long)
func (s *StateMachine) SetValidatorPaused(address crypto.AddressI, validator *types.Validator, maxPausedHeight uint64) lib.ErrorI {
	// set an entry in the state to mark this validator as paused, a single byte is used to allow 'get' calls to differentiate between non-existing keys
	if err := s.Set(types.KeyForPaused(maxPausedHeight, address), []byte{0x1}); err != nil {
		return err
	}
	// update the validator max paused height
	validator.MaxPausedHeight = maxPausedHeight
	// set the updated validator in state
	return s.SetValidator(validator)
}

// SetValidatorUnpaused() updates a Validator as 'unpaused'
func (s *StateMachine) SetValidatorUnpaused(address crypto.AddressI, validator *types.Validator) lib.ErrorI {
	// remove the 'paused' entry in the state to mark this validator as not paused
	if err := s.Delete(types.KeyForPaused(validator.MaxPausedHeight, address)); err != nil {
		return err
	}
	// update the validator max paused height to 0
	validator.MaxPausedHeight = 0
	// set the updated validator in state
	return s.SetValidator(validator)
}

// GetAuthorizedSignersForValidator() returns the addresses that are able to sign messages on behalf of the validator
func (s *StateMachine) GetAuthorizedSignersForValidator(address []byte) (signers [][]byte, err lib.ErrorI) {
	// retrieve the validator from state
	validator, err := s.GetValidator(crypto.NewAddressFromBytes(address))
	if err != nil {
		return nil, err
	}
	// return the operator only if custodial
	if bytes.Equal(validator.Address, validator.Output) {
		return [][]byte{validator.Address}, nil
	}
	// return the operator and output
	return [][]byte{validator.Address, validator.Output}, nil
}

// pubKeyBytesToAddress() is a convenience function that converts a public key to an address
func (s *StateMachine) pubKeyBytesToAddress(public []byte) ([]byte, lib.ErrorI) {
	// get the public key object ref from the bytes
	pk, err := crypto.NewPublicKeyFromBytes(public)
	if err != nil {
		return nil, types.ErrInvalidPublicKey(err)
	}
	// get the address bytes from the public key
	return pk.Address().Bytes(), nil
}

// marshalValidator() converts the Validator object to bytes
func (s *StateMachine) marshalValidator(validator *types.Validator) ([]byte, lib.ErrorI) {
	// convert the object ref into bytes
	return lib.Marshal(validator)
}

// unmarshalValidator() converts bytes into a Validator object
func (s *StateMachine) unmarshalValidator(bz []byte) (*types.Validator, lib.ErrorI) {
	// create a new validator object reference to ensure a non-nil result
	val := new(types.Validator)
	// populate the object reference with validator bytes
	if err := lib.Unmarshal(bz, val); err != nil {
		return nil, err
	}
	// return the object ref
	return val, nil
}
