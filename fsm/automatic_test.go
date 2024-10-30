package fsm

import (
	"github.com/ginchuco/canopy/fsm/types"
	"github.com/ginchuco/canopy/lib"
	"github.com/ginchuco/canopy/lib/crypto"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestBeginBlock(t *testing.T) {
	tests := []struct {
		name            string
		detail          string
		isGenesis       bool
		protocolVersion int
		error           lib.ErrorI
	}{
		{
			name:            "begin_block at genesis",
			detail:          "genesis skips begin block logic",
			protocolVersion: 1,
			isGenesis:       true,
		},
		{
			name:            "begin_block at genesis with invalid protocol version",
			detail:          "genesis skips begin block logic so invalid protocol version does not error",
			protocolVersion: 0,
			isGenesis:       true,
		},
		{
			name:            "begin_block after genesis",
			detail:          "after genesis with a valid protocol version",
			protocolVersion: 1,
		},
		{
			name:            "begin_block after genesis with invalid protocol version",
			detail:          "after genesis with an invalid protocol version will return error",
			protocolVersion: 0,
			error:           types.ErrInvalidProtocolVersion(),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				expectedCommitteeMint, expectedDAOMint uint64
			)
			// create a state machine instance with default parameters
			sm := newSingleAccountStateMachine(t)
			// set protocol version
			sm.ProtocolVersion = test.protocolVersion
			// if at genesis, set height to 1
			if test.isGenesis {
				sm.height = 1
			}
			// ensure expected error on function call
			require.Equal(t, test.error, sm.BeginBlock())
			if test.error != nil {
				return
			}
			// check committee reward
			if !test.isGenesis {
				expectedCommitteeMint, expectedDAOMint = sm.calculateRewardPerCommittee(t, 1)
			}
			// check canopy reward pool for proper mint
			canopyRewardPool, err := sm.GetPool(lib.CanopyCommitteeId)
			require.NoError(t, err)
			require.Equal(t, expectedCommitteeMint, canopyRewardPool.Amount)
			// check DAO reward pool for proper mint
			daoRewardPool, err := sm.GetPool(lib.DAOPoolID)
			require.NoError(t, err)
			require.Equal(t, expectedDAOMint, daoRewardPool.Amount)
		})
	}
}

