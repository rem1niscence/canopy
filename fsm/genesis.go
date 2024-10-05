package fsm

import (
	"encoding/json"
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"os"
	"path/filepath"
)

// NewFromGenesisFile() creates a new beginning state from a file
func (s *StateMachine) NewFromGenesisFile() lib.ErrorI {
	genesis, err := s.ReadGenesisFromFile()
	if err != nil {
		return err
	}
	if err = s.NewStateFromGenesis(genesis); err != nil {
		return err
	}
	if _, err = s.store.(lib.StoreI).Commit(); err != nil {
		return err
	}
	return nil
}

// ReadGenesisFromFile() reads a GenesisState object from a file
func (s *StateMachine) ReadGenesisFromFile() (genesis *types.GenesisState, e lib.ErrorI) {
	genesis = new(types.GenesisState)
	bz, err := os.ReadFile(filepath.Join(s.Config.DataDirPath, lib.GenesisFilePath))
	if err != nil {
		return nil, types.ErrReadGenesisFile(err)
	}
	if err = json.Unmarshal(bz, genesis); err != nil {
		return nil, types.ErrUnmarshalGenesis(err)
	}
	e = s.ValidateGenesisState(genesis)
	return
}

// NewStateFromGenesis() creates a new beginning state using a GenesisState object
func (s *StateMachine) NewStateFromGenesis(genesis *types.GenesisState) lib.ErrorI {
	supply := new(types.Supply)
	if err := s.SetAccounts(genesis.Accounts, supply); err != nil {
		return err
	}
	if err := s.SetPools(genesis.Pools, supply); err != nil {
		return err
	}
	if err := s.SetValidators(genesis.Validators, supply); err != nil {
		return err
	}
	if genesis.OrderBooks != nil {
		if err := s.SetOrderBooks(genesis.OrderBooks, supply); err != nil {
			return err
		}
	}
	if err := s.SetSupply(supply); err != nil {
		return err
	}
	return s.SetParams(genesis.Params)
}

// ValidateGenesisState() validates a GenesisState object
func (s *StateMachine) ValidateGenesisState(genesis *types.GenesisState) lib.ErrorI {
	if err := genesis.Params.Check(); err != nil {
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
		if val.StakedAmount < genesis.Params.Validator.ValidatorMinStake {
			return types.ErrBelowMinimumStake()
		}
	}
	for _, account := range genesis.Accounts {
		if len(account.Address) < crypto.AddressSize {
			return types.ErrAddressSize()
		}
		if account.Amount == 0 {
			return types.ErrInvalidAmount()
		}
	}
	for _, pool := range genesis.Pools {
		if pool.Id < 0 {
			return types.ErrInvalidPoolName()
		}
	}
	return nil
}

// ExportState() creates a GenesisState object from the current state
func (s *StateMachine) ExportState() (genesis *types.GenesisState, err lib.ErrorI) {
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
	genesis.NonSigners, err = s.GetNonSigners()
	if err != nil {
		return nil, err
	}
	genesis.DoubleSigners, err = s.GetDoubleSigners()
	if err != nil {
		return nil, err
	}
	genesis.OrderBooks, err = s.GetOrderBooks()
	if err != nil {
		return nil, err
	}
	genesis.Supply, err = s.GetSupply()
	if err != nil {
		return nil, err
	}
	return genesis, nil
}
