package fsm

import (
	"github.com/canopy-network/canopy/fsm/types"
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGetSetValidator(t *testing.T) {
	tests := []struct {
		name       string
		detail     string
		validators []*types.Validator
		error      lib.ErrorI
	}{
		{
			name:   "single validator",
			detail: "set and get a single validator",
			validators: []*types.Validator{
				{
					Address:      newTestAddressBytes(t),
					StakedAmount: 100,
					Committees:   []uint64{lib.CanopyCommitteeId},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a test state machine
			sm := newTestStateMachine(t)
			// set the validators
			for _, v := range test.validators {
				require.NoError(t, sm.SetValidator(v))
			}
			// get the validators
			for _, expected := range test.validators {
				got, err := sm.GetValidator(crypto.NewAddress(expected.Address))
				require.Equal(t, test.error, err)
				if err != nil {
					continue
				}
				require.EqualExportedValues(t, expected, got)
			}
		})
	}
}
