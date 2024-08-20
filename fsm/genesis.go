package fsm

import (
	"encoding/json"
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"os"
	"path/filepath"
)

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
	if err := s.SetSupply(supply); err != nil {
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
	genesis.ConsValidators, err = s.GetConsensusValidators(true)
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
	genesis.Supply, err = s.GetSupply()
	if err != nil {
		return nil, err
	}
	return genesis, nil
}

func (s *StateMachine) GenesisBlockHeader() (*lib.BlockHeader, lib.ErrorI) {
	genesis, err := s.ReadGenesisFromFile()
	if err != nil {
		return nil, err
	}
	return &lib.BlockHeader{
		Hash:              lib.MaxHash,
		NetworkId:         s.Config.NetworkID,
		Time:              genesis.Time,
		LastBlockHash:     lib.MaxHash,
		StateRoot:         lib.MaxHash,
		TransactionRoot:   lib.MaxHash,
		ValidatorRoot:     lib.MaxHash,
		NextValidatorRoot: lib.MaxHash,
		ProposerAddress:   lib.MaxAddress,
	}, nil
}
