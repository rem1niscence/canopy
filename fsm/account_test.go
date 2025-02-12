package fsm

import (
	"bytes"
	"sort"
	"testing"

	"github.com/canopy-network/canopy/fsm/types"
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
	"github.com/stretchr/testify/require"
)

func TestSetGetAccount(t *testing.T) {
	tests := []struct {
		name     string
		detail   string
		accounts []*types.Account
	}{
		{
			name:     "zero / empty account",
			detail:   "test getting an account that doesn't exist; should return a non nil account with a zero balance",
			accounts: nil,
		},
		{
			name:   "single account",
			detail: "test setting and getting an account",
			accounts: []*types.Account{{
				Address: newTestAddress(t).Bytes(),
				Amount:  100,
			}, {
				Address: newTestAddress(t).Bytes(),
				Amount:  101,
			}},
		},
		{
			name:   "multi-accounts",
			detail: "test setting and getting multiple accounts",
			accounts: []*types.Account{{
				Address: newTestAddress(t).Bytes(),
				Amount:  100,
			}, {
				Address: newTestAddress(t, 1).Bytes(),
				Amount:  100,
			}, {
				Address: newTestAddress(t, 2).Bytes(),
				Amount:  0,
			}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a test state machine
			sm := newTestStateMachine(t)
			// special case to test getting an account that doesn't exist
			// should return a non-nil account with a zero balance
			if test.accounts == nil {
				got, err := sm.GetAccount(newTestAddress(t))
				require.NoError(t, err)
				require.Equal(t, newTestAddress(t).Bytes(), got.Address)
				require.Zero(t, got.Amount)
				return
			}
			// needed vars to ensure non zero are not returned later
			lenNonZero := 0
			accsMap := make(map[string]bool, len(test.accounts))
			// test setting and getting accounts
			for _, acc := range test.accounts {
				ok := accsMap[crypto.NewAddress(acc.Address).String()]
				if !ok {
					accsMap[crypto.NewAddress(acc.Address).String()] = true
					if acc.Amount != 0 {
						lenNonZero++
					}
				}
				// ensure no error on setting the account
				require.NoError(t, sm.SetAccount(acc))
				// ensure expected
				got, err := sm.GetAccount(crypto.NewAddress(acc.Address))
				require.NoError(t, err)
				require.EqualExportedValues(t, *got, *acc)
				// test 'GetAccountBalance' as well
				balance, err := sm.GetAccountBalance(crypto.NewAddress(acc.Address))
				require.NoError(t, err)
				require.Equal(t, acc.Amount, balance)
			}
			// ensure amoun 0 accounts are not returned on GetAccounts()
			accs, err := sm.GetAccounts()
			require.NoError(t, err)
			require.Equal(t, lenNonZero, len(accs))
		})
	}
}

func TestGetSetAccounts(t *testing.T) {
	tests := []struct {
		name     string
		detail   string
		accounts []*types.Account
	}{
		{
			name:     "no accounts",
			detail:   "test when there exists no accounts in the state",
			accounts: nil,
		},
		{
			name:   "multi-accounts",
			detail: "test with multiple accounts",
			accounts: []*types.Account{{
				Address: newTestAddress(t).Bytes(),
				Amount:  100,
			}, {
				Address: newTestAddress(t, 1).Bytes(),
				Amount:  100,
			}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a test state machine
			sm := newTestStateMachine(t)
			// setup supply tracker and expected total tokens
			supply, expectedTokens := &types.Supply{}, uint64(0)
			for _, acc := range test.accounts {
				expectedTokens += acc.Amount
			}
			// test setting all accounts with a supply tracker (used in genesis)
			require.NoError(t, sm.SetAccounts(test.accounts, supply))
			require.Equal(t, supply.Total, expectedTokens)
			// sort the slice lexicographically as that's the deterministic ordering that is expected by the protocol
			sort.Slice(test.accounts, func(i, j int) bool {
				return bytes.Compare(test.accounts[i].Address, test.accounts[j].Address) == -1
			})
			// ensure no error on function call
			got, err := sm.GetAccounts()
			require.NoError(t, err)
			// ensure results equal expected
			for i, acc := range got {
				require.EqualExportedValues(t, acc, test.accounts[i])
			}
		})
	}
}

