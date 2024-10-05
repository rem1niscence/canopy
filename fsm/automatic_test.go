package fsm

import (
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestBeginBlock(t *testing.T) {
	tests := []struct {
		name            string
		isGenesis       bool
		protocolVersion int
		error           lib.ErrorI
	}{
		{
			name:            "begin_block at genesis",
			protocolVersion: 1,
			isGenesis:       true,
		},
		{
			name:            "begin_block at genesis with invalid protocol version",
			protocolVersion: 0,
			isGenesis:       true,
		},
		{
			name:            "begin_block after genesis",
			protocolVersion: 1,
		},
		{
			name:            "begin_block after genesis with invalid protocol version",
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

func (s *StateMachine) calculateRewardPerCommittee(t *testing.T, numberOfSubsidizedCommittees int) (mintAmountPerCommittee uint64, daoCut uint64) {
	// get the necessary parameters
	params, err := s.GetParamsVal()
	require.NoError(t, err)
	govParams, err := s.GetParamsGov()
	require.NoError(t, err)
	// the total mint amount is defined by a governance parameter
	totalMintAmount := params.ValidatorCommitteeReward
	// calculate the amount left for the committees after the parameterized DAO cut
	mintAmountAfterDAOCut := lib.Uint64ReducePercentage(totalMintAmount, float64(govParams.DaoRewardPercentage))
	// calculate the DAO cut
	daoCut = totalMintAmount - mintAmountAfterDAOCut
	// calculate the amount given to each qualifying committee
	mintAmountPerCommittee = mintAmountAfterDAOCut / uint64(numberOfSubsidizedCommittees)
	return
}
