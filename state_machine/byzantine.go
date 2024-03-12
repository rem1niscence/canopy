package state_machine

import (
	"github.com/ginchuco/ginchu/crypto"
	"github.com/ginchuco/ginchu/state_machine/types"
	lib "github.com/ginchuco/ginchu/types"
)

func (s *StateMachine) SlashAndResetNonSigners(params *types.ValidatorParams) lib.ErrorI {
	var keys [][]byte
	var addrs []crypto.AddressI
	callback := func(k, v []byte) lib.ErrorI {
		addr, err := types.AddressFromKey(k)
		if err != nil {
			return err
		}
		ptr := new(types.NonSignerInfo)
		if err = types.Unmarshal(v, ptr); err != nil {
			return err
		}
		if ptr.Counter > params.ValidatorMaxNonSign.Value {
			addrs = append(addrs, addr)
		}
		keys = append(keys, k)
		return nil
	}
	if err := s.IterateAndExecute(types.NonSignerPrefix(), callback); err != nil {
		return err
	}
	if err := s.SetValidatorsPaused(params, addrs); err != nil {
		return err
	}
	if err := s.SlashNonSigners(params, addrs); err != nil {
		return err
	}
	return s.DeleteAll(keys)
}

func (s *StateMachine) IncrementNonSigners(nonSigners []crypto.AddressI) lib.ErrorI {
	for _, nonSigner := range nonSigners {
		key := types.KeyForNonSigner(nonSigner)
		ptr := new(types.NonSignerInfo)
		bz, err := s.Get(key)
		if err != nil {
			return err
		}
		if err = types.Unmarshal(bz, ptr); err != nil {
			return err
		}
		ptr.Counter++
		bz, err = types.Marshal(ptr)
		if err != nil {
			return err
		}
		if err = s.Set(key, bz); err != nil {
			return err
		}
	}
	return nil
}

func (s *StateMachine) SlashNonSigners(params *types.ValidatorParams, nonSigners []crypto.AddressI) lib.ErrorI {
	return s.SlashValidators(nonSigners, params.ValidatorNonSignSlashPercentage.Value)
}

func (s *StateMachine) SlashBadProposers(params *types.ValidatorParams, badProposers []crypto.AddressI) lib.ErrorI {
	return s.SlashValidators(badProposers, params.ValidatorBadProposalSlashPercentage.Value)
}

func (s *StateMachine) SlashFaultySigners(params *types.ValidatorParams, faultySigners []crypto.AddressI) lib.ErrorI {
	return s.SlashValidators(faultySigners, params.ValidatorFaultySignSlashPercentage.Value)
}

func (s *StateMachine) SlashDoubleSigners(params *types.ValidatorParams, doubleSigners []crypto.AddressI) lib.ErrorI {
	return s.SlashValidators(doubleSigners, params.ValidatorDoubleSignSlashPercentage.Value)
}

func (s *StateMachine) ForceUnstakeValidator(address crypto.AddressI) lib.ErrorI {
	// get validator
	validator, err := s.GetValidator(address)
	if err != nil {
		return nil // TODO log only. Validator already deleted
	}
	// check if already unstaking
	if validator.UnstakingHeight != 0 {
		return nil // TODO log only. Validator already unstaking
	}
	// get params for unstaking blocks
	p, err := s.GetParamsVal()
	if err != nil {
		return err
	}
	unstakingBlocks := p.GetValidatorUnstakingBlocks().Value
	unstakingHeight := s.Height() + unstakingBlocks
	// set validator unstaking
	return s.SetValidatorUnstaking(address, validator, unstakingHeight)
}

func (s *StateMachine) SlashValidators(addresses []crypto.AddressI, percent uint64) lib.ErrorI {
	for _, addr := range addresses {
		validator, err := s.GetValidator(addr)
		if err != nil {
			return err
		}
		if err = s.SlashValidator(validator, percent); err != nil {
			return err
		}
	}
	return nil
}

func (s *StateMachine) SlashValidator(validator *types.Validator, percent uint64) lib.ErrorI {
	if percent > 100 {
		return types.ErrInvalidSlashPercentage()
	}
	newStake, err := lib.StringReducePercentage(validator.StakedAmount, int8(percent))
	if err != nil {
		return err
	}
	if err = s.UpdateConsensusValidator(crypto.NewAddressFromBytes(validator.Address), validator.StakedAmount, newStake); err != nil {
		return err
	}
	validator.StakedAmount = newStake
	return s.SetValidator(validator)
}
