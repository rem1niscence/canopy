package fsm

import (
	"encoding/json"
	"github.com/canopy-network/canopy/fsm/types"
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
	"os"
	"path/filepath"
)

/* GENESIS LOGIC: implements logic to import a json file to create the state at height 0 and export the state at any height */

// NewFromGenesisFile() creates a new beginning state from a file
func (s *StateMachine) NewFromGenesisFile() (err lib.ErrorI) {
	// get the genesis object from a file
	genesis, err := s.ReadGenesisFromFile()
	if err != nil {
		return
	}
	// set the state using the genesis object
	if err = s.NewStateFromGenesis(genesis); err != nil {
		return
	}
	// commit the genesis state to persistence (database)
	if _, err = s.store.(lib.StoreI).Commit(); err != nil {
		return
	}
	// update the height from 0 to 1
	s.height += 1
	// exit
	return
}

// ReadGenesisFromFile() reads a GenesisState object from a file
func (s *StateMachine) ReadGenesisFromFile() (genesis *types.GenesisState, e lib.ErrorI) {
	// create a new genesis object to ensure no nil result
	genesis = new(types.GenesisState)
	// read the genesis file from the data directory + `genesis.json` path
	bz, err := os.ReadFile(filepath.Join(s.Config.DataDirPath, lib.GenesisFilePath))
	if err != nil {
		return nil, types.ErrReadGenesisFile(err)
	}
	// populate the genesis object using the file bytes
	if err = json.Unmarshal(bz, genesis); err != nil {
		return nil, types.ErrUnmarshalGenesis(err)
	}
	// ensure the genesis object is valid
	e = s.ValidateGenesisState(genesis)
	// exit
	return
}

// NewStateFromGenesis() creates a new beginning state using a GenesisState object
// There are purposefully non-included fields that are 'export only'
func (s *StateMachine) NewStateFromGenesis(genesis *types.GenesisState) (err lib.ErrorI) {
	// create a new supply tracker object reference
	supply := new(types.Supply)
	// set the accounts from the genesis object in state
	if err = s.SetAccounts(genesis.Accounts, supply); err != nil {
		return
	}
	// set the pools from the genesis object in state
	if err = s.SetPools(genesis.Pools, supply); err != nil {
		return
	}
	// set the validators from the genesis object in state
	if err = s.SetValidators(genesis.Validators, supply); err != nil {
		return
	}
	// set the order books from the genesis object in state
	if err = s.SetOrderBooks(genesis.OrderBooks, supply); err != nil {
		return
	}
	// set the calculated supply from the genesis object in state
	if err = s.SetSupply(supply); err != nil {
		return
	}
	// set the retired committees from the genesis object in state
	if err = s.SetRetiredCommittees(genesis.RetiredCommittees); err != nil {
		return
	}
	// set the governance params from the genesis object in state
	return s.SetParams(genesis.Params)
}

// ValidateGenesisState() validates a GenesisState object
func (s *StateMachine) ValidateGenesisState(genesis *types.GenesisState) (err lib.ErrorI) {
	// ensure the governance params from the genesis object are valid
	if err = genesis.Params.Check(); err != nil {
		return
	}
	// for each validator, apply basic validations on the required fields
	for _, val := range genesis.Validators {
		// ensure the validator address is the proper length
		if len(val.Address) != crypto.AddressSize {
			return types.ErrAddressSize()
		}
		// ensure the validator public key is the proper length
		if len(val.PublicKey) != crypto.BLS12381PubKeySize {
			return types.ErrPublicKeySize()
		}
		// ensure the validator output address is the proper length
		if len(val.Output) != crypto.AddressSize {
			return types.ErrAddressSize()
		}
	}
	// for each account, apply basic validations on the required fields
	for _, account := range genesis.Accounts {
		// ensure the account address has the proper size
		if len(account.Address) != crypto.AddressSize {
			return types.ErrAddressSize()
		}
	}
	// if the order books aren't nil
	if genesis.OrderBooks != nil {
		// de-duplicate the committee order books
		deDuplicateCommittees := lib.NewDeDuplicator[uint64]()
		// for each order book in the list
		for _, orderBook := range genesis.OrderBooks.OrderBooks {
			// if already found
			if found := deDuplicateCommittees.Found(orderBook.ChainId); found {
				return types.InvalidSellOrder()
			}
			// if the book exists but there's no sell orders
			if len(orderBook.Orders) == 0 {
				return types.InvalidSellOrder()
			}
			// ensure there's no duplicate order-ids within the book
			deDuplicateIds := lib.NewDeDuplicator[uint64]()
			// for each order in the book
			for _, order := range orderBook.Orders {
				// check if order already found
				if found := deDuplicateIds.Found(order.Id); found {
					return types.InvalidSellOrder()
				}
			}
		}
	}
	return
}

// ExportState() creates a GenesisState object from the current state
func (s *StateMachine) ExportState() (genesis *types.GenesisState, err lib.ErrorI) {
	// create a new genesis state object
	genesis = new(types.GenesisState)
	// populate the accounts from the state
	genesis.Accounts, err = s.GetAccounts()
	if err != nil {
		return nil, err
	}
	// populate the pools from the state
	genesis.Pools, err = s.GetPools()
	if err != nil {
		return nil, err
	}
	// populate the validators from the state
	genesis.Validators, err = s.GetValidators()
	if err != nil {
		return nil, err
	}
	// populate the governance params from the state
	genesis.Params, err = s.GetParams()
	if err != nil {
		return nil, err
	}
	// populate the non-signers from the state
	genesis.NonSigners, err = s.GetNonSigners()
	if err != nil {
		return nil, err
	}
	// populate the double-signers from the state
	genesis.DoubleSigners, err = s.GetDoubleSigners()
	if err != nil {
		return nil, err
	}
	// populate the order books from the state
	genesis.OrderBooks, err = s.GetOrderBooks()
	if err != nil {
		return nil, err
	}
	// populate the supply from the state
	genesis.Supply, err = s.GetSupply()
	if err != nil {
		return nil, err
	}
	// populate the retired committees from the state
	genesis.RetiredCommittees, err = s.GetRetiredCommittees()
	if err != nil {
		return nil, err
	}
	// populate the list of committee data from the state
	genesis.Committees, err = s.GetCommitteesData()
	if err != nil {
		return nil, err
	}
	// return the genesis file
	return genesis, nil
}
