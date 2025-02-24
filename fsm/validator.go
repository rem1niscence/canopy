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
				supply.CommitteeStaked = supplyFromState.CommitteeStaked
			case true:
				supply.DelegatedOnly += val.StakedAmount
				if err := s.SetDelegations(crypto.NewAddressFromBytes(val.Address), val.StakedAmount, val.Committees); err != nil {
					return err
				}
				supplyFromState, err := s.GetSupply()
				if err != nil {
					return err
				}
				supply.CommitteeStaked = supplyFromState.CommitteeStaked
				supply.CommitteeDelegatedOnly = supplyFromState.CommitteeDelegatedOnly
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
// NOTE: new stake amount must be GTE the previous stake amount
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
	// subtract from staked supply
	if err := s.SubFromStakedSupply(validator.StakedAmount); err != nil {
		return err
	}
	// subtract from delegate supply
	if validator.Delegate {
		// subtract those tokens from the delegate supply count
		if err := s.SubFromDelegatedSupply(validator.StakedAmount); err != nil {
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
	if err := s.Set(types.KeyForUnstaking(finishUnstakingHeight, address), []byte{0x0}); err != nil {
		return err
	}
	// if validator is 'paused' - unpause them
	if validator.MaxPausedHeight != 0 {
		if err := s.SetValidatorUnpaused(address, validator); err != nil {
			return err
		}
	}
	// set the height at which the validator finishes unstaking
	validator.UnstakingHeight = finishUnstakingHeight
	// update the validator structure
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
	if err := s.Set(types.KeyForPaused(maxPausedHeight, address), []byte{0x0}); err != nil {
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
	// ensure not nil
	if validator == nil {
		return nil, types.ErrValidatorNotExists()
	}
	// return the operator only if custodial
	if bytes.Equal(validator.Address, validator.Output) {
		return [][]byte{validator.Address}, nil
	}
	// return the operator and output
	return [][]byte{validator.Address, validator.Output}, nil
}

// LotteryWinner() selects a validator/delegate randomly weighted based on their stake within a committee
// if there's no committee, then fallback to the proposer's address
func (s *StateMachine) LotteryWinner(id uint64, validators ...bool) (lottery *lib.LotteryWinner, err lib.ErrorI) {
	var p lib.ValidatorSet
	// if validators
	if len(validators) == 1 && validators[0] == true {
		p, _ = s.GetCommitteeMembers(s.Config.ChainId)
	} else {
		// else get the delegates
		p, _ = s.GetAllDelegates(id)
	}
	// get the validator params from state
	valParams, err := s.GetParamsVal()
	if err != nil {
		return nil, err
	}
	// if there are no validators in the set - return
	if p.NumValidators == 0 {
		return &lib.LotteryWinner{Winner: nil, Cut: valParams.DelegateRewardPercentage}, nil
	}
	// get the last proposers
	lastProposers, err := s.GetLastProposers()
	if err != nil {
		return nil, err
	}
	// use un-grindable weighted pseudorandom
	return &lib.LotteryWinner{
		Winner: lib.WeightedPseudorandom(&lib.PseudorandomParams{
			SortitionData: &lib.SortitionData{
				LastProposerAddresses: lastProposers.Addresses,
				Height:                s.Height(),
				TotalValidators:       p.NumValidators,
				TotalPower:            p.TotalPower,
			}, ValidatorSet: p.ValidatorSet,
		}).Address().Bytes(), Cut: valParams.DelegateRewardPercentage,
	}, nil
}

// pubKeyBytesToAddress() is a convenience function that converts a public key to an address
func (s *StateMachine) pubKeyBytesToAddress(public []byte) ([]byte, lib.ErrorI) {
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
