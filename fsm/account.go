package fsm

import (
	"github.com/canopy-network/canopy/fsm/types"
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
)

// ACCOUNT CODE BELOW

// GetAccount() returns an Account structure for a specific address
func (s *StateMachine) GetAccount(address crypto.AddressI) (*types.Account, lib.ErrorI) {
	// retrieve the account from the state store
	bz, err := s.Get(types.KeyForAccount(address))
	if err != nil {
		return nil, err
	}
	// convert the account bytes into a structure
	acc, err := s.unmarshalAccount(bz)
	if err != nil {
		return nil, err
	}
	// convert the address into bytes and set it
	acc.Address = address.Bytes()
	return acc, nil
}

// GetAccounts() returns all Account structures in the state
func (s *StateMachine) GetAccounts() (result []*types.Account, err lib.ErrorI) {
	// iterate through the account prefix
	it, err := s.Iterator(types.AccountPrefix())
	if err != nil {
		return nil, err
	}
	defer it.Close()
	// for each item of the iterator
	for ; it.Valid(); it.Next() {
		var acc *types.Account
		acc, err = s.unmarshalAccount(it.Value())
		if err != nil {
			return nil, err
		}
		result = append(result, acc)
	}
	// return the result
	return result, nil
}

// GetAccountsPaginated() returns a page of Account structures in the state
func (s *StateMachine) GetAccountsPaginated(p lib.PageParams) (page *lib.Page, err lib.ErrorI) {
	// create a new 'accounts' page
	page, res := lib.NewPage(p, types.AccountsPageName), make(types.AccountPage, 0)
	// load the page using the account prefix iterator
	err = page.Load(types.AccountPrefix(), false, &res, s.store, func(_, b []byte) (err lib.ErrorI) {
		acc, err := s.unmarshalAccount(b)
		if err == nil {
			res = append(res, acc)
		}
		return
	})
	return
}

// GetAccountBalance() returns the balance of an Account at a specific address
func (s *StateMachine) GetAccountBalance(address crypto.AddressI) (uint64, lib.ErrorI) {
	// get the account from the state
	account, err := s.GetAccount(address)
	if err != nil {
		return 0, err
	}
	// return the amount linked to the account
	return account.Amount, nil
}

// SetAccount() upserts an account into the state
func (s *StateMachine) SetAccount(account *types.Account) lib.ErrorI {
	// convert bytes to the address object
	address := crypto.NewAddressFromBytes(account.Address)
	// if the amount is 0, delete the account from state to prevent unnecessary bloat
	if account.Amount == 0 {
		return s.Delete(types.KeyForAccount(address))
	}
	// convert the account into bytes
	bz, err := s.marshalAccount(account)
	if err != nil {
		return err
	}
	// set the account into state using the 'prefixed' key for the account
	if err = s.Set(types.KeyForAccount(address), bz); err != nil {
		return err
	}
	return nil
}

// SetAccount() upserts multiple accounts into the state
func (s *StateMachine) SetAccounts(accounts []*types.Account, supply *types.Supply) (err lib.ErrorI) {
	// for each account
	for _, acc := range accounts {
		// add the account amount to the supply object
		supply.Total += acc.Amount
		// set the account in state
		if err = s.SetAccount(acc); err != nil {
			return
		}
	}
	return
}

// AccountDeductFees() removes fees from a specific address and adds them to the Canopy reward pool
func (s *StateMachine) AccountDeductFees(address crypto.AddressI, fee uint64) lib.ErrorI {
	// deduct the fee from the account
	if err := s.AccountSub(address, fee); err != nil {
		return err
	}
	// add the fee to the reward pool for the 'self' chain id
	return s.PoolAdd(s.Config.ChainId, fee)
}

// AccountAdd() adds tokens to an Account
func (s *StateMachine) AccountAdd(address crypto.AddressI, amountToAdd uint64) lib.ErrorI {
	// get the account from state
	account, err := s.GetAccount(address)
	if err != nil {
		return err
	}
	// add the tokens to the account structure
	account.Amount += amountToAdd
	// set the account back in state
	return s.SetAccount(account)
}

// AccountSub() removes tokens from an Account
func (s *StateMachine) AccountSub(address crypto.AddressI, amountToSub uint64) lib.ErrorI {
	// get the account from the state
	account, err := s.GetAccount(address)
	if err != nil {
		return err
	}
	// if the account amount is less than the amount to subtract; return insufficient funds
	if account.Amount < amountToSub {
		return types.ErrInsufficientFunds()
	}
	// subtract from the account amount
	account.Amount -= amountToSub
	// set the account in state
	return s.SetAccount(account)
}

