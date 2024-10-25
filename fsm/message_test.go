package fsm

import (
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestHandleMessage(t *testing.T) {
	const amount = uint64(100)
	// pre-create a 'change parameter' proposal to use during testing
	a, err := lib.NewAny(&lib.StringWrapper{Value: types.NewProtocolVersion(3, 2)})
	require.NoError(t, err)
	msgChangeParam := &types.MessageChangeParameter{
		ParameterSpace: "cons",
		ParameterKey:   types.ParamProtocolVersion,
		ParameterValue: a,
		StartHeight:    1,
		EndHeight:      2,
		Signer:         newTestAddressBytes(t),
	}
	// run test cases
	tests := []struct {
		name     string
		detail   string
		preset   func(sm StateMachine) // required state pre-set for message to be accepted
		msg      lib.MessageI
		validate func(sm StateMachine) // 'very basic' validation that the correct message was handled
		error    string
	}{
		{
			name:   "message send",
			detail: "basic 'happy path' handling for message send",
			preset: func(sm StateMachine) {
				require.NoError(t, sm.AccountAdd(newTestAddress(t), 100))
			},
			msg: &types.MessageSend{
				FromAddress: newTestAddressBytes(t),
				ToAddress:   newTestAddressBytes(t, 1),
				Amount:      amount,
			},
			validate: func(sm StateMachine) {
				// ensure the sender account was subtracted from
				got, e := sm.GetAccountBalance(newTestAddress(t))
				require.NoError(t, e)
				require.Zero(t, got)
				// ensure the receiver account was added to
				got, e = sm.GetAccountBalance(newTestAddress(t, 1))
				require.NoError(t, e)
				require.Equal(t, amount, got)
			},
		},
		{
			name:   "message stake",
			detail: "basic 'happy path' handling for message stake",
			preset: func(sm StateMachine) {
				require.NoError(t, sm.AccountAdd(newTestAddress(t), 100))
			},
			msg: &types.MessageStake{
				PublicKey:     newTestPublicKeyBytes(t),
				Amount:        amount,
				Committees:    []uint64{lib.CanopyCommitteeId},
				NetAddress:    "http://example.com",
				OutputAddress: newTestAddressBytes(t),
			},
			validate: func(sm StateMachine) {
				// ensure the sender account was subtracted from
				got, e := sm.GetAccountBalance(newTestAddress(t))
				require.NoError(t, e)
				require.Zero(t, got)
				// ensure the validator was created
				exists, e := sm.GetValidatorExists(newTestAddress(t))
				require.NoError(t, e)
				require.True(t, exists)
			},
		},
		{
			name:   "message edit-stake",
			detail: "basic 'happy path' handling for message edit-stake",
			preset: func(sm StateMachine) {
				// set account balance
				require.NoError(t, sm.AccountAdd(newTestAddress(t), 1))
				// create validator
				v := &types.Validator{
					Address:      newTestAddressBytes(t),
					StakedAmount: amount,
					Committees:   []uint64{lib.CanopyCommitteeId},
				}
				// add the validator stake to total supply
				require.NoError(t, sm.AddToTotalSupply(v.StakedAmount))
				// add the validator stake to supply
				require.NoError(t, sm.AddToStakedSupply(v.StakedAmount))
				// set the validator in state
				require.NoError(t, sm.SetValidator(v))
				// set validator committees
				require.NoError(t, sm.SetCommittees(crypto.NewAddress(v.Address), v.StakedAmount, v.Committees))
			},
			msg: &types.MessageEditStake{
				Address:       newTestAddressBytes(t),
				Amount:        amount + 1,
				Committees:    []uint64{lib.CanopyCommitteeId},
				NetAddress:    "http://example.com",
				OutputAddress: newTestAddressBytes(t),
			},
			validate: func(sm StateMachine) {
				// ensure the sender account was subtracted from
				got, e := sm.GetAccountBalance(newTestAddress(t))
				require.NoError(t, e)
				require.Zero(t, got)
				// ensure the validator stake was updated
				val, e := sm.GetValidator(newTestAddress(t))
				require.NoError(t, e)
				require.Equal(t, amount+1, val.StakedAmount)
			},
		},
		{
			name:   "message unstake",
			detail: "basic 'happy path' handling for message unstake",
			preset: func(sm StateMachine) {
				// create validator
				v := &types.Validator{
					Address:      newTestAddressBytes(t),
					StakedAmount: amount,
					Committees:   []uint64{lib.CanopyCommitteeId},
				}
				// set the validator in state
				require.NoError(t, sm.SetValidator(v))
			},
			msg: &types.MessageUnstake{Address: newTestAddressBytes(t)},
			validate: func(sm StateMachine) {
				// ensure the validator is unstaking
				val, e := sm.GetValidator(newTestAddress(t))
				require.NoError(t, e)
				require.NotZero(t, val.UnstakingHeight)
			},
		},
		{
			name:   "message pause",
			detail: "basic 'happy path' handling for message pause",
			preset: func(sm StateMachine) {
				// create validator
				v := &types.Validator{
					Address:      newTestAddressBytes(t),
					StakedAmount: amount,
					Committees:   []uint64{lib.CanopyCommitteeId},
				}
				// set the validator in state
				require.NoError(t, sm.SetValidator(v))
			},
			msg: &types.MessagePause{Address: newTestAddressBytes(t)},
			validate: func(sm StateMachine) {
				// ensure the validator is paused
				val, e := sm.GetValidator(newTestAddress(t))
				require.NoError(t, e)
				require.NotZero(t, val.MaxPausedHeight)
			},
		},
		{
			name:   "message unpause",
			detail: "basic 'happy path' handling for message unpause",
			preset: func(sm StateMachine) {
				// create validator
				v := &types.Validator{
					Address:         newTestAddressBytes(t),
					StakedAmount:    amount,
					Committees:      []uint64{lib.CanopyCommitteeId},
					MaxPausedHeight: 1,
				}
				// set the validator in state
				require.NoError(t, sm.SetValidator(v))
			},
			msg: &types.MessageUnpause{Address: newTestAddressBytes(t)},
			validate: func(sm StateMachine) {
				// ensure the validator is paused
				val, e := sm.GetValidator(newTestAddress(t))
				require.NoError(t, e)
				require.Zero(t, val.MaxPausedHeight)
			},
		},
		{
			name:   "message change param",
			detail: "basic 'happy path' handling for message change param",
			preset: func(sm StateMachine) {},
			msg:    msgChangeParam,
			validate: func(sm StateMachine) {
				// ensure the validator is paused
				consParams, e := sm.GetParamsCons()
				require.NoError(t, e)
				require.Equal(t, types.NewProtocolVersion(3, 2), consParams.ProtocolVersion)
			},
		},
		{
			name:   "message dao transfer",
			detail: "basic 'happy path' handling for message dao transfer",
			preset: func(sm StateMachine) {
				require.NoError(t, sm.PoolAdd(lib.DAOPoolID, amount))
			},
			msg: &types.MessageDAOTransfer{
				Address:     newTestAddressBytes(t),
				Amount:      amount,
				StartHeight: 1,
				EndHeight:   2,
			},
			validate: func(sm StateMachine) {
				// ensure the pool was subtracted from
				got, e := sm.GetPoolBalance(lib.DAOPoolID)
				require.NoError(t, e)
				require.Zero(t, got)
				// ensure the receiver account was added to
				got, e = sm.GetAccountBalance(newTestAddress(t))
				require.NoError(t, e)
				require.Equal(t, amount, got)
			},
		},
		{
			name:   "message subsidy",
			detail: "basic 'happy path' handling for message subsidy",
			preset: func(sm StateMachine) {
				require.NoError(t, sm.AccountAdd(newTestAddress(t), amount))
			},
			msg: &types.MessageSubsidy{
				Address:     newTestAddressBytes(t),
				CommitteeId: lib.CanopyCommitteeId,
				Amount:      amount,
				Opcode:      "note",
			},
			validate: func(sm StateMachine) {
				// ensure the account was subtracted from
				got, e := sm.GetAccountBalance(newTestAddress(t))
				require.NoError(t, e)
				require.Zero(t, got)
				// ensure the pool was added to
				got, e = sm.GetPoolBalance(lib.CanopyCommitteeId)
				require.NoError(t, e)
				require.Equal(t, amount, got)
			},
		},
		{
			name:   "message create order",
			detail: "basic 'happy path' handling for message create order",
			preset: func(sm StateMachine) {
				require.NoError(t, sm.AccountAdd(newTestAddress(t), amount))
				// get the validator params
				params, e := sm.GetParamsVal()
				require.NoError(t, e)
				// update the minimum order size to accomodate the small amount
				params.ValidatorMinimumOrderSize = amount
				// set the params back in state
				require.NoError(t, sm.SetParamsVal(params))
			},
			msg: &types.MessageCreateOrder{
				CommitteeId:          lib.CanopyCommitteeId,
				AmountForSale:        amount,
				RequestedAmount:      1000,
				SellerReceiveAddress: newTestPublicKeyBytes(t),
				SellersSellAddress:   newTestAddressBytes(t),
			},
			validate: func(sm StateMachine) {
				// ensure the account was subtracted from
				got, e := sm.GetAccountBalance(newTestAddress(t))
				require.NoError(t, e)
				require.Zero(t, got)
				// ensure the pool was added to
				got, e = sm.GetPoolBalance(lib.CanopyCommitteeId + types.EscrowPoolAddend)
				require.NoError(t, e)
				require.Equal(t, amount, got)
				// ensure the order was created
				order, e := sm.GetOrder(0, lib.CanopyCommitteeId)
				require.NoError(t, e)
				require.Equal(t, amount, order.AmountForSale)
			},
		},
		{
			name:   "message edit order",
			detail: "basic 'happy path' handling for message edit order",
			preset: func(sm StateMachine) {
				require.NoError(t, sm.AccountAdd(newTestAddress(t), amount))
				// get the validator params
				params, e := sm.GetParamsVal()
				require.NoError(t, e)
				// update the minimum order size to accomodate the small amount
				params.ValidatorMinimumOrderSize = amount
				// set the params back in state
				require.NoError(t, sm.SetParamsVal(params))
				// pre-set an order to edit
				// add to the pool
				require.NoError(t, sm.PoolAdd(lib.CanopyCommitteeId+types.EscrowPoolAddend, amount))
				// save the order in state
				_, err = sm.CreateOrder(&types.SellOrder{
					Committee:            lib.CanopyCommitteeId,
					AmountForSale:        amount,
					RequestedAmount:      1000,
					SellerReceiveAddress: newTestPublicKeyBytes(t),
					SellersSellAddress:   newTestAddressBytes(t),
				}, lib.CanopyCommitteeId)
				require.NoError(t, err)
			},
			msg: &types.MessageEditOrder{
				OrderId:              0,
				CommitteeId:          lib.CanopyCommitteeId,
				AmountForSale:        amount * 2,
				RequestedAmount:      2000,
				SellerReceiveAddress: newTestAddressBytes(t),
			},
			validate: func(sm StateMachine) {
				// ensure the account was subtracted from
				got, e := sm.GetAccountBalance(newTestAddress(t))
				require.NoError(t, e)
				require.Zero(t, got)
				// ensure the pool was added to
				got, e = sm.GetPoolBalance(lib.CanopyCommitteeId + types.EscrowPoolAddend)
				require.NoError(t, e)
				require.Equal(t, amount*2, got)
				// ensure the order was edited
				order, e := sm.GetOrder(0, lib.CanopyCommitteeId)
				require.NoError(t, e)
				require.Equal(t, amount*2, order.AmountForSale)
			},
		},
		{
			name:   "message delete order",
			detail: "basic 'happy path' handling for message delete order",
			preset: func(sm StateMachine) {
				// add to the pool
				require.NoError(t, sm.PoolAdd(lib.CanopyCommitteeId+types.EscrowPoolAddend, amount))
				// save the order in state
				_, err = sm.CreateOrder(&types.SellOrder{
					Committee:            lib.CanopyCommitteeId,
					AmountForSale:        amount,
					RequestedAmount:      1000,
					SellerReceiveAddress: newTestPublicKeyBytes(t),
					SellersSellAddress:   newTestAddressBytes(t),
				}, lib.CanopyCommitteeId)
				require.NoError(t, err)
			},
			msg: &types.MessageDeleteOrder{
				OrderId:     0,
				CommitteeId: lib.CanopyCommitteeId,
			},
			validate: func(sm StateMachine) {
				// ensure the account was subtracted from
				got, e := sm.GetAccountBalance(newTestAddress(t))
				require.NoError(t, e)
				require.Equal(t, amount, got)
				// ensure the pool was added to
				got, e = sm.GetPoolBalance(lib.CanopyCommitteeId + types.EscrowPoolAddend)
				require.NoError(t, e)
				require.Zero(t, got)
				// ensure the order was deleted
				_, e = sm.GetOrder(0, lib.CanopyCommitteeId)
				require.ErrorContains(t, e, "not found")
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a state machine instance with default parameters
			sm := newTestStateMachine(t)
			// run the preset function
			test.preset(sm)
			// execute the handler
			e := sm.HandleMessage(test.msg)
			// validate the expected error
			require.Equal(t, test.error != "", e != nil, e)
			if e != nil {
				require.ErrorContains(t, e, test.error)
				return
			}
			// run the validation
			test.validate(sm)
		})
	}
}

func TestGetFeeForMessage(t *testing.T) {
	tests := []struct {
		name   string
		detail string
		msg    lib.MessageI
	}{
		{
			name:   "msg send",
			detail: "evaluates the function for message send",
			msg:    &types.MessageSend{},
		},
		{
			name:   "msg stake",
			detail: "evaluates the function for message stake",
			msg:    &types.MessageStake{},
		},
		{
			name:   "msg edit-stake",
			detail: "evaluates the function for message edit-stake",
			msg:    &types.MessageEditStake{},
		},
		{
			name:   "msg unstake",
			detail: "evaluates the function for message unstake",
			msg:    &types.MessageUnstake{},
		},
		{
			name:   "msg pause",
			detail: "evaluates the function for message pause",
			msg:    &types.MessagePause{},
		},
		{
			name:   "msg unpause",
			detail: "evaluates the function for message unpause",
			msg:    &types.MessageUnpause{},
		},
		{
			name:   "msg change param",
			detail: "evaluates the function for message change param",
			msg:    &types.MessageChangeParameter{},
		},
		{
			name:   "msg dao transfer",
			detail: "evaluates the function for message dao transfer",
			msg:    &types.MessageDAOTransfer{},
		},
		{
			name:   "msg certificate results",
			detail: "evaluates the function for message certificate results",
			msg:    &types.MessageCertificateResults{},
		},
		{
			name:   "msg subsidy",
			detail: "evaluates the function for message subsidy",
			msg:    &types.MessageSubsidy{},
		},
		{
			name:   "msg create order",
			detail: "evaluates the function for message create order",
			msg:    &types.MessageCreateOrder{},
		},
		{
			name:   "msg edit order",
			detail: "evaluates the function for message edit order",
			msg:    &types.MessageEditOrder{},
		},
		{
			name:   "msg delete order",
			detail: "evaluates the function for message delete order",
			msg:    &types.MessageDeleteOrder{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a state machine instance with default parameters
			sm := newTestStateMachine(t)
			// get the fee params
			feeParams, err := sm.GetParamsFee()
			require.NoError(t, err)
			// define expected
			expected := func() uint64 {
				switch test.msg.(type) {
				case *types.MessageSend:
					return feeParams.MessageSendFee
				case *types.MessageStake:
					return feeParams.MessageStakeFee
				case *types.MessageEditStake:
					return feeParams.MessageEditStakeFee
				case *types.MessageUnstake:
					return feeParams.MessageUnstakeFee
				case *types.MessagePause:
					return feeParams.MessagePauseFee
				case *types.MessageUnpause:
					return feeParams.MessageUnpauseFee
				case *types.MessageChangeParameter:
					return feeParams.MessageChangeParameterFee
				case *types.MessageDAOTransfer:
					return feeParams.MessageDaoTransferFee
				case *types.MessageCertificateResults:
					return feeParams.MessageCertificateResultsFee
				case *types.MessageSubsidy:
					return feeParams.MessageSubsidyFee
				case *types.MessageCreateOrder:
					return feeParams.MessageCreateOrderFee
				case *types.MessageEditOrder:
					return feeParams.MessageEditOrderFee
				case *types.MessageDeleteOrder:
					return feeParams.MessageDeleteOrderFee
				default:
					panic("unknown msg")
				}
			}()
			// execute function call
			got, err := sm.GetFeeForMessage(test.msg)
			// validate the expected error
			require.NoError(t, err)
			// compare got vs expected
			require.Equal(t, expected, got)
		})
	}
}

func TestGetAuthorizedSignersFor(t *testing.T) {
	tests := []struct {
		name     string
		detail   string
		msg      lib.MessageI
		expected [][]byte
	}{
		{
			name:     "msg send",
			detail:   "retrieves the authorized signers for message send",
			msg:      &types.MessageSend{FromAddress: newTestAddressBytes(t)},
			expected: [][]byte{newTestAddressBytes(t)},
		}, {
			name:     "msg stake",
			detail:   "retrieves the authorized signers for message stake",
			msg:      &types.MessageStake{PublicKey: newTestPublicKeyBytes(t), OutputAddress: newTestAddressBytes(t)},
			expected: [][]byte{newTestAddressBytes(t), newTestAddressBytes(t)},
		}, {
			name:     "msg edit-stake",
			detail:   "retrieves the authorized signers for message stake",
			msg:      &types.MessageEditStake{Address: newTestAddressBytes(t)},
			expected: [][]byte{newTestAddressBytes(t), newTestAddressBytes(t, 1)},
		}, {
			name:     "msg unstake",
			detail:   "retrieves the authorized signers for message unstake",
			msg:      &types.MessageUnstake{Address: newTestAddressBytes(t)},
			expected: [][]byte{newTestAddressBytes(t), newTestAddressBytes(t, 1)},
		}, {
			name:     "msg pause",
			detail:   "retrieves the authorized signers for message pause",
			msg:      &types.MessagePause{Address: newTestAddressBytes(t)},
			expected: [][]byte{newTestAddressBytes(t), newTestAddressBytes(t, 1)},
		}, {
			name:     "msg unpause",
			detail:   "retrieves the authorized signers for message unpause",
			msg:      &types.MessageUnpause{Address: newTestAddressBytes(t)},
			expected: [][]byte{newTestAddressBytes(t), newTestAddressBytes(t, 1)},
		}, {
			name:     "msg change param",
			detail:   "retrieves the authorized signers for message change param",
			msg:      &types.MessageChangeParameter{Signer: newTestAddressBytes(t)},
			expected: [][]byte{newTestAddressBytes(t)},
		}, {
			name:     "msg dao transfer",
			detail:   "retrieves the authorized signers for message dao transfer",
			msg:      &types.MessageDAOTransfer{Address: newTestAddressBytes(t)},
			expected: [][]byte{newTestAddressBytes(t)},
		}, {
			name:     "msg subsidy",
			detail:   "retrieves the authorized signers for message subsidy",
			msg:      &types.MessageSubsidy{Address: newTestAddressBytes(t)},
			expected: [][]byte{newTestAddressBytes(t)},
		}, {
			name:     "msg create order",
			detail:   "retrieves the authorized signers for message create order",
			msg:      &types.MessageCreateOrder{SellersSellAddress: newTestAddressBytes(t)},
			expected: [][]byte{newTestAddressBytes(t)},
		}, {
			name:     "msg edit order",
			detail:   "retrieves the authorized signers for message edit order",
			msg:      &types.MessageEditOrder{},
			expected: [][]byte{newTestAddressBytes(t)},
		}, {
			name:     "msg delete order",
			detail:   "retrieves the authorized signers for message delete order",
			msg:      &types.MessageEditOrder{},
			expected: [][]byte{newTestAddressBytes(t)},
		}, {
			name:   "msg certificate results",
			detail: "retrieves the authorized signers for message delete order",
			msg: &types.MessageCertificateResults{
				Qc: &lib.QuorumCertificate{
					Header: &lib.View{CommitteeId: lib.CanopyCommitteeId, Height: 1},
				},
			},
			expected: [][]byte{newTestAddressBytes(t)},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a state machine instance with default parameters
			sm := newTestStateMachine(t)
			// set the state machine height at 1 for the 'time machine' call
			sm.height = 1
			// preset a validator
			require.NoError(t, sm.SetValidator(&types.Validator{
				Address:      newTestAddressBytes(t),
				PublicKey:    newTestPublicKeyBytes(t),
				StakedAmount: 100,
				Output:       newTestAddressBytes(t, 1),
			}))
			// preset a committee member
			require.NoError(t, sm.SetCommitteeMember(newTestAddress(t), lib.CanopyCommitteeId, 100))
			// preset an order
			_, err := sm.CreateOrder(&types.SellOrder{
				Committee:          lib.CanopyCommitteeId,
				SellersSellAddress: newTestAddressBytes(t),
			}, lib.CanopyCommitteeId)
			require.NoError(t, err)
			// execute function call
			got, err := sm.GetAuthorizedSignersFor(test.msg)
			// validate the expected error
			require.NoError(t, err)
			// compare got vs expected
			require.Equal(t, test.expected, got)
		})
	}
}

