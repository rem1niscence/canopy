package fsm

import (
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
)

func (s *StateMachine) SlashAndResetNonSigners(params *types.ValidatorParams) lib.ErrorI {
	var keys, addrs [][]byte
	callback := func(k, v []byte) lib.ErrorI {
		addr, err := types.AddressFromKey(k)
		if err != nil {
			return err
		}
		ptr := new(types.NonSignerInfo)
		if err = lib.Unmarshal(v, ptr); err != nil {
			return err
		}
		if ptr.Counter > params.ValidatorMaxNonSign {
			addrs = append(addrs, addr.Bytes())
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

func (s *StateMachine) GetNonSigners() (results types.NonSigners, e lib.ErrorI) {
	it, e := s.Iterator(types.NonSignerPrefix())
	if e != nil {
		return nil, e
	}
	defer it.Close()
	for ; it.Valid(); it.Next() {
		addr, err := types.AddressFromKey(it.Key())
		if err != nil {
			return nil, err
		}
		ptr := new(types.NonSignerInfo)
		if err = lib.Unmarshal(it.Value(), ptr); err != nil {
			return nil, err
		}
		results = append(results, &types.NonSigner{
			Address: addr.Bytes(),
			Counter: ptr.Counter,
		})
	}
	return results, nil
}

func (s *StateMachine) IncrementNonSigners(nonSigners [][]byte) lib.ErrorI {
	for _, nonSigner := range nonSigners {
		key := types.KeyForNonSigner(nonSigner)
		ptr := new(types.NonSignerInfo)
		bz, err := s.Get(key)
		if err != nil {
			return err
		}
		if err = lib.Unmarshal(bz, ptr); err != nil {
			return err
		}
		ptr.Counter++
		bz, err = lib.Marshal(ptr)
		if err != nil {
			return err
		}
		if err = s.Set(key, bz); err != nil {
			return err
		}
	}
	return nil
}

func (s *StateMachine) SlashNonSigners(params *types.ValidatorParams, nonSigners [][]byte) lib.ErrorI {
	return s.SlashValidators(nonSigners, params.ValidatorNonSignSlashPercentage, params)
}

func (s *StateMachine) SlashBadProposers(params *types.ValidatorParams, badProposers [][]byte) lib.ErrorI {
	return s.SlashValidators(badProposers, params.ValidatorBadProposalSlashPercentage, params)
}

func (s *StateMachine) SlashDoubleSigners(params *types.ValidatorParams, doubleSigners [][]byte) lib.ErrorI {
	return s.SlashValidators(doubleSigners, params.ValidatorDoubleSignSlashPercentage, params)
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
	// set validator unstaking
	return s.forceUnstakeValidator(address, validator, p)
}

func (s *StateMachine) forceUnstakeValidator(address crypto.AddressI, val *types.Validator, p *types.ValidatorParams) lib.ErrorI {
	unstakingBlocks := p.GetValidatorUnstakingBlocks()
	unstakingHeight := s.Height() + unstakingBlocks
	// set validator unstaking
	return s.SetValidatorUnstaking(address, val, unstakingHeight)
}

func (s *StateMachine) SlashValidators(addresses [][]byte, percent uint64, p *types.ValidatorParams) lib.ErrorI {
	for _, addr := range addresses {
		validator, err := s.GetValidator(crypto.NewAddressFromBytes(addr))
		if err != nil {
			return err
		}
		if err = s.SlashValidator(validator, percent, p); err != nil {
			return err
		}
	}
	return nil
}

func (s *StateMachine) SlashValidator(validator *types.Validator, percent uint64, p *types.ValidatorParams) lib.ErrorI {
	if percent > 100 {
		return types.ErrInvalidSlashPercentage()
	}
	oldStake := validator.StakedAmount
	validator.StakedAmount = lib.Uint64ReducePercentage(validator.StakedAmount, int8(percent))
	if err := s.SubFromStakedSupply(oldStake - validator.StakedAmount); err != nil {
		return err
	}
	if validator.StakedAmount < p.ValidatorMinStake {
		return s.forceUnstakeValidator(crypto.NewAddressFromBytes(validator.Address), validator, p)
	}
	if err := s.UpdateConsensusValidator(crypto.NewAddressFromBytes(validator.Address), oldStake, validator.StakedAmount); err != nil {
		return err
	}
	return s.SetValidator(validator)
}

func (s *StateMachine) GetMinimumEvidenceHeight() (uint64, lib.ErrorI) {
	valParams, err := s.GetParamsVal()
	if err != nil {
		return 0, err
	}
	height, unstakingBlocks := s.Height(), valParams.GetValidatorUnstakingBlocks()
	if height < unstakingBlocks {
		return 0, nil
	}
	return height - unstakingBlocks, nil
}