func TestGetAccountsPaginated(t *testing.T) {
	tests := []struct {
		name       string
		detail     string
		accounts   []*types.Account
		pageParams lib.PageParams
	}{
		{
			name:     "no accounts",
			detail:   "test when there exists no accounts in the state",
			accounts: nil,
			pageParams: lib.PageParams{
				PageNumber: 1,
				PerPage:    100,
			},
		},
		{
			name:   "multi-accounts",
			detail: "test with multiple accounts and default page params",
			accounts: []*types.Account{{
				Address: newTestAddress(t).Bytes(),
				Amount:  100,
			}, {
				Address: newTestAddress(t, 1).Bytes(),
				Amount:  100,
			}},
			pageParams: lib.PageParams{
				PageNumber: 1,
				PerPage:    100,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a test state machine
			sm := newTestStateMachine(t)
			// set all accounts
			for _, acc := range test.accounts {
				require.NoError(t, sm.SetAccount(acc))
			}
			// sort the slice lexicographically as that's the deterministic ordering that is expected by the protocol
			sort.Slice(test.accounts, func(i, j int) bool {
				return bytes.Compare(test.accounts[i].Address, test.accounts[j].Address) == -1
			})
			// ensure no error on function call
			got, err := sm.GetAccountsPaginated(test.pageParams)
			require.NoError(t, err)
			// ensure results type string
			require.Equal(t, got.Type, types.AccountsPageName)
			// ensure total count
			require.Equal(t, got.TotalCount, len(test.accounts))
			// ensure expected per page NOTE: if setting below minimum page params, it will change results
			require.Equal(t, got.PerPage, test.pageParams.PerPage)
			// ensure expected page number NOTE: if setting below minimum page params, it will change results
			require.Equal(t, got.PageNumber, test.pageParams.PageNumber)
			// ensure expected results type
			accountsPage, ok := got.Results.(*types.AccountPage)
			require.True(t, ok)
			require.NotNil(t, accountsPage)
			// ensure expected results
			for i, acc := range *accountsPage {
				require.EqualExportedValues(t, acc, test.accounts[i])
			}
		})
	}
}

func TestAccountDeductFees(t *testing.T) {
	tests := []struct {
		name    string
		detail  string
		account *types.Account
		pool    *types.Pool
		error   bool
	}{
		{
			name:   "empty account",
			detail: "try (and fail) deducting from an empty account",
			account: &types.Account{
				Address: newTestAddress(t).Bytes(),
			},
			pool: &types.Pool{
				Id: lib.CanopyChainId,
			},
			error: true,
		},
		{
			name:   "insufficient account balance",
			detail: "try (and fail) deducting from an insufficient account balance",
			account: &types.Account{
				Address: newTestAddress(t).Bytes(),
				Amount:  100,
			},
			pool: &types.Pool{
				Id: lib.CanopyChainId,
			},
			error: true,
		},
		{
			name:   "just enough account balance",
			detail: "deduct from a account that has just enough balance",
			account: &types.Account{
				Address: newTestAddress(t).Bytes(),
				Amount:  10000,
			},
			pool: &types.Pool{
				Id: lib.CanopyChainId,
			},
		},
		{
			name:   "non empty fee pool",
			detail: "add to a pool that has some balance already",
			account: &types.Account{
				Address: newTestAddress(t).Bytes(),
				Amount:  100000,
			},
			pool: &types.Pool{
				Id:     lib.CanopyChainId,
				Amount: 100,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fee := uint64(10000)
			// create a test state machine
			sm := newTestStateMachine(t)
			// setup account address object
			address := crypto.NewAddress(test.account.Address)
			// set the account
			require.NoError(t, sm.SetAccount(test.account))
			// set the fee pool
			require.NoError(t, sm.SetPool(test.pool))
			// try to deduct a fee from the account
			err := sm.AccountDeductFees(address, fee)
			require.Equal(t, err != nil, test.error)
			if err != nil {
				return
			}
			// check account balance
			balance, err := sm.GetAccountBalance(address)
			require.NoError(t, err)
			require.Equal(t, test.account.Amount-fee, balance)
			// check pool balance
			balance, err = sm.GetPoolBalance(lib.CanopyChainId)
			require.NoError(t, err)
			require.Equal(t, test.pool.Amount+fee, balance)
		})
	}
}