func TestHandleMessageSend(t *testing.T) {
	tests := []struct {
		name           string
		detail         string
		presetSender   uint64
		presetReceiver uint64
		msg            *types.MessageSend
		error          string
	}{
		{
			name:           "insufficient amount",
			detail:         "the sender doesn't have enough tokens",
			presetSender:   1,
			presetReceiver: 0,
			msg: &types.MessageSend{
				FromAddress: newTestAddressBytes(t),
				ToAddress:   newTestAddressBytes(t, 1),
				Amount:      2,
			},
			error: "insufficient funds",
		},
		{
			name:           "send all",
			detail:         "the sender sends all of its tokens (1) to the recipient",
			presetSender:   1,
			presetReceiver: 0,
			msg: &types.MessageSend{
				FromAddress: newTestAddressBytes(t),
				ToAddress:   newTestAddressBytes(t, 1),
				Amount:      1,
			},
		},
		{
			name:           "send 1",
			detail:         "the sender sends one of its tokens to the recipient",
			presetSender:   2,
			presetReceiver: 0,
			msg: &types.MessageSend{
				FromAddress: newTestAddressBytes(t),
				ToAddress:   newTestAddressBytes(t, 1),
				Amount:      1,
			},
		},
		{
			name:           "add one",
			detail:         "the sender sends 1 of its tokens to the recipient, who adds it to their existing balance",
			presetSender:   2,
			presetReceiver: 1,
			msg: &types.MessageSend{
				FromAddress: newTestAddressBytes(t),
				ToAddress:   newTestAddressBytes(t, 1),
				Amount:      1,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a state machine instance with default parameters
			sm := newTestStateMachine(t)
			// create sender addr object
			sender := crypto.NewAddress(test.msg.FromAddress)
			// create recipient addr object
			recipient := crypto.NewAddress(test.msg.ToAddress)
			// preset the accounts with some funds
			require.NoError(t, sm.AccountAdd(sender, test.presetSender))
			require.NoError(t, sm.AccountAdd(recipient, test.presetReceiver))
			// execute the function call
			err := sm.HandleMessageSend(test.msg)
			// validate the expected error
			require.Equal(t, test.error != "", err != nil, err)
			if err != nil {
				require.ErrorContains(t, err, test.error)
				return
			}
			// validate the send
			got, err := sm.GetAccount(sender)
			require.NoError(t, err)
			require.Equal(t, test.presetSender-test.msg.Amount, got.Amount)
			// validate the receipt
			got, err = sm.GetAccount(recipient)
			require.NoError(t, err)
			// compare got vs expected
			require.Equal(t, test.presetReceiver+test.msg.Amount, got.Amount)
		})
	}
}

