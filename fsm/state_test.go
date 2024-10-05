package fsm

import (
	"bytes"
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"github.com/ginchuco/ginchu/store"
	"github.com/stretchr/testify/require"
	"testing"
)

func newSingleAccountStateMachine(t *testing.T) StateMachine {
	sm := newTestStateMachine(t)
	keyGroup := newTestKeyGroup(t)
	require.NoError(t, sm.SetParams(types.DefaultParams()))
	require.NoError(t, sm.SetAccount(&types.Account{
		Address: keyGroup.Address.Bytes(),
		Amount:  1000000,
	}))
	require.NoError(t, sm.HandleMessageStake(&types.MessageStake{
		PublicKey:     keyGroup.PublicKey.Bytes(),
		Amount:        1000000,
		Committees:    []uint64{0},
		NetAddress:    "http://localhost:80",
		OutputAddress: keyGroup.Address.Bytes(),
		Delegate:      false,
		Compound:      true,
	}))
	require.NoError(t, sm.SetParams(types.DefaultParams()))
	return sm
}

func newTestStateMachine(t *testing.T) StateMachine {
	log := lib.NewDefaultLogger()
	db, err := store.NewStoreInMemory(log)
	require.NoError(t, err)
	return StateMachine{
		store:             db,
		ProtocolVersion:   0,
		NetworkID:         0,
		height:            2,
		vdfIterations:     0,
		proposeVoteConfig: types.AcceptAllProposals,
		Config:            lib.Config{},
		log:               log,
	}
}

func newTestAddress(_ *testing.T, variation ...bool) crypto.AddressI {
	if variation != nil {
		return crypto.NewAddress(bytes.Repeat([]byte("A"), crypto.AddressSize))
	}
	return crypto.NewAddress(bytes.Repeat([]byte("F"), crypto.AddressSize))
}

func newTestKeyGroup(t *testing.T) *crypto.KeyGroup {
	key, err := crypto.NewBLSPrivateKeyFromString("01553a101301cd7019b78ffa1186842dd93923e563b8ae22e2ab33ae889b23ee")
	require.NoError(t, err)
	return crypto.NewKeyGroup(key)
}