func TestMintToAccount(t *testing.T) {
	tests := []struct {
		name           string
		detail         string
		account        *types.Account
		startingSupply uint64
		amount         uint64
	}{
		{
			name:   "empty account",
			detail: "mint to an empty account",
			amount: 100,
		},
		{
			name:   "non empty account",
			detail: "mint to a non-empty account",
			account: &types.Account{
				Address: newTestAddress(t).Bytes(),
				Amount:  1000,
			},
			amount: 100,
		},
		{
			name:   "non empty supply",
			detail: "mint with a non-zero starting supply",
			account: &types.Account{
				Address: newTestAddress(t).Bytes(),
				Amount:  1000,
			},
			startingSupply: 1000,
			amount:         100,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a test state machine
			sm := newTestStateMachine(t)
			// if non-empty, preset the account
			if test.account != nil {
				require.NoError(t, sm.SetAccount(test.account))
			}
			// setup starting supply
			require.NoError(t, sm.AddToTotalSupply(test.amount))
			// retrieve the account to be minted to
			acc, err := sm.GetAccount(newTestAddress(t))
			require.NoError(t, err)
			// retrieve the supply before minting
			sup, err := sm.GetSupply()
			// ensure no error on function call
			require.NoError(t, sm.MintToAccount(newTestAddress(t), test.amount))
			// retrieve the account after being minted to
			accAfter, err := sm.GetAccount(newTestAddress(t))
			require.NoError(t, err)
			// retrieve the supply after mint
			supAfter, err := sm.GetSupply()
			require.NoError(t, err)
			// ensure the difference of the account is expected
			require.Equal(t, test.amount, accAfter.Amount-acc.Amount)
			// ensure the difference of the supply is expected
			require.Equal(t, test.amount, supAfter.Total-sup.Total)
		})
	}
}

func TestAccountAdd(t *testing.T) {
	tests := []struct {
		name    string
		detail  string
		account *types.Account
		amount  uint64
	}{
		{
			name:   "empty account",
			detail: "add to an empty account",
			amount: 100,
		},
		{
			name:   "non empty account",
			detail: "add to a non-empty account",
			account: &types.Account{
				Address: newTestAddress(t).Bytes(),
				Amount:  1000,
			},
			amount: 100,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a test state machine
			sm := newTestStateMachine(t)
			// setup test address
			testAddr := newTestAddress(t)
			// if non-empty, preset the account
			if test.account != nil {
				require.NoError(t, sm.SetAccount(test.account))
			}
			// retrieve the account to be added to
			acc, err := sm.GetAccount(testAddr)
			require.NoError(t, err)
			// ensure no error on function call
			require.NoError(t, sm.AccountAdd(testAddr, test.amount))
			// retrieve the account after being minted to
			accAfter, err := sm.GetAccount(testAddr)
			require.NoError(t, err)
			// ensure the difference of the account is expected
			require.Equal(t, test.amount, accAfter.Amount-acc.Amount)
		})
	}
}