func TestEndBlock(t *testing.T) {
	// generate committee data for testing
	committeeData := []*types.CommitteeData{
		{
			CommitteeId:     lib.CanopyCommitteeId,
			ChainHeight:     1,
			CommitteeHeight: 2,
			PaymentPercents: []*lib.PaymentPercents{
				{Address: newTestAddressBytes(t, 1), Percent: 100},
			},
		},
		{
			CommitteeId:     lib.CanopyCommitteeId,
			ChainHeight:     2,
			CommitteeHeight: 2,
			PaymentPercents: []*lib.PaymentPercents{
				{Address: newTestAddressBytes(t, 2), Percent: 100},
			},
		},
		{
			CommitteeId:     lib.CanopyCommitteeId,
			ChainHeight:     3,
			CommitteeHeight: 2,
			PaymentPercents: []*lib.PaymentPercents{
				{Address: newTestAddressBytes(t, 3), Percent: 100},
			},
		},
	}
	// create test cases
	tests := []struct {
		name                  string
		detail                string
		height                uint64
		previousProposers     [][]byte
		committeeRewardAmount uint64
		committeeData         []*types.CommitteeData
		validators            []*types.Validator
		error                 lib.ErrorI
	}{
		{
			name:              "genesis",
			detail:            "no previous proposers, no committee reward/data, no max paused validators, no unstaking validators",
			previousProposers: [][]byte{{}, {}, {}, {}, {}},
		},
		{
			name:   "after genesis, with previous proposers",
			detail: "with previous proposers, no committee reward/data, no max paused validators, no unstaking validators",
			previousProposers: [][]byte{
				newTestAddressBytes(t, 1),
				newTestAddressBytes(t, 2),
				newTestAddressBytes(t, 3),
				newTestAddressBytes(t, 4),
				newTestAddressBytes(t, 5),
			},
		},
		{
			name:                  "after genesis, with committee reward and data",
			detail:                "no previous proposers, with committee reward/data, no max paused validators, no unstaking validators",
			committeeRewardAmount: 100,
			committeeData:         committeeData,
			previousProposers:     [][]byte{{}, {}, {}, {}, {}},
		},
		{
			name:              "after genesis, with committee data and NO reward",
			detail:            "no previous proposers, with committee data and NO reward, no max paused validators, no unstaking validators",
			committeeData:     committeeData,
			previousProposers: [][]byte{{}, {}, {}, {}, {}},
		},
		{
			name:              "after genesis, with a max paused validator",
			detail:            "no previous proposers, no committee data/reward, one max paused validators, no unstaking validators",
			committeeData:     committeeData,
			height:            1,
			previousProposers: [][]byte{{}, {}, {}, {}, {}},
			validators: []*types.Validator{{
				Address:         newTestAddressBytes(t),
				NetAddress:      "http://localhost:8081",
				StakedAmount:    100,
				MaxPausedHeight: 1,
				Output:          newTestAddressBytes(t),
			}},
		},
		{
			name:              "after genesis, with an unstaking validator",
			detail:            "no previous proposers, no committee data/reward, no max paused validators, one unstaking validators",
			committeeData:     committeeData,
			height:            1,
			previousProposers: [][]byte{{}, {}, {}, {}, {}},
			validators: []*types.Validator{{
				Address:         newTestAddressBytes(t),
				NetAddress:      "http://localhost:8081",
				StakedAmount:    100,
				UnstakingHeight: 1,
				Output:          newTestAddressBytes(t),
			}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a test proposer address
			proposerAddress := newTestAddress(t).Bytes()
			// create a state machine instance with default parameters
			sm := newSingleAccountStateMachine(t)

			// STEP 0) inject the test data into state
			func() {
				// set height
				sm.height = test.height
				// set last proposers
				require.NoError(t, sm.SetLastProposers(&lib.Proposers{
					Addresses: test.previousProposers,
				}))
				// set the committee reward
				require.NoError(t, sm.MintToPool(lib.CanopyCommitteeId, test.committeeRewardAmount))
				// set the committee data
				for _, d := range test.committeeData {
					require.NoError(t, sm.UpsertCommitteeData(d))
				}
				// set the validators
				for _, v := range test.validators {
					if v.MaxPausedHeight != 0 {
						require.NoError(t, sm.SetValidatorPaused(crypto.NewAddress(v.Address), v, v.MaxPausedHeight))
					}
					if v.UnstakingHeight != 0 {
						require.NoError(t, sm.SetValidatorUnstaking(crypto.NewAddress(v.Address), v, v.UnstakingHeight))
					}
				}
			}()

			// STEP 1) run function call and check for expected error
			func() { require.Equal(t, test.error, sm.EndBlock(proposerAddress)) }()

			// STEP 2) validate the update of addresses who proposed the block
			func() {
				// generate expected proposers
				test.previousProposers[test.height%5] = proposerAddress
				// retrieve actual proposers
				lastProposers, err := sm.GetLastProposers()
				require.NoError(t, err)
				// validate equality between the expected and got
				require.Equal(t, test.previousProposers, lastProposers.Addresses)
			}()

			// STEP 3) validate the distribution of the committee rewards based on the various Committee Data
			func() {
				for _, d := range test.committeeData {
					for _, paymentPercents := range d.PaymentPercents {
						valParams, err := sm.GetParamsVal()
						require.NoError(t, err)
						// get the account that should have been minted to
						account, err := sm.GetAccount(crypto.NewAddress(paymentPercents.Address))
						// full_reward = ROUND_DOWN( percentage / number_of_samples * available_reward )
						fullReward := uint64(float64(paymentPercents.Percent) / float64(len(committeeData)*100) * float64(test.committeeRewardAmount))
						// if not compounding, use the early withdrawal reward
						earlyWithdrawalReward := lib.Uint64ReducePercentage(fullReward, float64(valParams.ValidatorEarlyWithdrawalPenalty))
						require.NoError(t, err)
						// compare got and expected
						require.Equal(t, earlyWithdrawalReward, account.Amount)
					}
				}
			}()

			// STEP 4) validate the force unstaking of validators who have been paused for MaxPauseBlocks
			func() {
				// if testing with validators with a max pause height
				if len(test.validators) == 0 || test.validators[0].MaxPausedHeight == 0 {
					return
				}
				// ensure validator no longer exists in state
				val, err := sm.GetValidator(crypto.NewAddress(test.validators[0].Address))
				require.NoError(t, err)
				// ensure was force unstaked
				require.True(t, val.UnstakingHeight != 0)
			}()

			// STEP 5) delete validators who are finishing unstaking
			func() {
				// if testing with validators with a max pause height
				if len(test.validators) == 0 || test.validators[0].UnstakingHeight == 0 {
					return
				}
				// ensure validator no longer exists in state
				exists, err := sm.GetValidatorExists(crypto.NewAddress(test.validators[0].Address))
				require.NoError(t, err)
				// ensure no longer exists after unstaking
				require.False(t, exists)
				// ensure output address has staked funds
				balance, err := sm.GetAccountBalance(crypto.NewAddress(test.validators[0].Output))
				require.NoError(t, err)
				require.Equal(t, test.validators[0].StakedAmount, balance)
			}()
		})
	}
}