func TestHandleMessageStake(t *testing.T) {
	tests := []struct {
		name            string
		detail          string
		presetSender    uint64
		presetValidator bool
		msg             *types.MessageStake
		expected        *types.Validator
		error           string
	}{
		{
			name:   "invalid public key",
			detail: "the sender public key is invalid",
			msg:    &types.MessageStake{PublicKey: newTestAddressBytes(t)},
			error:  "public key is invalid",
		}, {
			name:            "validator already exists",
			detail:          "the validator already exists in state",
			msg:             &types.MessageStake{PublicKey: newTestPublicKeyBytes(t)},
			expected:        &types.Validator{Address: newTestAddressBytes(t)},
			presetValidator: true,
			error:           "validator exists",
		},
		{
			name:         "insufficient amount",
			detail:       "the sender doesn't have enough tokens",
			presetSender: 0,
			msg: &types.MessageStake{
				PublicKey: newTestPublicKeyBytes(t),
				Amount:    1,
			},
			error: "insufficient funds",
		},
		{
			name:         "stake all funds as committee member",
			detail:       "the sender stakes all funds as committee member",
			presetSender: 1,
			msg: &types.MessageStake{
				PublicKey:     newTestPublicKeyBytes(t),
				Amount:        1,
				Committees:    []uint64{0, 1},
				NetAddress:    "http://example.com",
				OutputAddress: newTestAddressBytes(t, 1),
				Delegate:      false,
				Compound:      true,
			},
			expected: &types.Validator{
				Address:      newTestAddressBytes(t),
				PublicKey:    newTestPublicKeyBytes(t),
				NetAddress:   "http://example.com",
				StakedAmount: 1,
				Committees:   []uint64{0, 1},
				Output:       newTestAddressBytes(t, 1),
				Delegate:     false,
				Compound:     true,
			},
		},
		{
			name:         "stake partial funds as committee member",
			detail:       "the sender stakes partial funds as committee member",
			presetSender: 2,
			msg: &types.MessageStake{
				PublicKey:     newTestPublicKeyBytes(t),
				Amount:        1,
				Committees:    []uint64{0, 1},
				NetAddress:    "http://example.com",
				OutputAddress: newTestAddressBytes(t, 1),
				Delegate:      false,
				Compound:      true,
			},
			expected: &types.Validator{
				Address:      newTestAddressBytes(t),
				PublicKey:    newTestPublicKeyBytes(t),
				NetAddress:   "http://example.com",
				StakedAmount: 1,
				Committees:   []uint64{0, 1},
				Output:       newTestAddressBytes(t, 1),
				Delegate:     false,
				Compound:     true,
			},
		},
		{
			name:         "stake all funds as delegate",
			detail:       "the sender stakes all funds as delegate",
			presetSender: 1,
			msg: &types.MessageStake{
				PublicKey:     newTestPublicKeyBytes(t),
				Amount:        1,
				Committees:    []uint64{0, 1},
				NetAddress:    "http://example.com",
				OutputAddress: newTestAddressBytes(t, 1),
				Delegate:      true,
				Compound:      true,
			},
			expected: &types.Validator{
				Address:      newTestAddressBytes(t),
				PublicKey:    newTestPublicKeyBytes(t),
				NetAddress:   "http://example.com",
				StakedAmount: 1,
				Committees:   []uint64{0, 1},
				Output:       newTestAddressBytes(t, 1),
				Delegate:     true,
				Compound:     true,
			},
		},
		{
			name:         "stake partial funds as delegate",
			detail:       "the sender stakes partial funds as delegate",
			presetSender: 2,
			msg: &types.MessageStake{
				PublicKey:     newTestPublicKeyBytes(t),
				Amount:        1,
				Committees:    []uint64{0, 1},
				NetAddress:    "http://example.com",
				OutputAddress: newTestAddressBytes(t, 1),
				Delegate:      true,
				Compound:      true,
			},
			expected: &types.Validator{
				Address:      newTestAddressBytes(t),
				PublicKey:    newTestPublicKeyBytes(t),
				NetAddress:   "http://example.com",
				StakedAmount: 1,
				Committees:   []uint64{0, 1},
				Output:       newTestAddressBytes(t, 1),
				Delegate:     true,
				Compound:     true,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var sender crypto.AddressI
			// create a state machine instance with default parameters
			sm := newTestStateMachine(t)
			// create sender pubkey object
			publicKey, err := crypto.NewPublicKeyFromBytes(test.msg.PublicKey)
			if err == nil {
				// create sender addr object
				sender = publicKey.Address()
				// preset the accounts with some funds
				require.NoError(t, sm.AccountAdd(sender, test.presetSender))
			}
			// preset the validator
			if test.presetValidator {
				require.NoError(t, sm.SetValidator(test.expected))
			}
			// execute the function call
			err = sm.HandleMessageStake(test.msg)
			// validate the expected error
			require.Equal(t, test.error != "", err != nil, err)
			if err != nil {
				require.ErrorContains(t, err, test.error)
				return
			}
			// validate the stake
			got, err := sm.GetAccount(sender)
			require.NoError(t, err)
			require.Equal(t, test.presetSender-test.msg.Amount, got.Amount)
			// validate the creation of the validator object
			val, err := sm.GetValidator(sender)
			require.NoError(t, err)
			// compare got vs expected
			require.EqualExportedValues(t, test.expected, val)
			// get the supply object
			supply, err := sm.GetSupply()
			require.NoError(t, err)
			// validate the addition to the staked pool
			require.Equal(t, test.msg.Amount, supply.Staked)
			// validate the addition to the committees
			for _, id := range val.Committees {
				// get the supply for each committee
				stakedSupply, e := sm.GetCommitteeStakedSupply(id)
				require.NoError(t, e)
				require.Equal(t, test.msg.Amount, stakedSupply.Amount)
			}
			if val.Delegate {
				// validate the addition to the delegated pool
				require.Equal(t, test.msg.Amount, supply.Delegated)
				// validate the addition to the delegations only
				for _, id := range val.Committees {
					// get the supply for each committee
					stakedSupply, e := sm.GetDelegateStakedSupply(id)
					require.NoError(t, e)
					require.Equal(t, test.msg.Amount, stakedSupply.Amount)
					// validate the delegate membership
					page, e := sm.GetDelegatesPaginated(lib.PageParams{}, id)
					require.NoError(t, e)
					// extract the list from the page
					list := (page.Results).(*types.ValidatorPage)
					// ensure the list count is correct
					require.Len(t, *list, 1)
					// ensure the expected validator is a member
					require.EqualExportedValues(t, test.expected, (*list)[0])
				}
			} else {
				for _, id := range val.Committees {
					// validate the committee membership
					page, e := sm.GetCommitteePaginated(lib.PageParams{}, id)
					require.NoError(t, e)
					// extract the list from the page
					list := (page.Results).(*types.ValidatorPage)
					// ensure the list count is correct
					require.Len(t, *list, 1)
					// ensure the expected validator is a member
					require.EqualExportedValues(t, test.expected, (*list)[0])
				}
			}
		})
	}
}