func TestAccountSub(t *testing.T) {
	tests := []struct {
		name    string
		detail  string
		account *types.Account
		amount  uint64
		error   bool
	}{
		{
			name:   "empty account",
			detail: "try (and fail) to subtract from an empty account",
			amount: 100,
			error:  true,
		},
		{
			name:   "insufficient account balance",
			detail: "try (and fail) to subtract from an insufficient account balance",
			account: &types.Account{
				Address: newTestAddress(t).Bytes(),
				Amount:  99,
			},
			amount: 100,
			error:  true,
		},
		{
			name:   "non empty account",
			detail: "subtract from a non-empty account",
			account: &types.Account{
				Address: newTestAddress(t).Bytes(),
				Amount:  1000,
			},
			amount: 100,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a test state machine
			sm := newTestStateMachine(t)
			// setup test address
			testAddr := newTestAddress(t)
			// if non-empty, preset the account
			if test.account != nil {
				require.NoError(t, sm.SetAccount(test.account))
			}
			// retrieve the account to be added to
			acc, err := sm.GetAccount(testAddr)
			require.NoError(t, err)
			// ensure no error on function call
			err = sm.AccountSub(testAddr, test.amount)
			require.Equal(t, test.error, err != nil)
			if err != nil {
				return
			}
			// retrieve the account after being minted to
			accAfter, err := sm.GetAccount(testAddr)
			require.NoError(t, err)
			// ensure the difference of the account is expected
			require.Equal(t, acc.Amount, accAfter.Amount+test.amount)
		})
	}
}

func TestSetGetPool(t *testing.T) {
	tests := []struct {
		name   string
		detail string
		pools  []*types.Pool
	}{
		{
			name:   "zero / empty pools",
			detail: "test getting an pool that doesn't exist; should return a non nil pool with a zero balance",
			pools:  nil,
		},
		{
			name:   "single pool",
			detail: "test setting and getting a pool",
			pools: []*types.Pool{{
				Id:     lib.CanopyChainId,
				Amount: 100,
			}, {
				Id:     lib.CanopyChainId,
				Amount: 101,
			}},
		},
		{
			name:   "multi-pool",
			detail: "test setting and getting multiple pools",
			pools: []*types.Pool{{
				Id:     lib.CanopyChainId,
				Amount: 100,
			}, {
				Id:     lib.CanopyChainId + 1,
				Amount: 100,
			}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a test state machine
			sm := newTestStateMachine(t)
			// special case to test getting a pool that doesn't exist
			// should return a non-nil pool with a zero balance
			if test.pools == nil {
				got, err := sm.GetPool(lib.CanopyChainId)
				require.NoError(t, err)
				require.Equal(t, lib.CanopyChainId, got.Id)
				require.Zero(t, got.Amount)
				return
			}
			// test setting and getting pools
			for _, p := range test.pools {
				// ensure no error on setting the pool
				require.NoError(t, sm.SetPool(p))
				// ensure expected
				got, err := sm.GetPool(p.Id)
				require.NoError(t, err)
				require.EqualExportedValues(t, *got, *p)
				// test 'GetPoolBalance' as well
				balance, err := sm.GetPoolBalance(p.Id)
				require.NoError(t, err)
				require.Equal(t, p.Amount, balance)
			}
		})
	}
}

func TestGetSetPools(t *testing.T) {
	tests := []struct {
		name   string
		detail string
		pools  []*types.Pool
	}{
		{
			name:   "no pools",
			detail: "test when there exists no pools in the state",
			pools:  nil,
		},
		{
			name:   "multi-pool",
			detail: "test with multiple pool",
			pools: []*types.Pool{{
				Id:     lib.CanopyChainId,
				Amount: 100,
			}, {
				Id:     lib.CanopyChainId + 1,
				Amount: 100,
			}, {
				Id:     lib.CanopyChainId + 2,
				Amount: 0,
			}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a test state machine
			sm := newTestStateMachine(t)
			// setup supply tracker and expected total tokens
			supply, expectedTokens := &types.Supply{}, uint64(0)
			for _, p := range test.pools {
				expectedTokens += p.Amount
			}
			// test setting all pools with a supply tracker (used in genesis)
			require.NoError(t, sm.SetPools(test.pools, supply))
			require.Equal(t, supply.Total, expectedTokens)
			// sort the slice numerically as that's the deterministic ordering that is expected by the protocol
			sort.Slice(test.pools, func(i, j int) bool {
				return test.pools[i].Id < test.pools[j].Id
			})
			// ensure no error on function call
			got, err := sm.GetPools()
			require.NoError(t, err)
			// ensure 0 amount pools are not returned
			var noZeroAccounts int
			for _, pool := range test.pools {
				if pool.Amount != 0 {
					noZeroAccounts++
				}
			}
			require.Equal(t, len(got), noZeroAccounts)
			// ensure results equal expected
			for i, pool := range got {
				require.EqualExportedValues(t, pool, test.pools[i])
			}
		})
	}
}

