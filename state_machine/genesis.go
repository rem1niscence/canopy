package state_machine

import (
	"github.com/ginchuco/ginchu/crypto"
	"github.com/ginchuco/ginchu/state_machine/types"
	lib "github.com/ginchuco/ginchu/types"
)

func (s *StateMachine) NewStateFromGenesis(genesis *types.GenesisState) lib.ErrorI {
	if err := s.SetAccounts(genesis.Accounts); err != nil {
		return err
	}
	if err := s.SetPools(genesis.Pools); err != nil {
		return err
	}
	if err := s.SetValidators(genesis.Validators); err != nil {
		return err
	}
	return s.SetParams(genesis.Params)
}

func (s *StateMachine) ValidateGenesisState(genesis *types.GenesisState) lib.ErrorI {
	if err := genesis.Params.Validate(); err != nil {
		return err
	}
	for _, val := range genesis.Validators {
		if len(val.Address) < crypto.AddressSize {
			return types.ErrAddressSize()
		}
		if len(val.PublicKey) < crypto.Ed25519PubKeySize {
			return types.ErrAddressSize()
		}
		if len(val.Output) < crypto.AddressSize {
			return types.ErrAddressSize()
		}
		lessThanMin, err := lib.StringsLess(val.StakedAmount, genesis.Params.Validator.ValidatorMinStake.Value)
		if err != nil {
			return err
		}
		if lessThanMin {
			return types.ErrBelowMinimumStake()
		}
	}
	for _, account := range genesis.Accounts {
		if len(account.Address) < crypto.AddressSize {
			return types.ErrAddressSize()
		}
		if _, err := lib.StringToBigInt(account.Amount); err != nil {
			return err
		}
	}
	for _, pool := range genesis.Pools {
		if len(pool.Name) < 1 {
			return types.ErrInvalidPoolName()
		}
		if _, err := lib.StringToBigInt(pool.Amount); err != nil {
			return err
		}
	}
	return nil
}

func (s *StateMachine) ExportStateToGenesis() (genesis *types.GenesisState, err lib.ErrorI) {
	genesis = new(types.GenesisState)
	genesis.Accounts, err = s.GetAccounts()
	if err != nil {
		return nil, err
	}
	genesis.Pools, err = s.GetPools()
	if err != nil {
		return nil, err
	}
	genesis.Validators, err = s.GetValidators()
	if err != nil {
		return nil, err
	}
	genesis.Params, err = s.GetParams()
	if err != nil {
		return nil, err
	}
	return genesis, nil
}