// unmarshalAccount() converts bytes into an Account structure
func (s *StateMachine) unmarshalAccount(bz []byte) (*types.Account, lib.ErrorI) {
	// create a new account structure to ensure we never have 'nil' accounts
	acc := new(types.Account)
	// unmarshal the bytes into the account structure
	if err := lib.Unmarshal(bz, acc); err != nil {
		return nil, err
	}
	// return the account
	return acc, nil
}

// marshalAccount() converts an Account structure into bytes
func (s *StateMachine) marshalAccount(account *types.Account) ([]byte, lib.ErrorI) {
	return lib.Marshal(account)
}

// POOL CODE BELOW

/*
	Pools are owner-less designation funds that are 'earmarked' for a purpose
	NOTE: A distinct structure for pools are used instead of a 'hard-coded account address'
	to simply prove that no-one owns the private key for that account
*/

// GetPool() returns a Pool structure for a specific ID
func (s *StateMachine) GetPool(id uint64) (*types.Pool, lib.ErrorI) {
	// get the pool bytes from the state using the Key a specific id
	bz, err := s.Get(types.KeyForPool(id))
	if err != nil {
		return nil, err
	}
	// convert the bytes into a pool structure
	pool, err := s.unmarshalPool(bz)
	if err != nil {
		return nil, err
	}
	// set the pool id from the key
	pool.Id = id
	// return the pool
	return pool, nil
}

// GetPools() returns all Pool structures in the state
func (s *StateMachine) GetPools() (result []*types.Pool, err lib.ErrorI) {
	// get an iterator for the pool group
	it, err := s.Iterator(types.PoolPrefix())
	if err != nil {
		return
	}
	defer it.Close()
	// for each item of the iterator
	for ; it.Valid(); it.Next() {
		var p *types.Pool
		p, err = s.unmarshalPool(it.Value())
		if err != nil {
			return
		}
		// append the pool to the result slice
		result = append(result, p)
	}
	return
}

// GetPoolsPaginated() returns a particular page of Pool structures in the state
func (s *StateMachine) GetPoolsPaginated(p lib.PageParams) (page *lib.Page, err lib.ErrorI) {
	// create a new pool page
	res, page := make(types.PoolPage, 0), lib.NewPage(p, types.PoolPageName)
	// populate the pool page using the pool prefix
	err = page.Load(types.PoolPrefix(), false, &res, s.store, func(_, b []byte) (err lib.ErrorI) {
		acc, err := s.unmarshalPool(b)
		if err == nil {
			res = append(res, acc)
		}
		return
	})
	return
}

// GetPoolBalance() returns the balance of a Pool at an ID
func (s *StateMachine) GetPoolBalance(id uint64) (uint64, lib.ErrorI) {
	// get the pool from state
	pool, err := s.GetPool(id)
	if err != nil {
		return 0, err
	}
	// return the pool amount
	return pool.Amount, nil
}

// SetPool() upserts a Pool structure into the state
func (s *StateMachine) SetPool(pool *types.Pool) (err lib.ErrorI) {
	// if the pool has a 0 balance
	if pool.Amount == 0 {
		return s.Delete(types.KeyForPool(pool.Id))
	}
	// convert the pool to bytes
	bz, err := s.marshalPool(pool)
	if err != nil {
		return
	}
	// set the pool bytes in state using the pool id
	if err = s.Set(types.KeyForPool(pool.Id), bz); err != nil {
		return
	}
	return
}

// SetPools() upserts multiple Pool structures into the state
func (s *StateMachine) SetPools(pools []*types.Pool, supply *types.Supply) (err lib.ErrorI) {
	// for each pool
	for _, pool := range pools {
		// add the pool amount to the total supply
		supply.Total += pool.Amount
		// set the pool in state
		if err = s.SetPool(pool); err != nil {
			return
		}
	}
	return
}

// MintToPool() adds newly created tokens to the Pool structure
func (s *StateMachine) MintToPool(id uint64, amount uint64) lib.ErrorI {
	// track the newly created inflation with the supply structure
	if err := s.AddToTotalSupply(amount); err != nil {
		return err
	}
	// update the pools balance with the new inflation
	return s.PoolAdd(id, amount)
}

// PoolAdd() adds tokens to the Pool structure
func (s *StateMachine) PoolAdd(id uint64, amountToAdd uint64) lib.ErrorI {
	// get the pool from the
	pool, err := s.GetPool(id)
	if err != nil {
		return err
	}
	pool.Amount += amountToAdd
	return s.SetPool(pool)
}