func TestHandleMessageEditStake(t *testing.T) {
	// predefine a function that calculates the differences between two uint64 slices
	difference := func(a, b []uint64) []uint64 {
		y := make(map[uint64]struct{}, len(b))
		for _, x := range b {
			y[x] = struct{}{}
		}
		var diff []uint64
		for _, x := range a {
			if _, found := y[x]; !found {
				diff = append(diff, x)
			}
		}
		return diff
	}
	tests := []struct {
		name              string
		detail            string
		presetSender      uint64
		presetValidator   *types.Validator
		msg               *types.MessageEditStake
		expectedValidator *types.Validator
		expectedSupply    *types.Supply
		error             string
	}{
		{
			name:   "validator doesn't exist",
			detail: "validator does not exist to edit it",
			msg:    &types.MessageEditStake{Address: newTestAddressBytes(t)},
			error:  "validator does not exist",
		},
		{
			name:   "unstaking",
			detail: "the validator is unstaking and cannot be edited",
			presetValidator: &types.Validator{
				Address:         newTestAddressBytes(t),
				UnstakingHeight: 1,
			},
			msg:   &types.MessageEditStake{Address: newTestAddressBytes(t)},
			error: "unstaking",
		},
		{
			name:   "invalid amount",
			detail: "the sender attempts to lower the stake by edit-stake",
			presetValidator: &types.Validator{
				Address:      newTestAddressBytes(t),
				StakedAmount: 2,
			},
			msg: &types.MessageEditStake{
				Address: newTestAddressBytes(t),
				Amount:  1,
			},
			error: "amount is invalid",
		},
		{
			name:   "insufficient funds",
			detail: "the sender doesn't have enough funds to complete the edit stake",
			presetValidator: &types.Validator{
				Address:      newTestAddressBytes(t),
				PublicKey:    newTestPublicKeyBytes(t),
				NetAddress:   "http://example.com",
				StakedAmount: 1,
				Committees:   []uint64{0, 1},
				Output:       newTestAddressBytes(t),
				Compound:     true,
			},
			msg: &types.MessageEditStake{
				Address:       newTestAddressBytes(t),
				Amount:        2,
				Committees:    []uint64{0, 1},
				NetAddress:    "http://example.com",
				OutputAddress: newTestAddressBytes(t),
				Compound:      true,
			},
			error: "insufficient funds",
		},
		{
			name:   "edit stake, same balance, same committees",
			detail: "the validator is updated but the balance and committees remains the same",
			presetValidator: &types.Validator{
				Address:      newTestAddressBytes(t),
				PublicKey:    newTestPublicKeyBytes(t),
				NetAddress:   "http://example.com",
				StakedAmount: 1,
				Committees:   []uint64{0, 1},
				Output:       newTestAddressBytes(t),
				Compound:     true,
			},
			msg: &types.MessageEditStake{
				Address:       newTestAddressBytes(t),
				Amount:        1,
				Committees:    []uint64{0, 1},
				NetAddress:    "http://example2.com",
				OutputAddress: newTestAddressBytes(t, 1),
				Delegate:      true, // can't be edited
				Compound:      false,
			},
			expectedValidator: &types.Validator{
				Address:      newTestAddressBytes(t),
				PublicKey:    newTestPublicKeyBytes(t),
				NetAddress:   "http://example2.com",
				StakedAmount: 1,
				Committees:   []uint64{0, 1},
				Output:       newTestAddressBytes(t, 1),
				Compound:     false,
			},
			expectedSupply: &types.Supply{
				Total:  1,
				Staked: 1,
				CommitteesWithDelegations: []*types.Pool{
					{
						Id:     1,
						Amount: 1,
					},
					{
						Id:     0,
						Amount: 1,
					},
				},
			},
		},
		{
			name:   "edit stake, same balance, same delegations",
			detail: "the validator is updated but the balance and delegations remains the same",
			presetValidator: &types.Validator{
				Address:      newTestAddressBytes(t),
				StakedAmount: 1,
				Committees:   []uint64{0, 1},
				Output:       newTestAddressBytes(t),
				Delegate:     true,
			},
			msg: &types.MessageEditStake{
				Address:    newTestAddressBytes(t),
				Amount:     1,
				Committees: []uint64{0, 1},
			},
			expectedValidator: &types.Validator{
				Address:      newTestAddressBytes(t),
				StakedAmount: 1,
				Committees:   []uint64{0, 1},
				Delegate:     true,
			},
			expectedSupply: &types.Supply{
				Total:     1,
				Staked:    1,
				Delegated: 1,
				CommitteesWithDelegations: []*types.Pool{
					{
						Id:     1,
						Amount: 1,
					},
					{
						Id:     0,
						Amount: 1,
					},
				},
				DelegationsOnly: []*types.Pool{
					{
						Id:     1,
						Amount: 1,
					},
					{
						Id:     0,
						Amount: 1,
					},
				},
			},
		},
		{
			name:   "edit stake, same balance, different committees",
			detail: "the validator is updated with different committees but the balance remains the same",
			presetValidator: &types.Validator{
				Address:      newTestAddressBytes(t),
				PublicKey:    newTestPublicKeyBytes(t),
				NetAddress:   "http://example.com",
				StakedAmount: 1,
				Committees:   []uint64{0, 1},
				Output:       newTestAddressBytes(t),
				Compound:     true,
			},
			msg: &types.MessageEditStake{
				Address:       newTestAddressBytes(t),
				Amount:        1,
				Committees:    []uint64{1, 2, 3},
				NetAddress:    "http://example.com",
				OutputAddress: newTestAddressBytes(t),
				Compound:      true,
			},
			expectedValidator: &types.Validator{
				Address:      newTestAddressBytes(t),
				PublicKey:    newTestPublicKeyBytes(t),
				NetAddress:   "http://example.com",
				StakedAmount: 1,
				Committees:   []uint64{1, 2, 3},
				Output:       newTestAddressBytes(t),
				Compound:     true,
			},
			expectedSupply: &types.Supply{
				Total:  1,
				Staked: 1,
				CommitteesWithDelegations: []*types.Pool{
					{
						Id:     3,
						Amount: 1,
					},
					{
						Id:     1,
						Amount: 1,
					},
					{
						Id:     2,
						Amount: 1,
					},
				},
			},
		},
		{
			name:   "edit stake, same balance, different delegations",
			detail: "the validator is updated with different delegations but the balance remains the same",
			presetValidator: &types.Validator{
				Address:      newTestAddressBytes(t),
				StakedAmount: 1,
				Committees:   []uint64{0, 1},
				Delegate:     true,
			},
			msg: &types.MessageEditStake{
				Address:    newTestAddressBytes(t),
				Amount:     1,
				Committees: []uint64{1, 2, 3},
				Delegate:   true,
			},
			expectedValidator: &types.Validator{
				Address:      newTestAddressBytes(t),
				StakedAmount: 1,
				Committees:   []uint64{1, 2, 3},
				Delegate:     true,
			},
			expectedSupply: &types.Supply{
				Total:     1,
				Staked:    1,
				Delegated: 1,
				CommitteesWithDelegations: []*types.Pool{
					{
						Id:     3,
						Amount: 1,
					},
					{
						Id:     1,
						Amount: 1,
					},
					{
						Id:     2,
						Amount: 1,
					},
				},
				DelegationsOnly: []*types.Pool{
					{
						Id:     3,
						Amount: 1,
					},
					{
						Id:     1,
						Amount: 1,
					},
					{
						Id:     2,
						Amount: 1,
					},
				},
			},
		},
		{
			name:         "edit stake, different balance, different committees",
			detail:       "the validator is updated with different committees and balance",
			presetSender: 2,
			presetValidator: &types.Validator{
				Address:      newTestAddressBytes(t),
				StakedAmount: 1,
				Committees:   []uint64{0, 1},
			},
			msg: &types.MessageEditStake{
				Address:    newTestAddressBytes(t),
				Amount:     2,
				Committees: []uint64{1, 2, 3},
			},
			expectedValidator: &types.Validator{
				Address:      newTestAddressBytes(t),
				StakedAmount: 2,
				Committees:   []uint64{1, 2, 3},
			},
			expectedSupply: &types.Supply{
				Total:  3,
				Staked: 2,
				CommitteesWithDelegations: []*types.Pool{
					{
						Id:     3,
						Amount: 2,
					},
					{
						Id:     1,
						Amount: 2,
					},
					{
						Id:     2,
						Amount: 2,
					},
				},
			},
		},
		{
			name:         "edit stake, different balance, different delegations",
			detail:       "the validator is updated with different delegations and balance",
			presetSender: 2,
			presetValidator: &types.Validator{
				Address:      newTestAddressBytes(t),
				StakedAmount: 1,
				Committees:   []uint64{0, 1},
				Delegate:     true,
			},
			msg: &types.MessageEditStake{
				Address:    newTestAddressBytes(t),
				Amount:     2,
				Committees: []uint64{1, 2, 3},
				Delegate:   true,
			},
			expectedValidator: &types.Validator{
				Address:      newTestAddressBytes(t),
				StakedAmount: 2,
				Committees:   []uint64{1, 2, 3},
				Delegate:     true,
			},
			expectedSupply: &types.Supply{
				Total:     3,
				Staked:    2,
				Delegated: 2,
				CommitteesWithDelegations: []*types.Pool{
					{
						Id:     3,
						Amount: 2,
					},
					{
						Id:     1,
						Amount: 2,
					},
					{
						Id:     2,
						Amount: 2,
					},
				},
				DelegationsOnly: []*types.Pool{
					{
						Id:     3,
						Amount: 2,
					},
					{
						Id:     1,
						Amount: 2,
					},
					{
						Id:     2,
						Amount: 2,
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var sender crypto.AddressI
			// create a state machine instance with default parameters
			sm := newTestStateMachine(t)
			// create sender address object
			sender = crypto.NewAddress(test.msg.Address)
			// preset the accounts with some funds
			require.NoError(t, sm.AccountAdd(sender, test.presetSender))
			// preset the validator
			if test.presetValidator != nil {
				supply := &types.Supply{}
				require.NoError(t, sm.SetValidators([]*types.Validator{test.presetValidator}, supply))
				supply.Total = test.presetSender + test.presetValidator.StakedAmount
				require.NoError(t, sm.SetSupply(supply))
			}
			// execute the function call
			err := sm.HandleMessageEditStake(test.msg)
			// validate the expected error
			require.Equal(t, test.error != "", err != nil, err)
			if err != nil {
				require.ErrorContains(t, err, test.error)
				return
			}
			// validate the account
			got, err := sm.GetAccount(sender)
			require.NoError(t, err)
			require.Equal(t, test.presetSender-(test.msg.Amount-test.presetValidator.StakedAmount), got.Amount)
			// validate the update of the validator object
			val, err := sm.GetValidator(sender)
			require.NoError(t, err)
			// compare got vs expected
			require.EqualExportedValues(t, test.expectedValidator, val)
			// get the supply object
			supply, err := sm.GetSupply()
			require.NoError(t, err)
			// validate the update to the supply
			require.EqualExportedValues(t, test.expectedSupply, supply)
			// calculate differences between before and after committees
			nonMembershipCommittees := difference(test.presetValidator.Committees, val.Committees)
			if val.Delegate {
				for _, id := range val.Committees {
					// validate the delegate membership
					page, e := sm.GetDelegatesPaginated(lib.PageParams{}, id)
					require.NoError(t, e)
					// extract the list from the page
					list := (page.Results).(*types.ValidatorPage)
					// ensure the list count is correct
					require.Len(t, *list, 1)
					// ensure the expected validator is a member
					require.EqualExportedValues(t, test.expectedValidator, (*list)[0])
				}
				for _, id := range nonMembershipCommittees {
					// validate the delegate non-membership
					page, e := sm.GetDelegatesPaginated(lib.PageParams{}, id)
					require.NoError(t, e)
					// extract the list from the page
					list := (page.Results).(*types.ValidatorPage)
					// ensure the non membership
					require.Len(t, *list, 0)
				}
			} else {
				for _, id := range val.Committees {
					// validate the committee membership
					page, e := sm.GetCommitteePaginated(lib.PageParams{}, id)
					require.NoError(t, e)
					// extract the list from the page
					list := (page.Results).(*types.ValidatorPage)
					// ensure the list count is correct
					require.Len(t, *list, 1)
					// ensure the expected validator is a member
					require.EqualExportedValues(t, test.expectedValidator, (*list)[0])
				}
				for _, id := range nonMembershipCommittees {
					// validate the committee non-membership
					page, e := sm.GetCommitteePaginated(lib.PageParams{}, id)
					require.NoError(t, e)
					// extract the list from the page
					list := (page.Results).(*types.ValidatorPage)
					// ensure the non membership
					require.Len(t, *list, 0)
				}
			}
		})
	}
}
