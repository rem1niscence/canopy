package fsm

import (
	"bytes"
	"github.com/canopy-network/canopy/fsm/types"
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
	"github.com/stretchr/testify/require"
	"math"
	"slices"
	"testing"
)

func TestGetValidator(t *testing.T) {
	tests := []struct {
		name   string
		detail string
		preset *types.Validator
		tryGet crypto.AddressI
		error  string
	}{
		{
			name:   "no preset",
			detail: "no validator was not preset so not exists",
			tryGet: newTestAddress(t),
			error:  "validator does not exist",
		},
		{
			name:   "different valdiator",
			detail: "the validator that was preset doesn't correspond with the tryGet",
			preset: &types.Validator{
				Address:      newTestAddressBytes(t),
				StakedAmount: 100,
				Committees:   []uint64{lib.CanopyCommitteeId},
			},
			tryGet: newTestAddress(t, 1),
			error:  "validator does not exist",
		},
		{
			name:   "single validator",
			detail: "set and get a single validator",
			preset: &types.Validator{
				Address:      newTestAddressBytes(t),
				StakedAmount: 100,
				Committees:   []uint64{lib.CanopyCommitteeId},
			},
			tryGet: newTestAddress(t),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a test state machine
			sm := newTestStateMachine(t)
			// set the validator
			if test.preset != nil {
				require.NoError(t, sm.SetValidator(test.preset))
			}
			// execute the function call
			got, err := sm.GetValidator(test.tryGet)
			// validate the expected error
			require.Equal(t, test.error != "", err != nil, err)
			if err != nil {
				require.ErrorContains(t, err, test.error)
				return
			}
			require.EqualExportedValues(t, test.preset, got)
		})
	}
}

func TestGetValidatorExists(t *testing.T) {
	tests := []struct {
		name   string
		detail string
		preset *types.Validator
		tryGet crypto.AddressI
		exists bool
	}{
		{
			name:   "no preset",
			detail: "no validator was not preset so not exists",
			tryGet: newTestAddress(t),
			exists: false,
		},
		{
			name:   "different valdiator",
			detail: "the validator that was preset doesn't correspond with the tryGet",
			preset: &types.Validator{
				Address:      newTestAddressBytes(t),
				StakedAmount: 100,
				Committees:   []uint64{lib.CanopyCommitteeId},
			},
			tryGet: newTestAddress(t, 1),
			exists: false,
		},
		{
			name:   "single validator",
			detail: "set and get a single validator",
			preset: &types.Validator{
				Address:      newTestAddressBytes(t),
				StakedAmount: 100,
				Committees:   []uint64{lib.CanopyCommitteeId},
			},
			tryGet: newTestAddress(t),
			exists: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a test state machine
			sm := newTestStateMachine(t)
			// set the validator
			if test.preset != nil {
				require.NoError(t, sm.SetValidator(test.preset))
			}
			// execute the function call
			got, err := sm.GetValidatorExists(test.tryGet)
			require.NoError(t, err)
			// compare got vs expected
			require.Equal(t, test.exists, got)
		})
	}
}