// PoolSub() removes tokens from the Pool structure
func (s *StateMachine) PoolSub(id uint64, amountToSub uint64) lib.ErrorI {
	// get the pool from the state using the 'id'
	pool, err := s.GetPool(id)
	if err != nil {
		return err
	}
	// if the pool amount is less than the subtracted amount; return insufficient funds
	if pool.Amount < amountToSub {
		return types.ErrInsufficientFunds()
	}
	// subtract from the pool balance
	pool.Amount -= amountToSub
	// update the pool in state
	return s.SetPool(pool)
}

// unmarshalPool() coverts bytes into a Pool structure
func (s *StateMachine) unmarshalPool(bz []byte) (*types.Pool, lib.ErrorI) {
	// create a new pool object reference to ensure no 'nil' pools are used
	pool := new(types.Pool)
	// populate the pool object with the bytes
	if err := lib.Unmarshal(bz, pool); err != nil {
		return nil, err
	}
	// return the pool
	return pool, nil
}

// marshalPool() coverts a Pool structure into bytes
func (s *StateMachine) marshalPool(pool *types.Pool) ([]byte, lib.ErrorI) {
	return lib.Marshal(pool)
}

// SUPPLY CODE BELOW

/*
	Supply structure provides an organized view of the overall financial status,
    showing both the total amount available and how it's distributed among various pools and purposes.
*/

// AddToStakedSupply() adds to the staked supply count (staked + delegated)
func (s *StateMachine) AddToStakedSupply(amount uint64) lib.ErrorI {
	// get the supply tracker from the state
	supply, err := s.GetSupply()
	if err != nil {
		return err
	}
	// add to the staked amount in the supply tracker
	supply.Staked += amount
	// set the supply tracker back in state
	return s.SetSupply(supply)
}

// AddToStakedSupply() adds to the staked supply count
func (s *StateMachine) AddToDelegateSupply(amount uint64) lib.ErrorI {
	// get the supply from the state
	supply, err := s.GetSupply()
	if err != nil {
		return err
	}
	// add to the delegation only amount in the supply tracker
	supply.DelegatedOnly += amount
	// set the supply structure back in state
	return s.SetSupply(supply)
}

// AddToTotalSupply() adds to the total supply count
func (s *StateMachine) AddToTotalSupply(amount uint64) lib.ErrorI {
	supply, err := s.GetSupply()
	if err != nil {
		return err
	}
	supply.Total += amount
	return s.SetSupply(supply)
}

// AddToCommitteeStakedSupply() adds to the committee staked supply count
func (s *StateMachine) AddToCommitteeStakedSupply(chainId uint64, amount uint64) lib.ErrorI {
	return s.addToSupplyPool(chainId, amount, types.CommitteesWithDelegations)
}

// AddToDelegateStakedSupply() adds to the delegate staked supply count
func (s *StateMachine) AddToDelegateStakedSupply(chainId uint64, amount uint64) lib.ErrorI {
	return s.addToSupplyPool(chainId, amount, types.DelegationsOnly)
}

// SubFromTotalSupply() removes from the total supply count
func (s *StateMachine) SubFromTotalSupply(amount uint64) lib.ErrorI {
	supply, err := s.GetSupply()
	if err != nil {
		return err
	}
	if supply.Total < amount {
		return types.ErrInsufficientSupply()
	}
	supply.Total -= amount
	return s.SetSupply(supply)
}

// SubFromStakedSupply() removes from the staked supply count (staked + delegated)
func (s *StateMachine) SubFromStakedSupply(amount uint64) lib.ErrorI {
	supply, err := s.GetSupply()
	if err != nil {
		return err
	}
	if supply.Staked < amount {
		return types.ErrInsufficientSupply()
	}
	supply.Staked -= amount
	return s.SetSupply(supply)
}

// SubFromDelegatedSupply() removes from the delegated supply count
func (s *StateMachine) SubFromDelegatedSupply(amount uint64) lib.ErrorI {
	supply, err := s.GetSupply()
	if err != nil {
		return err
	}
	if supply.DelegatedOnly < amount {
		return types.ErrInsufficientSupply()
	}
	supply.DelegatedOnly -= amount
	return s.SetSupply(supply)
}

// SubFromCommitteeStakedSupply() removes from the committee staked supply count
func (s *StateMachine) SubFromCommitteeStakedSupply(chainId uint64, amount uint64) lib.ErrorI {
	return s.subFromSupplyPool(chainId, amount, types.CommitteesWithDelegations)
}

// SubFromDelegateStakedSupply() removes from the delegate committee staked supply count
func (s *StateMachine) SubFromDelegateStakedSupply(chainId uint64, amount uint64) lib.ErrorI {
	return s.subFromSupplyPool(chainId, amount, types.DelegationsOnly)
}

// GetCommitteeStakedSupply() retrieves the committee staked supply count
func (s *StateMachine) GetCommitteeStakedSupply(chainId uint64) (p *types.Pool, err lib.ErrorI) {
	return s.getSupplyPool(chainId, types.CommitteesWithDelegations)
}