func TestCheckProtocolVersion(t *testing.T) {
	tests := []struct {
		name                  string
		detail                string
		localProtocolVersion  int
		localHeight           uint64
		protocolVersion       uint64
		protocolVersionHeight uint64
		error                 lib.ErrorI
	}{
		{
			name:        "same protocol version before height",
			detail:      "local protocol version == protocol version && local height < protocol version height. Like someone upgrading before the required height",
			localHeight: 0, localProtocolVersion: 1,
			protocolVersionHeight: 1, protocolVersion: 1,
			error: nil,
		},
		{
			name:        "same protocol version at height",
			detail:      "local protocol version == protocol version && local height == protocol version height. Like someone on the proper version at the upgrade height",
			localHeight: 1, localProtocolVersion: 1,
			protocolVersionHeight: 1, protocolVersion: 1,
			error: nil,
		},
		{
			name:        "same protocol version at future height",
			detail:      "local protocol version == protocol version && local height > protocol version height. Like someone on the proper version after the upgrade height",
			localHeight: 2, localProtocolVersion: 1,
			protocolVersionHeight: 1, protocolVersion: 1,
			error: nil,
		},
		{
			name:        "higher local protocol version before height",
			detail:      "local protocol version > protocol version && local height < protocol version height. Like someone beyond upgraded",
			localHeight: 0, localProtocolVersion: 2,
			protocolVersionHeight: 1, protocolVersion: 1,
			error: nil,
		},
		{
			name:        "same protocol version at height",
			detail:      "local protocol version : protocol version && local height == protocol version height. Like someone beyond upgraded at the upgrade height",
			localHeight: 1, localProtocolVersion: 2,
			protocolVersionHeight: 1, protocolVersion: 1,
			error: nil,
		},
		{
			name:        "same protocol version at future height",
			detail:      "local protocol version > protocol version && local height > protocol version height. Like someone upgraded before the change parameter txn sent",
			localHeight: 2, localProtocolVersion: 2,
			protocolVersionHeight: 1, protocolVersion: 1,
			error: nil,
		},
		{
			name:        "higher protocol version before height",
			detail:      "local protocol version < protocol version && local height < protocol version height. Like someone not upgraded before the required height",
			localHeight: 0, localProtocolVersion: 0,
			protocolVersionHeight: 1, protocolVersion: 1,
			error: nil,
		},
		{
			name:        "higher protocol version at height",
			detail:      "local protocol version < protocol version && local height == protocol version height. Like someone not upgraded at the required height",
			localHeight: 1, localProtocolVersion: 0,
			protocolVersionHeight: 1, protocolVersion: 1,
			error: types.ErrInvalidProtocolVersion(),
		},
		{
			name:        "higher protocol version after height",
			detail:      "local protocol version < protocol version && local height == protocol version height. Like someone not upgraded after the required height",
			localHeight: 2, localProtocolVersion: 0,
			protocolVersionHeight: 1, protocolVersion: 1,
			error: types.ErrInvalidProtocolVersion(),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a state machine instance with default parameters
			sm := newSingleAccountStateMachine(t)
			// set the local protocol version
			sm.ProtocolVersion = test.localProtocolVersion
			// set the local height
			sm.height = test.localHeight
			// get consensus params
			consParams, err := sm.GetParamsCons()
			require.NoError(t, err)
			// set the protocol version in state
			consParams.ProtocolVersion = types.NewProtocolVersion(test.protocolVersionHeight, test.protocolVersion)
			require.NoError(t, sm.SetParamsCons(consParams))
			// run the function call and ensure expected error is returned
			require.Equal(t, test.error, sm.CheckProtocolVersion())
		})
	}
}