func TestGetPoolsPaginated(t *testing.T) {
	tests := []struct {
		name       string
		detail     string
		pools      []*types.Pool
		pageParams lib.PageParams
	}{
		{
			name:   "no pools",
			detail: "test when there exists no pools in the state",
			pools:  nil,
			pageParams: lib.PageParams{
				PageNumber: 1,
				PerPage:    100,
			},
		},
		{
			name:   "multi-pool",
			detail: "test with multiple pools and default page params",
			pools: []*types.Pool{{
				Id:     lib.CanopyChainId,
				Amount: 100,
			}, {
				Id:     lib.CanopyChainId + 1,
				Amount: 100,
			}},
			pageParams: lib.PageParams{
				PageNumber: 1,
				PerPage:    100,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a test state machine
			sm := newTestStateMachine(t)
			// set all pools
			for _, p := range test.pools {
				require.NoError(t, sm.SetPool(p))
			}
			// sort the slice numerically as that's the deterministic ordering that is expected by the protocol
			sort.Slice(test.pools, func(i, j int) bool {
				return test.pools[i].Id < test.pools[j].Id
			})
			// ensure no error on function call
			got, err := sm.GetPoolsPaginated(test.pageParams)
			require.NoError(t, err)
			// ensure results type string
			require.Equal(t, got.Type, types.PoolPageName)
			// ensure total count
			require.Equal(t, got.TotalCount, len(test.pools))
			// ensure expected per page NOTE: if setting below minimum page params, it will change results
			require.Equal(t, got.PerPage, test.pageParams.PerPage)
			// ensure expected page number NOTE: if setting below minimum page params, it will change results
			require.Equal(t, got.PageNumber, test.pageParams.PageNumber)
			// ensure expected results type
			poolsPage, ok := got.Results.(*types.PoolPage)
			require.True(t, ok)
			require.NotNil(t, poolsPage)
			// ensure expected results
			for i, p := range *poolsPage {
				require.EqualExportedValues(t, p, test.pools[i])
			}
		})
	}
}

func TestMintToPool(t *testing.T) {
	tests := []struct {
		name           string
		detail         string
		pool           *types.Pool
		startingSupply uint64
		amount         uint64
	}{
		{
			name:   "empty pool",
			detail: "mint to an empty pool",
			amount: 100,
		},
		{
			name:   "non empty pool",
			detail: "mint to a non-empty pool",
			pool: &types.Pool{
				Id:     lib.CanopyChainId,
				Amount: 1000,
			},
			amount: 100,
		},
		{
			name:   "non empty supply",
			detail: "mint with a non-zero starting supply",
			pool: &types.Pool{
				Id:     lib.CanopyChainId,
				Amount: 1000,
			},
			startingSupply: 1000,
			amount:         100,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a test state machine
			sm := newTestStateMachine(t)
			// if non-empty, preset the pool
			if test.pool != nil {
				require.NoError(t, sm.SetPool(test.pool))
			}
			// setup starting supply
			require.NoError(t, sm.AddToTotalSupply(test.amount))
			// retrieve the pool to be minted to
			pool, err := sm.GetPool(lib.CanopyChainId)
			require.NoError(t, err)
			// retrieve the supply before minting
			sup, err := sm.GetSupply()
			// ensure no error on function call
			require.NoError(t, sm.MintToPool(lib.CanopyChainId, test.amount))
			// retrieve the pool after being minted to
			poolAfter, err := sm.GetPool(lib.CanopyChainId)
			require.NoError(t, err)
			// retrieve the supply after mint
			supAfter, err := sm.GetSupply()
			require.NoError(t, err)
			// ensure the difference of the pool is expected
			require.Equal(t, test.amount, poolAfter.Amount-pool.Amount)
			// ensure the difference of the supply is expected
			require.Equal(t, test.amount, supAfter.Total-sup.Total)
		})
	}
}