// GetFromDelegateStakedSupply() retrieves the delegate committee staked supply count
func (s *StateMachine) GetDelegateStakedSupply(chainId uint64) (p *types.Pool, err lib.ErrorI) {
	return s.getSupplyPool(chainId, types.DelegationsOnly)
}

// GetSupply() returns the Supply structure held in the state
func (s *StateMachine) GetSupply() (*types.Supply, lib.ErrorI) {
	bz, err := s.Get(types.SupplyPrefix())
	if err != nil {
		return nil, err
	}
	return s.unmarshalSupply(bz)
}

// SetSupply() upserts the Supply structure into the state
func (s *StateMachine) SetSupply(supply *types.Supply) lib.ErrorI {
	bz, err := s.marshalSupply(supply)
	if err != nil {
		return err
	}
	if err = s.Set(types.SupplyPrefix(), bz); err != nil {
		return err
	}
	return nil
}

// unmarshalSupply() converts bytes into the supply
func (s *StateMachine) unmarshalSupply(bz []byte) (*types.Supply, lib.ErrorI) {
	supply := new(types.Supply)
	if err := lib.Unmarshal(bz, supply); err != nil {
		return nil, err
	}
	return supply, nil
}

// marshalSupply() converts the Supply into bytes
func (s *StateMachine) marshalSupply(supply *types.Supply) ([]byte, lib.ErrorI) {
	return lib.Marshal(supply)
}

// addToSupplyPool() adds to a supply pool using an addition callback with 'executeOnSupplyPool'
func (s *StateMachine) addToSupplyPool(chainId, amount uint64, targetType types.SupplyPoolType) lib.ErrorI {
	return s.executeOnSupplyPool(chainId, targetType, func(s *types.Supply, p *types.Pool) (err lib.ErrorI) {
		p.Amount += amount
		return
	})
}

// subFromSupplyPool() subtracts from a supply pool using a subtraction callback with 'executeOnSupplyPool'
func (s *StateMachine) subFromSupplyPool(chainId, amount uint64, targetType types.SupplyPoolType) lib.ErrorI {
	return s.executeOnSupplyPool(chainId, targetType, func(s *types.Supply, p *types.Pool) (err lib.ErrorI) {
		if p == nil || p.Amount < amount {
			return types.ErrInsufficientSupply()
		}
		p.Amount -= amount
		return
	})
}

// getSupplyPool() returns the supply pool based on the target type
func (s *StateMachine) getSupplyPool(chainId uint64, targetType types.SupplyPoolType) (p *types.Pool, err lib.ErrorI) {
	arr, _, err := s.getSupplyPools(targetType)
	if err != nil {
		return
	}
	p = s.findOrCreateSupplyPool(arr, chainId)
	return
}

// getSupplyPools retrieves a particular pool based on the target type
func (s *StateMachine) getSupplyPools(targetType types.SupplyPoolType) (arr *[]*types.Pool, supply *types.Supply, err lib.ErrorI) {
	supply, err = s.GetSupply()
	if err != nil {
		return
	}
	// determine the type of the target
	switch targetType {
	case types.CommitteesWithDelegations:
		arr = &supply.CommitteeStaked
	case types.DelegationsOnly:
		arr = &supply.CommitteeDelegatedOnly
	}
	return
}

// executeOnSupplyPool() finds a target pool using the target type and chainId and executes a callback on it
func (s *StateMachine) executeOnSupplyPool(chainId uint64, targetType types.SupplyPoolType, callback func(s *types.Supply, p *types.Pool) lib.ErrorI) lib.ErrorI {
	arr, supply, err := s.getSupplyPools(targetType)
	if err != nil {
		return err
	}
	// locate the target pool
	targetPool := s.findOrCreateSupplyPool(arr, chainId)
	// execute the business logic callback
	if err = callback(supply, targetPool); err != nil {
		return err
	}
	// filter zeroes and sort the pool
	// this prevents dead committees from bloating the supply structure
	types.FilterAndSortPool(arr)
	// finally set the supply
	return s.SetSupply(supply)
}

// findOrCreateSupplyPool() searches for a pool by chainId or creates a new one if not found
func (s *StateMachine) findOrCreateSupplyPool(poolArr *[]*types.Pool, chainId uint64) (pool *types.Pool) {
	// iterate through the list looking for the supply pool
	for _, pool = range *poolArr {
		if pool.Id == chainId {
			return
		}
	}
	// if pool not found, add it to the list
	pool = &types.Pool{Id: chainId}
	*poolArr = append(*poolArr, pool)
	return
}