func TestLastProposers(t *testing.T) {
	tests := []struct {
		name              string
		detail            string
		height            uint64
		newProposer       []byte
		previousProposers [][]byte
		expectedProposers [][]byte
	}{
		{
			name:              "genesis",
			detail:            "no previous proposers",
			height:            0,
			newProposer:       newTestAddressBytes(t),
			expectedProposers: [][]byte{newTestAddressBytes(t), {}, {}, {}, {}},
		},
		{
			name:              "after genesis",
			detail:            "1 previous proposer",
			height:            1,
			newProposer:       newTestAddressBytes(t),
			previousProposers: [][]byte{newTestAddressBytes(t, 1), {}, {}, {}, {}},
			expectedProposers: [][]byte{newTestAddressBytes(t, 1), newTestAddressBytes(t), {}, {}, {}},
		},
		{
			name:              "with previous proposers",
			detail:            "with 5 previous proposers",
			height:            5,
			newProposer:       newTestAddressBytes(t),
			previousProposers: [][]byte{newTestAddressBytes(t, 1), newTestAddressBytes(t, 2), newTestAddressBytes(t, 3), newTestAddressBytes(t, 4), newTestAddressBytes(t, 5)},
			expectedProposers: [][]byte{newTestAddressBytes(t), newTestAddressBytes(t, 2), newTestAddressBytes(t, 3), newTestAddressBytes(t, 4), newTestAddressBytes(t, 5)},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a state machine instance with default parameters
			sm := newSingleAccountStateMachine(t)
			// set the local height
			sm.height = test.height
			// set previous proposers (if any)
			if test.previousProposers != nil {
				require.NoError(t, sm.SetLastProposers(&lib.Proposers{
					Addresses: test.previousProposers,
				}))
			}
			// update previous proposers
			require.NoError(t, sm.UpdateLastProposers(test.newProposer))
			// retrieve the last proposers from state
			lastProposers, err := sm.GetLastProposers()
			require.NoError(t, err)
			// ensure expected
			require.Equal(t, test.expectedProposers, lastProposers.Addresses)
		})
	}
}

func (s *StateMachine) calculateRewardPerCommittee(t *testing.T, numberOfSubsidizedCommittees int) (mintAmountPerCommittee uint64, daoCut uint64) {
	// get the necessary parameters
	params, err := s.GetParamsVal()
	require.NoError(t, err)
	govParams, err := s.GetParamsGov()
	require.NoError(t, err)
	// the total mint amount is defined by a governance parameter
	totalMintAmount := params.ValidatorBlockReward
	// calculate the amount left for the committees after the parameterized DAO cut
	mintAmountAfterDAOCut := lib.Uint64ReducePercentage(totalMintAmount, float64(govParams.DaoRewardPercentage))
	// calculate the DAO cut
	daoCut = totalMintAmount - mintAmountAfterDAOCut
	// calculate the amount given to each qualifying committee
	mintAmountPerCommittee = mintAmountAfterDAOCut / uint64(numberOfSubsidizedCommittees)
	return
}