func TestPoolAdd(t *testing.T) {
	tests := []struct {
		name   string
		detail string
		pool   *types.Pool
		amount uint64
	}{
		{
			name:   "empty pool",
			detail: "add to an empty pool",
			amount: 100,
		},
		{
			name:   "non empty pool",
			detail: "add to a non-empty pool",
			pool: &types.Pool{
				Id:     lib.CanopyChainId,
				Amount: 1000,
			},
			amount: 100,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a test state machine
			sm := newTestStateMachine(t)
			// if non-empty, preset the pool
			if test.pool != nil {
				require.NoError(t, sm.SetPool(test.pool))
			}
			// retrieve the pool to be added to
			pool, err := sm.GetPool(lib.CanopyChainId)
			require.NoError(t, err)
			// ensure no error on function call
			require.NoError(t, sm.PoolAdd(lib.CanopyChainId, test.amount))
			// retrieve the pool after being minted to
			poolAfter, err := sm.GetPool(lib.CanopyChainId)
			require.NoError(t, err)
			// ensure the difference of the pool is expected
			require.Equal(t, test.amount, poolAfter.Amount-pool.Amount)
		})
	}
}

func TestPoolSub(t *testing.T) {
	tests := []struct {
		name   string
		detail string
		pool   *types.Pool
		amount uint64
		error  bool
	}{
		{
			name:   "empty pool",
			detail: "try (and fail) to subtract from an empty pool",
			amount: 100,
			error:  true,
		},
		{
			name:   "insufficient pool balance",
			detail: "try (and fail) to subtract from an insufficient pool balance",
			pool: &types.Pool{
				Id:     lib.CanopyChainId,
				Amount: 99,
			},
			amount: 100,
			error:  true,
		},
		{
			name:   "non empty pool",
			detail: "subtract from a non-empty pool",
			pool: &types.Pool{
				Id:     lib.CanopyChainId,
				Amount: 1000,
			},
			amount: 100,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a test state machine
			sm := newTestStateMachine(t)
			// if non-empty, preset the pool
			if test.pool != nil {
				require.NoError(t, sm.SetPool(test.pool))
			}
			// retrieve the pool to be added to
			pool, err := sm.GetPool(lib.CanopyChainId)
			require.NoError(t, err)
			// ensure no error on function call
			err = sm.PoolSub(lib.CanopyChainId, test.amount)
			require.Equal(t, test.error, err != nil)
			if err != nil {
				return
			}
			// retrieve the account after being minted to
			poolAfter, err := sm.GetPool(lib.CanopyChainId)
			require.NoError(t, err)
			// ensure the difference of the account is expected
			require.Equal(t, pool.Amount, poolAfter.Amount+test.amount)
		})
	}
}

func TestAddToStakedSupply(t *testing.T) {
	tests := []struct {
		name      string
		detail    string
		amount    uint64
		preAmount uint64
	}{
		{
			name:   "without preexisting staked supply",
			detail: "starting with 0 staked supply",
			amount: 100,
		},
		{
			name:      "with preexisting staked supply",
			detail:    "starting with some staked supply",
			amount:    100,
			preAmount: 100,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a test state machine
			sm := newTestStateMachine(t)
			// retrieve supply before addition
			supply, err := sm.GetSupply()
			require.NoError(t, err)
			// add pre-amount if exists
			if test.preAmount != 0 {
				supply.Staked = test.preAmount
				require.NoError(t, sm.SetSupply(supply))
			}
			// ensure no error on function call
			require.NoError(t, sm.AddToStakedSupply(test.amount))
			// retrieve the supply
			afterSupply, err := sm.GetSupply()
			require.NoError(t, err)
			// validate the expected supply
			require.Equal(t, afterSupply.Staked, supply.Staked+test.amount)
		})
	}
}