func TestSetGetValidators(t *testing.T) {
	const amount = uint64(100)
	tests := []struct {
		name           string
		detail         string
		preset         []*types.Validator
		expectedSupply *types.Supply
	}{
		{
			name:   "validators (non-delegate)",
			detail: "set and get validators (non-delegate)",
			preset: []*types.Validator{
				{
					Address:      newTestAddressBytes(t),
					PublicKey:    newTestPublicKeyBytes(t),
					StakedAmount: amount + 2,
					Committees:   []uint64{lib.CanopyCommitteeId, 1},
				},
				{
					Address:      newTestAddressBytes(t, 1),
					PublicKey:    newTestPublicKeyBytes(t),
					StakedAmount: amount + 1,
					Committees:   []uint64{lib.CanopyCommitteeId, 1},
				},
				{
					Address:      newTestAddressBytes(t, 2),
					PublicKey:    newTestPublicKeyBytes(t),
					StakedAmount: amount,
					Committees:   []uint64{lib.CanopyCommitteeId, 1},
				},
			},
			expectedSupply: &types.Supply{
				Total:  amount*3 + 3,
				Staked: amount*3 + 3,
				CommitteeStaked: []*types.Pool{
					{
						Id:     lib.CanopyCommitteeId,
						Amount: amount*3 + 3,
					},
					{
						Id:     1,
						Amount: amount*3 + 3,
					},
				},
			},
		},
		{
			name:   "delegates",
			detail: "set and get delegates",
			preset: []*types.Validator{
				{
					Address:      newTestAddressBytes(t),
					PublicKey:    newTestPublicKeyBytes(t),
					StakedAmount: amount + 2,
					Delegate:     true,
					Committees:   []uint64{lib.CanopyCommitteeId, 1},
				},
				{
					Address:      newTestAddressBytes(t, 1),
					PublicKey:    newTestPublicKeyBytes(t),
					StakedAmount: amount + 1,
					Delegate:     true,
					Committees:   []uint64{lib.CanopyCommitteeId, 1},
				},
				{
					Address:      newTestAddressBytes(t, 2),
					PublicKey:    newTestPublicKeyBytes(t),
					StakedAmount: amount,
					Delegate:     true,
					Committees:   []uint64{lib.CanopyCommitteeId, 1},
				},
			},
			expectedSupply: &types.Supply{
				Total:         amount*3 + 3,
				Staked:        amount*3 + 3,
				DelegatedOnly: amount*3 + 3,
				CommitteeStaked: []*types.Pool{
					{
						Id:     lib.CanopyCommitteeId,
						Amount: amount*3 + 3,
					},
					{
						Id:     1,
						Amount: amount*3 + 3,
					},
				},
				CommitteeDelegatedOnly: []*types.Pool{
					{
						Id:     lib.CanopyCommitteeId,
						Amount: amount*3 + 3,
					},
					{
						Id:     1,
						Amount: amount*3 + 3,
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a test state machine
			sm := newTestStateMachine(t)
			// set the validators
			if test.preset != nil {
				// convenience variable for supply
				supply := &types.Supply{}
				require.NoError(t, sm.SetValidators(test.preset, supply))
				// set the supply
				require.NoError(t, sm.SetSupply(supply))
			}
			// execute the function call
			got, err := sm.GetValidators()
			require.NoError(t, err)
			// sort got by stake
			slices.SortFunc(got, func(a *types.Validator, b *types.Validator) int {
				switch {
				case a.StakedAmount == b.StakedAmount:
					return 0
				case a.StakedAmount < b.StakedAmount:
					return 1
				default:
					return -1
				}
			})
			// compare got vs expected
			for i, v := range got {
				require.EqualExportedValues(t, test.preset[i], v)
			}
			// get the committees from state
			set, err := sm.GetCommitteePaginated(lib.PageParams{}, lib.CanopyCommitteeId)
			require.NoError(t, err)
			// check committees got vs expected
			for i, member := range *set.Results.(*types.ValidatorPage) {
				require.EqualExportedValues(t, test.preset[i], member)
			}
			// get delegates from state
			set, err = sm.GetDelegatesPaginated(lib.PageParams{}, lib.CanopyCommitteeId)
			require.NoError(t, err)
			// check delegates got vs expected
			for i, member := range *set.Results.(*types.ValidatorPage) {
				require.EqualExportedValues(t, test.preset[i], member)
			}
			gotSupply, err := sm.GetSupply()
			require.NoError(t, err)
			// compare supply
			require.EqualExportedValues(t, test.expectedSupply, gotSupply)
		})
	}
}

func TestGetValidatorsPaginated(t *testing.T) {
	const amount = uint64(100)
	tests := []struct {
		name            string
		detail          string
		validators      []*types.Validator
		pageParams      lib.PageParams
		expectedAddress [][]byte
		filters         lib.ValidatorFilters
	}{
		{
			name:       "no validators",
			detail:     "test when there exists no validators in the state",
			validators: nil,
			pageParams: lib.PageParams{
				PageNumber: 1,
				PerPage:    100,
			},
		},
		{
			name:   "multi-validator",
			detail: "test with multiple validators and default page params",
			validators: []*types.Validator{
				{
					Address:      newTestAddressBytes(t),
					StakedAmount: amount,
				},
				{
					Address:      newTestAddressBytes(t, 1),
					StakedAmount: amount,
				},
				{
					Address:      newTestAddressBytes(t, 2),
					StakedAmount: amount,
				},
			},
			expectedAddress: [][]byte{
				newTestAddressBytes(t),
				newTestAddressBytes(t, 2),
				newTestAddressBytes(t, 1),
			},
			pageParams: lib.PageParams{
				PageNumber: 1,
				PerPage:    100,
			},
		},
		{
			name:   "multi-validator filter paused",
			detail: "test with multiple validators and default page params, filtering paused",
			validators: []*types.Validator{
				{
					Address:      newTestAddressBytes(t),
					StakedAmount: amount,
				},
				{
					Address:         newTestAddressBytes(t, 1),
					StakedAmount:    amount,
					MaxPausedHeight: 1,
				},
				{
					Address:         newTestAddressBytes(t, 2),
					StakedAmount:    amount,
					UnstakingHeight: 1,
				},
			},
			expectedAddress: [][]byte{
				newTestAddressBytes(t),
				newTestAddressBytes(t, 2),
			},
			pageParams: lib.PageParams{
				PageNumber: 1,
				PerPage:    100,
			},
			filters: lib.ValidatorFilters{
				Paused: lib.FilterOption_Exclude,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a test state machine
			sm := newTestStateMachine(t)
			// set the validators
			if test.validators != nil {
				require.NoError(t, sm.SetValidators(test.validators, &types.Supply{}))
			}
			// execute the function call
			page, err := sm.GetValidatorsPaginated(test.pageParams, test.filters)
			// validate no error
			require.NoError(t, err)
			// check got vs expected page type
			require.Equal(t, types.ValidatorsPageName, page.Type)
			// check got vs expected page params
			require.EqualExportedValues(t, test.pageParams, page.PageParams)
			// check got vs expected page result
			for i, got := range *page.Results.(*types.ValidatorPage) {
				require.Equal(t, test.expectedAddress[i], got.Address)
			}
		})
	}
}

func TestUpdateValidatorStake(t *testing.T) {
	const amount = uint64(100)
	tests := []struct {
		name           string
		detail         string
		preset         *types.Validator
		update         *types.Validator
		expectedSupply *types.Supply
	}{
		{
			name:   "no updates",
			detail: "no updates to the validator",
			preset: &types.Validator{
				Address:      newTestAddressBytes(t),
				StakedAmount: amount,
				Committees:   []uint64{0, 1},
			},
			update: &types.Validator{
				Address:      newTestAddressBytes(t),
				StakedAmount: amount,
				Committees:   []uint64{0, 1},
			},
			expectedSupply: &types.Supply{
				Total:  amount,
				Staked: amount,
				CommitteeStaked: []*types.Pool{
					{
						Id:     0,
						Amount: amount,
					},
					{
						Id:     1,
						Amount: amount,
					},
				},
			},
		},
		{
			name:   "stake",
			detail: "update validator stake",
			preset: &types.Validator{
				Address:      newTestAddressBytes(t),
				StakedAmount: amount,
				Committees:   []uint64{0, 1},
			},
			update: &types.Validator{
				Address:      newTestAddressBytes(t),
				StakedAmount: amount + 1,
				Committees:   []uint64{0, 1},
			},
			expectedSupply: &types.Supply{
				Total:  amount, // not updated by this function
				Staked: amount + 1,
				CommitteeStaked: []*types.Pool{
					{
						Id:     0,
						Amount: amount + 1,
					},
					{
						Id:     1,
						Amount: amount + 1,
					},
				},
			},
		},
		{
			name:   "delegated stake",
			detail: "update delegate stake",
			preset: &types.Validator{
				Address:      newTestAddressBytes(t),
				StakedAmount: amount,
				Committees:   []uint64{0, 1},
				Delegate:     true,
			},
			update: &types.Validator{
				Address:      newTestAddressBytes(t),
				StakedAmount: amount + 1,
				Committees:   []uint64{0, 1},
				Delegate:     true,
			},
			expectedSupply: &types.Supply{
				Total:         amount, // not updated by this function
				Staked:        amount + 1,
				DelegatedOnly: amount + 1,
				CommitteeStaked: []*types.Pool{
					{
						Id:     0,
						Amount: amount + 1,
					},
					{
						Id:     1,
						Amount: amount + 1,
					},
				},
				CommitteeDelegatedOnly: []*types.Pool{
					{
						Id:     0,
						Amount: amount + 1,
					},
					{
						Id:     1,
						Amount: amount + 1,
					},
				},
			},
		},
		{
			name:   "committees",
			detail: "update validator committees",
			preset: &types.Validator{
				Address:      newTestAddressBytes(t),
				StakedAmount: amount,
				Committees:   []uint64{0, 1},
			},
			update: &types.Validator{
				Address:      newTestAddressBytes(t),
				StakedAmount: amount + 1,
				Committees:   []uint64{0, 2},
			},
			expectedSupply: &types.Supply{
				Total:  amount, // not updated by this function
				Staked: amount + 1,
				CommitteeStaked: []*types.Pool{
					{
						Id:     0,
						Amount: amount + 1,
					},
					{
						Id:     2,
						Amount: amount + 1,
					},
				},
			},
		},
		{
			name:   "delegated committees",
			detail: "update delegate committees",
			preset: &types.Validator{
				Address:      newTestAddressBytes(t),
				StakedAmount: amount,
				Committees:   []uint64{0, 1},
				Delegate:     true,
			},
			update: &types.Validator{
				Address:      newTestAddressBytes(t),
				StakedAmount: amount + 1,
				Committees:   []uint64{0, 2},
				Delegate:     true,
			},
			expectedSupply: &types.Supply{
				Total:         amount, // not updated by this function
				Staked:        amount + 1,
				DelegatedOnly: amount + 1,
				CommitteeStaked: []*types.Pool{
					{
						Id:     0,
						Amount: amount + 1,
					},
					{
						Id:     2,
						Amount: amount + 1,
					},
				},
				CommitteeDelegatedOnly: []*types.Pool{
					{
						Id:     0,
						Amount: amount + 1,
					},
					{
						Id:     2,
						Amount: amount + 1,
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a test state machine
			sm := newTestStateMachine(t)
			// set the validators
			if test.preset != nil {
				supply := &types.Supply{}
				// set the validator
				require.NoError(t, sm.SetValidators([]*types.Validator{test.preset}, supply))
				// update the supply in state
				require.NoError(t, sm.SetSupply(supply))
			}
			// execute the function call
			require.NoError(t, sm.UpdateValidatorStake(test.preset, test.update.Committees, test.update.StakedAmount-test.preset.StakedAmount))
			// get the validator
			got, err := sm.GetValidator(crypto.NewAddress(test.preset.Address))
			require.NoError(t, err)
			// check got vs expected
			require.EqualExportedValues(t, test.update, got)
			// validate committee membership
			for _, cId := range test.update.Committees {
				var page *lib.Page
				if test.update.Delegate {
					// get the delegates
					page, err = sm.GetDelegatesPaginated(lib.PageParams{}, cId)
				} else {
					// get the committee
					page, err = sm.GetCommitteePaginated(lib.PageParams{}, cId)
				}
				require.NoError(t, err)
				// ensure the slice contains the expected
				var contains bool
				for _, member := range *page.Results.(*types.ValidatorPage) {
					if bytes.Equal(member.PublicKey, test.update.PublicKey) {
						contains = true
						break
					}
				}
				require.True(t, contains)
			}
			// get the supply
			supply, err := sm.GetSupply()
			require.NoError(t, err)
			// validate supply update
			require.EqualExportedValues(t, test.expectedSupply, supply)
		})
	}
}

func TestDeleteValidator(t *testing.T) {
	const amount = uint64(100)
	tests := []struct {
		name           string
		detail         string
		preset         *types.Validator
		expectedSupply *types.Supply
	}{
		{
			name:   "delete validator",
			detail: "delete validator with 1 committee",
			preset: &types.Validator{
				Address:      newTestAddressBytes(t),
				StakedAmount: amount,
				Committees:   []uint64{0},
			},
			expectedSupply: &types.Supply{
				Total: amount,
			},
		}, {
			name:   "delete validator multi committee",
			detail: "delete validator with multiple committees",
			preset: &types.Validator{
				Address:      newTestAddressBytes(t),
				StakedAmount: amount,
				Committees:   []uint64{0, 1, 2},
			},
			expectedSupply: &types.Supply{
				Total: amount,
			},
		},
		{
			name:   "delete delegate",
			detail: "delete delegate with 1 committee",
			preset: &types.Validator{
				Address:      newTestAddressBytes(t),
				StakedAmount: amount,
				Committees:   []uint64{0, 1, 2},
				Delegate:     true,
			},
			expectedSupply: &types.Supply{
				Total: amount,
			},
		},
		{
			name:   "delete delegate multi committee",
			detail: "delete delegate with multiple committees",
			preset: &types.Validator{
				Address:      newTestAddressBytes(t),
				StakedAmount: amount,
				Committees:   []uint64{0, 1, 2},
				Delegate:     true,
			},
			expectedSupply: &types.Supply{
				Total: amount,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a test state machine
			sm := newTestStateMachine(t)
			// set the validators
			if test.preset != nil {
				supply := &types.Supply{}
				// set the validator
				require.NoError(t, sm.SetValidators([]*types.Validator{test.preset}, supply))
				// update the supply in state
				require.NoError(t, sm.SetSupply(supply))
			}
			// execute the function call
			require.NoError(t, sm.DeleteValidator(test.preset))
			// get the validator
			_, err := sm.GetValidator(crypto.NewAddress(test.preset.Address))
			require.ErrorContains(t, err, "validator does not exist")
			// validate committee non-membership
			for _, cId := range test.preset.Committees {
				var page *lib.Page
				if test.preset.Delegate {
					// get the delegates
					page, err = sm.GetDelegatesPaginated(lib.PageParams{}, cId)
				} else {
					// get the committee
					page, err = sm.GetCommitteePaginated(lib.PageParams{}, cId)
				}
				require.NoError(t, err)
				// ensure the slice contains the expected
				var contains bool
				for _, member := range *page.Results.(*types.ValidatorPage) {
					if bytes.Equal(member.PublicKey, test.preset.PublicKey) {
						contains = true
						break
					}
				}
				require.False(t, contains)
			}
			// get the supply
			supply, err := sm.GetSupply()
			require.NoError(t, err)
			// validate supply update
			require.EqualExportedValues(t, test.expectedSupply, supply)
		})
	}
}

func TestSetValidatorUnstaking(t *testing.T) {
	tests := []struct {
		name                  string
		detail                string
		preset                *types.Validator
		finishUnstakingHeight uint64
	}{
		{
			name:   "set unstaking",
			detail: "set a standard validator unstaking",
			preset: &types.Validator{
				Address:         newTestAddressBytes(t),
				Committees:      nil,
				MaxPausedHeight: 0,
			},
			finishUnstakingHeight: 1,
		},
		{
			name:   "set paused unstaking",
			detail: "set a paused validator unstaking",
			preset: &types.Validator{
				Address:         newTestAddressBytes(t),
				Committees:      nil,
				MaxPausedHeight: 1,
			},
			finishUnstakingHeight: 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// convenience variable for address
			address := crypto.NewAddress(test.preset.Address)
			// create a test state machine
			sm := newTestStateMachine(t)
			// set the validator
			require.NoError(t, sm.SetValidator(test.preset))
			// execute the function call
			require.NoError(t, sm.SetValidatorUnstaking(address, test.preset, test.finishUnstakingHeight))
			// get the validator
			validator, err := sm.GetValidator(address)
			require.NoError(t, err)
			// ensure validator is unpaused
			require.Zero(t, validator.MaxPausedHeight)
			// ensure validator unstaking height is expected
			require.Equal(t, test.finishUnstakingHeight, validator.UnstakingHeight)
			// ensure unstaking key exists
			got, err := sm.Get(types.KeyForUnstaking(test.finishUnstakingHeight, address))
			require.NoError(t, err)
			require.Len(t, got, 1)
		})
	}
}

func TestDeleteFinishedUnstaking(t *testing.T) {
	tests := []struct {
		name   string
		detail string
		preset *types.Validator
	}{
		{
			name:   "validator same output/operator",
			detail: "validator with the same output and operator address",
			preset: &types.Validator{
				Address:      newTestAddressBytes(t),
				StakedAmount: 100,
				Committees:   []uint64{0, 1},
				Output:       newTestAddressBytes(t),
			},
		},
		{
			name:   "validator different output/operator",
			detail: "validator with the different output and operator address",
			preset: &types.Validator{
				Address:      newTestAddressBytes(t),
				StakedAmount: 100,
				Committees:   []uint64{0, 1},
				Output:       newTestAddressBytes(t, 1),
			},
		},
		{
			name:   "delegate same output/operator",
			detail: "delegate with the same output and operator address",
			preset: &types.Validator{
				Address:      newTestAddressBytes(t),
				StakedAmount: 100,
				Committees:   []uint64{0, 1},
				Output:       newTestAddressBytes(t),
				Delegate:     true,
			},
		},
		{
			name:   "delegate different output/operator",
			detail: "delegate with the different output and operator address",
			preset: &types.Validator{
				Address:      newTestAddressBytes(t),
				StakedAmount: 100,
				Committees:   []uint64{0, 1},
				Output:       newTestAddressBytes(t, 1),
				Delegate:     true,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// convenience variable for address
			address := crypto.NewAddress(test.preset.Address)
			// create a test state machine
			sm := newTestStateMachine(t)
			// convenience variable for supply
			supply := &types.Supply{}
			// set the validator in state
			require.NoError(t, sm.SetValidators([]*types.Validator{test.preset}, supply))
			// set the supply in state
			require.NoError(t, sm.SetSupply(supply))
			// set the validator as unstaking
			require.NoError(t, sm.SetValidatorUnstaking(address, test.preset, sm.height))
			// execute the function call
			require.NoError(t, sm.DeleteFinishedUnstaking())
			// get the validator
			_, err := sm.GetValidator(crypto.NewAddress(test.preset.Address))
			// validate the deletion of the validator
			require.ErrorContains(t, err, "validator does not exist")
			// get the output account balance
			balance, err := sm.GetAccountBalance(crypto.NewAddress(test.preset.Output))
			require.NoError(t, err)
			// validate the addition to the account
			require.Equal(t, test.preset.StakedAmount, balance)
			// ensure unstaking key doesn't exist
			got, err := sm.Get(types.KeyForUnstaking(sm.height, address))
			require.NoError(t, err)
			require.Len(t, got, 0)
		})
	}
}

func TestSetValidatorsPaused(t *testing.T) {
	tests := []struct {
		name    string
		detail  string
		preset  []*types.Validator
		toPause [][]byte
	}{
		{
			name:   "single validator pause",
			detail: "single validator pause",
			preset: []*types.Validator{{
				Address: newTestAddressBytes(t),
			}},
			toPause: [][]byte{newTestAddressBytes(t)},
		},
		{
			name:   "multi validator pause",
			detail: "multi validator pause",
			preset: []*types.Validator{{
				Address: newTestAddressBytes(t),
			}, {
				Address: newTestAddressBytes(t, 1),
			}},
			toPause: [][]byte{newTestAddressBytes(t), newTestAddressBytes(t, 1)},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a state machine instance with default parameters
			sm := newTestStateMachine(t)
			// preset the validator
			if test.preset != nil {
				supply := &types.Supply{}
				require.NoError(t, sm.SetValidators(test.preset, supply))
				require.NoError(t, sm.SetSupply(supply))
			}
			// execute the function call
			sm.SetValidatorsPaused(test.toPause)
			for _, validator := range test.toPause {
				paused := crypto.NewAddress(validator)
				// validate the unstaking of the validator object
				val, e := sm.GetValidator(paused)
				require.NoError(t, e)
				// get validator params
				valParams, e := sm.GetParamsVal()
				require.NoError(t, e)
				// calculate the finish unstaking height
				maxPauseBlocks := valParams.ValidatorMaxPauseBlocks + sm.Height()
				// compare got vs expected
				require.Equal(t, maxPauseBlocks, val.MaxPausedHeight)
				// check for the paused key
				bz, e := sm.Get(types.KeyForPaused(maxPauseBlocks, paused))
				require.NoError(t, e)
				require.Len(t, bz, 1)
			}
		})
	}
}

func TestSetValidatorPausedAndUnpaused(t *testing.T) {
	tests := []struct {
		name           string
		detail         string
		validator      *types.Validator
		maxPauseHeight uint64
	}{
		{
			name:           "pause height 1",
			detail:         "this function creates a validator object and a key for the validator under the unstaking prefix",
			validator:      &types.Validator{Address: newTestAddressBytes(t)},
			maxPauseHeight: 1,
		},
		{
			name:           "pause height 100",
			detail:         "this function creates a validator object and a key for the validator under the unstaking prefix",
			validator:      &types.Validator{Address: newTestAddressBytes(t)},
			maxPauseHeight: 100,
		},
		{
			name:           "pause height max",
			detail:         "this function creates a validator object and a key for the validator under the unstaking prefix",
			validator:      &types.Validator{Address: newTestAddressBytes(t)},
			maxPauseHeight: math.MaxUint64,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			address := crypto.NewAddress(test.validator.Address)
			// create a state machine instance with default parameters
			sm := newTestStateMachine(t)
			// execute the function 1
			require.NoError(t, sm.SetValidatorPaused(address, test.validator, test.maxPauseHeight))
			// validate the pause of the validator object
			val, e := sm.GetValidator(address)
			require.NoError(t, e)
			// compare got vs expected
			require.Equal(t, test.maxPauseHeight, val.MaxPausedHeight)
			// check for the paused key
			bz, e := sm.Get(types.KeyForPaused(test.maxPauseHeight, address))
			require.NoError(t, e)
			require.Len(t, bz, 1)
			// execute the function 2
			require.NoError(t, sm.SetValidatorUnpaused(address, test.validator))
			// validate the un-pause of the validator object
			val, e = sm.GetValidator(address)
			require.NoError(t, e)
			// compare got vs expected
			require.Zero(t, val.MaxPausedHeight)
			// validate no paused key
			bz, e = sm.Get(types.KeyForPaused(test.maxPauseHeight, address))
			require.NoError(t, e)
			require.Len(t, bz, 0)
		})
	}
}

func TestForceUnstakeMaxPaused(t *testing.T) {
	tests := []struct {
		name            string
		detail          string
		preset          []*types.Validator
		expected        []*types.Validator
		unstakingBlocks uint64
		height          uint64
	}{
		{
			name:   "single validator",
			detail: "only 1 validator",
			preset: []*types.Validator{
				{
					Address:         newTestAddressBytes(t),
					MaxPausedHeight: 2,
				},
			},
			expected: []*types.Validator{
				{
					Address:         newTestAddressBytes(t),
					MaxPausedHeight: 0,
					UnstakingHeight: 3,
				},
			},
			height:          2,
			unstakingBlocks: 1,
		},
		{
			name:   "multi validator all max paused",
			detail: "multiple validators all max paused",
			preset: []*types.Validator{
				{
					Address:         newTestAddressBytes(t),
					MaxPausedHeight: 2,
				},
				{
					Address:         newTestAddressBytes(t, 1),
					MaxPausedHeight: 2,
				},
			},
			expected: []*types.Validator{
				{
					Address:         newTestAddressBytes(t),
					MaxPausedHeight: 0,
					UnstakingHeight: 3,
				},
				{
					Address:         newTestAddressBytes(t, 1),
					MaxPausedHeight: 0,
					UnstakingHeight: 3,
				},
			},
			height:          2,
			unstakingBlocks: 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a state machine instance with default parameters
			sm := newTestStateMachine(t)
			// set the height
			sm.height = test.height
			// set the unstaking blocks
			require.NoError(t, sm.UpdateParam(types.ParamSpaceVal, types.ParamValidatorUnstakingBlocks, &lib.UInt64Wrapper{Value: test.unstakingBlocks}))
			// get the validator params
			valParams, err := sm.GetParamsVal()
			require.NoError(t, err)
			// for each validator
			for _, val := range test.preset {
				// preset the validators as paused
				require.NoError(t, sm.SetValidatorPaused(crypto.NewAddress(val.Address), val, sm.Height()))
			}
			// execute the function call
			require.NoError(t, sm.ForceUnstakeMaxPaused())
			// validate the effects
			for _, expected := range test.expected {
				address := crypto.NewAddress(expected.Address)
				// get the validator from state
				got, e := sm.GetValidator(address)
				require.NoError(t, e)
				// compare got vs expected
				require.EqualExportedValues(t, expected, got)
				// validate pause key removed
				bz, e := sm.Get(types.KeyForPaused(sm.Height(), address))
				require.NoError(t, e)
				require.Len(t, bz, 0)
				// validate not paused on structure
				require.Zero(t, expected.MaxPausedHeight)
				// calculate the expected unstaking height
				expectedUnstakingHeight := valParams.ValidatorUnstakingBlocks + sm.height
				// validate unstaking on structure
				require.Equal(t, expectedUnstakingHeight, expected.UnstakingHeight)
				// validate unstaking key exists
				bz, e = sm.Get(types.KeyForUnstaking(expectedUnstakingHeight, address))
				require.NoError(t, e)
				require.Len(t, bz, 1)
			}
		})
	}
}

