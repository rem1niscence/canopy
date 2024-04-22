package fsm

import (
	"encoding/json"
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"os"
	"path/filepath"
	"strings"
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

func (s *StateMachine) GenesisBlockHeader() (*lib.BlockHeader, lib.ErrorI) {
	genesis, err := s.ReadGenesisFromFile()
	if err != nil {
		return nil, err
	}
	maxHash, maxAddress := []byte(strings.Repeat("F", crypto.HashSize*2)), []byte(strings.Repeat("F", crypto.AddressSize*2))
	return &lib.BlockHeader{
		Height:                0,
		Hash:                  maxHash,
		NetworkId:             s.Config.NetworkID,
		Time:                  genesis.Time,
		NumTxs:                0,
		TotalTxs:              0,
		LastBlockHash:         maxHash,
		StateRoot:             maxHash,
		TransactionRoot:       maxHash,
		ValidatorRoot:         maxHash,
		NextValidatorRoot:     maxHash,
		ProposerAddress:       maxAddress,
		BadProposers:          nil,
		LastDoubleSigners:     nil,
		LastQuorumCertificate: nil,
	}, nil
}

func (s *StateMachine) ReadGenesisFromFile() (genesis *types.GenesisState, e lib.ErrorI) {
	bz, err := os.ReadFile(filepath.Join(s.Config.DataDirPath, s.Config.GenesisFileName))
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
		if pool.Name < 0 {
			return types.ErrInvalidPoolName()
		}
		if pool.Amount == 0 {
			return types.ErrInvalidAmount()
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