func TestAddToTotalSupply(t *testing.T) {
	tests := []struct {
		name      string
		detail    string
		amount    uint64
		preAmount uint64
	}{
		{
			name:   "without preexisting supply",
			detail: "starting with 0 supply",
			amount: 100,
		},
		{
			name:      "with preexisting supply",
			detail:    "starting with some supply",
			amount:    100,
			preAmount: 100,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a test state machine
			sm := newTestStateMachine(t)
			// retrieve supply before addition
			supply, err := sm.GetSupply()
			require.NoError(t, err)
			// add pre-amount if exists
			if test.preAmount != 0 {
				supply.Total = test.preAmount
				require.NoError(t, sm.SetSupply(supply))
			}
			// ensure no error on function call
			require.NoError(t, sm.AddToTotalSupply(test.amount))
			// retrieve the supply
			afterSupply, err := sm.GetSupply()
			require.NoError(t, err)
			// validate the expected supply
			require.Equal(t, afterSupply.Total, supply.Total+test.amount)
		})
	}
}

func TestAddToCommitteeStakedSupply(t *testing.T) {
	tests := []struct {
		name      string
		detail    string
		amount    uint64
		preAmount uint64
		id        uint64
	}{
		{
			name:   "without preexisting supply",
			detail: "starting with 0 supply",
			amount: 100,
		},
		{
			name:      "with preexisting supply",
			detail:    "starting with some supply",
			amount:    100,
			preAmount: 100,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a test state machine
			sm := newTestStateMachine(t)
			// add pre-amount if exists
			if test.preAmount != 0 {
				// retrieve supply before addition
				supply, err := sm.GetSupply()
				require.NoError(t, err)
				supply.CommitteeStaked = append(supply.CommitteeStaked, &types.Pool{
					Id:     test.id,
					Amount: test.preAmount,
				})
				require.NoError(t, sm.SetSupply(supply))
			}
			// ensure no error on function call
			require.NoError(t, sm.AddToCommitteeStakedSupply(test.id, test.amount))
			// retrieve the supply
			supply, err := sm.GetSupply()
			require.NoError(t, err)
			// validate the expected supply
			require.Equal(t, supply.CommitteeStaked[0].Amount, test.preAmount+test.amount)
		})
	}
}

func TestAddToDelegationStakedSupply(t *testing.T) {
	tests := []struct {
		name      string
		detail    string
		amount    uint64
		preAmount uint64
		id        uint64
	}{
		{
			name:   "without preexisting supply",
			detail: "starting with 0 supply",
			amount: 100,
		},
		{
			name:      "with preexisting supply",
			detail:    "starting with some supply",
			amount:    100,
			preAmount: 100,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a test state machine
			sm := newTestStateMachine(t)
			// add pre-amount if exists
			if test.preAmount != 0 {
				// retrieve supply before addition
				supply, err := sm.GetSupply()
				require.NoError(t, err)
				supply.CommitteeDelegatedOnly = append(supply.CommitteeDelegatedOnly, &types.Pool{
					Id:     test.id,
					Amount: test.preAmount,
				})
				require.NoError(t, sm.SetSupply(supply))
			}
			// ensure no error on function call
			require.NoError(t, sm.AddToDelegateStakedSupply(test.id, test.amount))
			// retrieve the supply
			supply, err := sm.GetSupply()
			require.NoError(t, err)
			// validate the expected supply
			require.Equal(t, supply.CommitteeDelegatedOnly[0].Amount, test.preAmount+test.amount)
		})
	}
}

func TestSubFromTotalSupply(t *testing.T) {
	tests := []struct {
		name      string
		detail    string
		amount    uint64
		preAmount uint64
		error     bool
	}{
		{
			name:   "without preexisting supply",
			detail: "starting with 0 supply",
			amount: 100,
			error:  true,
		},
		{
			name:      "0 after subtraction",
			detail:    "starting with exactly the right starting supply",
			amount:    100,
			preAmount: 100,
		},
		{
			name:      "leftover after subtraction",
			detail:    "starting with exactly the right starting supply",
			amount:    100,
			preAmount: 100,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a test state machine
			sm := newTestStateMachine(t)
			// retrieve supply before addition
			supply, err := sm.GetSupply()
			require.NoError(t, err)
			// add pre-amount if exists
			if test.preAmount != 0 {
				supply.Total = test.preAmount
				require.NoError(t, sm.SetSupply(supply))
			}
			// check error on function call
			err = sm.SubFromTotalSupply(test.amount)
			require.Equal(t, test.error, err != nil)
			if err != nil {
				return
			}
			// retrieve the supply
			afterSupply, err := sm.GetSupply()
			require.NoError(t, err)
			// validate the expected supply
			require.Equal(t, afterSupply.Total, supply.Total-test.amount)
		})
	}
}