func TestGetAuthorizedSignersForValidator(t *testing.T) {
	tests := []struct {
		name     string
		detail   string
		preset   *types.Validator
		address  []byte
		expected [][]byte
		error    string
	}{
		{
			name:    "validator doesn't exist",
			detail:  "the operation fails because the validator doesn't exist",
			address: newTestAddressBytes(t),
			expected: [][]byte{
				newTestAddressBytes(t),
			},
			error: "validator does not exist",
		}, {
			name:    "custodial",
			detail:  "same output and operator",
			address: newTestAddressBytes(t),
			preset: &types.Validator{
				Address: newTestAddressBytes(t),
				Output:  newTestAddressBytes(t),
			},
			expected: [][]byte{
				newTestAddressBytes(t),
			},
		},
		{
			name:    "non-custodial",
			detail:  "different output and operator",
			address: newTestAddressBytes(t),
			preset: &types.Validator{
				Address: newTestAddressBytes(t),
				Output:  newTestAddressBytes(t, 1),
			},
			expected: [][]byte{
				newTestAddressBytes(t), newTestAddressBytes(t, 1),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a state machine instance with default parameters
			sm := newTestStateMachine(t)
			// preset the validator
			if test.preset != nil {
				require.NoError(t, sm.SetValidator(test.preset))
			}
			// execute the function call
			got, err := sm.GetAuthorizedSignersForValidator(test.address)
			// validate the expected error
			require.Equal(t, test.error != "", err != nil, err)
			if err != nil {
				require.ErrorContains(t, err, test.error)
				return
			}
			// compare got vs expected
			require.Equal(t, test.expected, got)
		})
	}
}