func TestSubFromCommitteeStakedSupply(t *testing.T) {
	tests := []struct {
		name      string
		detail    string
		amount    uint64
		preAmount uint64
		id        uint64
		zeroAfter bool
		error     bool
	}{
		{
			name:   "without preexisting supply",
			detail: "starting with 0 supply",
			amount: 100,
			error:  true,
		},
		{
			name:      "0 after subtraction",
			detail:    "starting with exactly the right starting supply",
			amount:    100,
			preAmount: 100,
			zeroAfter: true,
		},
		{
			name:      "leftover after subtraction",
			detail:    "starting with exactly the right starting supply",
			amount:    99,
			preAmount: 100,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a test state machine
			sm := newTestStateMachine(t)
			// add pre-amount if exists
			if test.preAmount != 0 {
				// retrieve supply before addition
				supply, err := sm.GetSupply()
				require.NoError(t, err)
				supply.CommitteeStaked = append(supply.CommitteeStaked, &types.Pool{
					Id:     test.id,
					Amount: test.preAmount,
				})
				require.NoError(t, sm.SetSupply(supply))
			}
			// check error on function call
			err := sm.SubFromCommitteeStakedSupply(test.id, test.amount)
			require.Equal(t, test.error, err != nil, err)
			if err != nil {
				return
			}
			// retrieve the supply
			supply, err := sm.GetSupply()
			require.NoError(t, err)
			// if zero result expected, the filtering function should have removed it from the list
			if test.zeroAfter {
				require.Zero(t, len(supply.CommitteeStaked))
				return
			}
			// validate the expected supply
			require.Equal(t, supply.CommitteeStaked[0].Amount, test.preAmount-test.amount)
		})
	}
}

func TestSubFromDelegationStakedSupply(t *testing.T) {
	tests := []struct {
		name      string
		detail    string
		amount    uint64
		preAmount uint64
		id        uint64
		zeroAfter bool
		error     bool
	}{
		{
			name:   "without preexisting supply",
			detail: "starting with 0 supply",
			amount: 100,
			error:  true,
		},
		{
			name:      "0 after subtraction",
			detail:    "starting with exactly the right starting supply",
			amount:    100,
			preAmount: 100,
			zeroAfter: true,
		},
		{
			name:      "leftover after subtraction",
			detail:    "starting with exactly the right starting supply",
			amount:    99,
			preAmount: 100,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a test state machine
			sm := newTestStateMachine(t)
			// add pre-amount if exists
			if test.preAmount != 0 {
				// retrieve supply before addition
				supply, err := sm.GetSupply()
				require.NoError(t, err)
				supply.CommitteeDelegatedOnly = append(supply.CommitteeDelegatedOnly, &types.Pool{
					Id:     test.id,
					Amount: test.preAmount,
				})
				require.NoError(t, sm.SetSupply(supply))
			}
			// check error on function call
			err := sm.SubFromDelegateStakedSupply(test.id, test.amount)
			require.Equal(t, test.error, err != nil)
			if err != nil {
				return
			}
			// retrieve the supply
			supply, err := sm.GetSupply()
			require.NoError(t, err)
			// if zero result expected, the filtering function should have removed it from the list
			if test.zeroAfter {
				require.Zero(t, len(supply.CommitteeDelegatedOnly))
				return
			}
			// validate the expected supply
			require.Equal(t, supply.CommitteeDelegatedOnly[0].Amount, test.preAmount-test.amount)
		})
	}
}
