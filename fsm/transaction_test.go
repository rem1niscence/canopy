package fsm

import (
	"github.com/canopy-network/canopy/fsm/types"
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"
	"testing"
	"time"
)

func TestApplyTransaction(t *testing.T) {
	const amount = uint64(100)
	// predefine a keygroup for signing the transaction
	kg := newTestKeyGroup(t)
	// predefine a send-transaction to insert into the block
	sendTx, e := types.NewSendTransaction(kg.PrivateKey, newTestAddress(t), amount-1, 1, 1, 1, "")
	require.NoError(t, e)
	tests := []struct {
		name          string
		detail        string
		transaction   lib.TransactionI
		presetSender  uint64
		lastBlockTime time.Time
		expected      *lib.TxResult
		error         string
	}{
		{
			name:          "deduct fee fails",
			detail:        "failure on fee deduction",
			lastBlockTime: time.Now(),
			transaction:   sendTx,
			expected:      &lib.TxResult{},
			error:         "insufficient funds",
		},
		{
			name:          "handle message fails",
			detail:        "failure on send",
			lastBlockTime: time.Now(),
			presetSender:  amount - 1,
			transaction:   sendTx,
			expected:      &lib.TxResult{},
			error:         "insufficient funds",
		},
		{
			name:          "valid send tx",
			detail:        "happy path of the transaction being applied",
			lastBlockTime: time.Now(),
			presetSender:  amount,
			transaction:   sendTx,
			expected: &lib.TxResult{
				Sender:      newTestAddressBytes(t),
				Recipient:   newTestAddressBytes(t),
				MessageType: "send",
				Height:      2,
				Index:       0,
				Transaction: sendTx.(*lib.Transaction),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a state machine instance with default parameters
			sm := newTestStateMachine(t)
			// convenience variable for store
			s := sm.store.(lib.StoreI)
			// convert the transaction to bytes
			tx, err := lib.Marshal(test.transaction)
			require.NoError(t, err)
			// set transaction hash in the expected object
			if test.expected != nil {
				test.expected.TxHash = crypto.HashString(tx)
			}
			// preset the state limit for send fee
			require.NoError(t, sm.UpdateParam("fee", types.ParamMessageSendFee, &lib.UInt64Wrapper{Value: 1}))
			// preset tokens to the sender account (for the fee)
			require.NoError(t, sm.AccountAdd(newTestAddress(t), test.presetSender))
			// preset last block for timestamp verification
			require.NoError(t, s.IndexBlock(&lib.BlockResult{
				BlockHeader: &lib.BlockHeader{
					Height: 1,
					Hash:   crypto.Hash([]byte("block_hash")),
					Time:   uint64(test.lastBlockTime.UnixMicro()),
				},
			}))
			// execute the function call
			got, err := sm.ApplyTransaction(0, tx, test.expected.TxHash)
			// validate the expected error
			require.Equal(t, test.error != "", err != nil, err)
			if err != nil {
				require.ErrorContains(t, err, test.error)
				return
			}
			// compare got vs expected
			require.EqualExportedValues(t, test.expected, got)
		})
	}
}

func TestCheckTx(t *testing.T) {
	const amount = uint64(100)
	// predefine a keygroup for signing the transaction
	kg := newTestKeyGroup(t)
	// predefine a send-transaction to insert into the block
	sendTx, e := types.NewSendTransaction(kg.PrivateKey, newTestAddress(t), amount-1, 1, 1, 1, "")
	require.NoError(t, e)
	// convert the object to bytes
	tx, e := lib.Marshal(sendTx)
	require.NoError(t, e)
	// define a version with a bad fee (below state limit)
	sendTxBadFee := sendTx.(*lib.Transaction)
	sendTxBadFee.Fee = 0
	require.NoError(t, sendTxBadFee.Sign(kg.PrivateKey))
	// convert the object to bytes
	txBadFee, e := lib.Marshal(sendTxBadFee)
	require.NoError(t, e)
	// define a version without a bad signature
	sendTxBadSig := sendTx.(*lib.Transaction)
	sendTxBadSig.Signature.Signature = []byte("bad sig")
	// convert the object to bytes
	txBadSig, e := lib.Marshal(sendTxBadSig)
	require.NoError(t, e)
	// define test cases
	tests := []struct {
		name          string
		detail        string
		tx            []byte
		presetSender  uint64
		lastBlockTime time.Time
		expected      *CheckTxResult
		error         string
	}{
		{
			name:   "unmarshal fails",
			detail: "failure on converting the bytes to a tx object",
			tx:     []byte("not a proto msg"),
			error:  "unmarshal",
		},
		{
			name:   "tx.check() fails",
			detail: "failure on stateless transaction checking",
			error:  "message is empty",
		},
		{
			name:   "tx timestamp fails",
			detail: "failure on transaction timestamp",
			tx:     tx,
			error:  "invalid tx timestamp",
		},
		{
			name:          "tx signature verification fails",
			detail:        "failure on transaction signature verification",
			lastBlockTime: time.Now(),
			tx:            txBadSig,
			error:         "invalid signature",
		},
		{
			name:          "tx fee check fails",
			detail:        "failure on transaction fee checking",
			lastBlockTime: time.Now(),
			tx:            txBadFee,
			error:         "below state limit",
		},
		{
			name:          "passes check tx",
			detail:        "the happy path of check tx",
			lastBlockTime: time.Now(),
			tx:            tx,
			expected: &CheckTxResult{
				tx: sendTx.(*lib.Transaction),
				msg: &types.MessageSend{
					FromAddress: newTestAddressBytes(t),
					ToAddress:   newTestAddressBytes(t),
					Amount:      amount,
				},
				sender: newTestAddress(t),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a state machine instance with default parameters
			sm := newTestStateMachine(t)
			// convenience variable for store
			s := sm.store.(lib.StoreI)
			// preset the state limit for send fee
			require.NoError(t, sm.UpdateParam("fee", types.ParamMessageSendFee, &lib.UInt64Wrapper{Value: 1}))
			// preset tokens to the sender account (for the fee)
			require.NoError(t, sm.AccountAdd(newTestAddress(t), test.presetSender))
			// preset last block for timestamp verification
			require.NoError(t, s.IndexBlock(&lib.BlockResult{
				BlockHeader: &lib.BlockHeader{
					Height: 1,
					Hash:   crypto.Hash([]byte("block_hash")),
					Time:   uint64(test.lastBlockTime.UnixMicro()),
				},
			}))
			// execute the function call
			got, err := sm.CheckTx(test.tx, crypto.HashString(test.tx))
			// validate the expected error
			require.Equal(t, test.error != "", err != nil, err)
			if err != nil {
				require.ErrorContains(t, err, test.error)
				return
			}
			// compare got vs expected
			require.EqualExportedValues(t, test.expected, got)
		})
	}
}

func TestCheckSignature(t *testing.T) {
	const amount = uint64(100)
	// predefine a keygroup for signing the transaction
	kg := newTestKeyGroup(t)
	// predefine a send message
	msg := &types.MessageSend{
		FromAddress: newTestAddressBytes(t),
		ToAddress:   newTestAddressBytes(t),
		Amount:      amount,
	}
	// predefine send message with a different signer
	msg2 := &types.MessageSend{
		FromAddress: newTestAddressBytes(t, 2),
		ToAddress:   newTestAddressBytes(t),
		Amount:      amount,
	}
	// convert the message to 'any' for transaction wrapping
	a, e := lib.NewAny(msg)
	require.NoError(t, e)
	// convert the message to 'any' for transaction wrapping
	a2, e := lib.NewAny(msg2)
	require.NoError(t, e)
	// define a transaction object to sign it
	tx := &lib.Transaction{
		MessageType: msg.Name(),
		Msg:         a,
		Time:        uint64(time.Now().UnixMicro()),
		Fee:         1,
	}
	// define a second transaction object to sign
	tx2 := &lib.Transaction{
		MessageType: msg2.Name(),
		Msg:         a2,
		Time:        uint64(time.Now().UnixMicro()),
		Fee:         1,
	}
	// sign the transaction to use in testing
	require.NoError(t, tx.Sign(kg.PrivateKey))
	// sign the second transaction to use in testing
	require.NoError(t, tx2.Sign(kg.PrivateKey))
	// define test cases
	tests := []struct {
		name           string
		detail         string
		transaction    *lib.Transaction
		msg            lib.MessageI
		expectedSigner crypto.AddressI
		error          string
	}{
		{
			name:   "empty signature",
			detail: "the function call errors due to an empty signature",
			transaction: &lib.Transaction{
				MessageType: msg.Name(),
				Msg:         a,
				Time:        uint64(time.Now().UnixMicro()),
				Fee:         1,
			},
			msg:   msg,
			error: "empty signature",
		},
		{
			name:   "bad public key",
			detail: "the function call errors due to a bad signature public key",
			transaction: &lib.Transaction{
				MessageType: msg.Name(),
				Msg:         a,
				Signature: &lib.Signature{
					PublicKey: newTestAddressBytes(t),
					Signature: crypto.Hash([]byte("some_signature")),
				},
				Time: uint64(time.Now().UnixMicro()),
				Fee:  1,
			},
			msg:   msg,
			error: "public key is invalid",
		},
		{
			name:   "bad signature verification",
			detail: "the function call errors due to a bad signature verification",
			transaction: &lib.Transaction{
				MessageType: msg.Name(),
				Msg:         a,
				Signature: &lib.Signature{
					PublicKey: newTestPublicKeyBytes(t),
					Signature: crypto.Hash([]byte("some_signature")),
				},
				Time: uint64(time.Now().UnixMicro()),
				Fee:  1,
			},
			msg:   msg,
			error: "invalid signature",
		},
		{
			name:        "unauthorized signer",
			detail:      "the function call errors due to an unauthorized signer",
			transaction: tx2,
			msg:         msg2,
			error:       "unauthorized",
		},
		{
			name:           "valid signature",
			detail:         "the function call errors due to an unauthorized signer",
			transaction:    tx,
			msg:            msg,
			expectedSigner: newTestAddress(t),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a state machine instance with default parameters
			sm := newTestStateMachine(t)
			// execute the function call
			signer, err := sm.CheckSignature(test.msg, test.transaction)
			// validate the expected error
			require.Equal(t, test.error != "", err != nil, err)
			if err != nil {
				require.ErrorContains(t, err, test.error)
				return
			}
			// validate got vs expected signer
			require.Equal(t, test.expectedSigner, signer)
		})
	}
}

func TestCheckReplay(t *testing.T) {
	start := time.Now()
	tests := []struct {
		name          string
		detail        string
		height        uint64
		tx            *lib.Transaction
		lastBlockTime time.Time
		error         string
	}{
		{
			name:   "bad network id",
			detail: "the network id is incorrect",
			tx: &lib.Transaction{
				NetworkId: 2,
				ChainId:   1,
			},
			error: "wrong network id",
		},
		{
			name:   "bad chain id",
			detail: "the chain id is incorrect",
			tx: &lib.Transaction{
				NetworkId: 1,
				ChainId:   2,
			},
			error: "wrong committee id",
		},
		{
			name:   "before height 2",
			detail: "before height 2 so timestamps are ignored",
			tx: &lib.Transaction{
				NetworkId: 1,
				ChainId:   1,
			},
		},
		{
			name:   "above maximum time",
			detail: "above maximum timestamp should fail",
			tx: &lib.Transaction{
				Time:      uint64(start.Add(time.Hour*6 + 1000).UnixMicro()),
				NetworkId: 1,
				ChainId:   1,
			},
			height:        2,
			lastBlockTime: start,
			error:         "invalid tx timestamp",
		},
		{
			name:   "below minimum time",
			detail: "below minimum timestamp should fail",
			tx: &lib.Transaction{
				Time:      uint64(start.UnixMicro()),
				NetworkId: 1,
				ChainId:   1,
			},
			height:        2,
			lastBlockTime: start.Add(-time.Hour*6 - 1000),
			error:         "invalid tx timestamp",
		},
		{
			name:   "maximum time",
			detail: "maximum timestamp should succeed",
			tx: &lib.Transaction{
				Time:      uint64(start.UnixMicro()),
				NetworkId: 1,
				ChainId:   1,
			},
			height:        2,
			lastBlockTime: start.Add(time.Hour * 6),
		},
		{
			name:   "minimum time",
			detail: "minimum timestamp should succeed",
			tx: &lib.Transaction{
				Time:      uint64(start.UnixMicro()),
				NetworkId: 1,
				ChainId:   1,
			},
			height:        2,
			lastBlockTime: start.Add(-time.Hour * 6),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a state machine instance with default parameters
			sm := newTestStateMachine(t)
			// set sm height
			sm.height = test.height
			// convenience variable for store
			s := sm.store.(lib.StoreI)
			// preset last block for timestamp verification
			require.NoError(t, s.IndexBlock(&lib.BlockResult{
				BlockHeader: &lib.BlockHeader{
					Height: 1,
					Hash:   crypto.Hash([]byte("block_hash")),
					Time:   uint64(test.lastBlockTime.UnixMicro()),
				},
			}))
			// execute the function call
			err := sm.CheckReplay(test.tx, crypto.HashString([]byte("hash")))
			// validate the expected error
			require.Equal(t, test.error != "", err != nil, err)
			if err != nil {
				require.ErrorContains(t, err, test.error)
				return
			}
		})
	}
}

func TestCheckMessage(t *testing.T) {
	// predefine non message any
	nonTxAny, e := lib.NewAny(&lib.UInt64Wrapper{})
	require.NoError(t, e)
	// predefine a send message
	invalidSend := &types.MessageSend{
		FromAddress: newTestAddressBytes(t),
	}
	// convert the message to 'any' for transaction wrapping
	invalidMsgSendAny, e := lib.NewAny(invalidSend)
	require.NoError(t, e)
	// predefine a send message
	sendMsg := &types.MessageSend{
		FromAddress: newTestAddressBytes(t),
		ToAddress:   newTestAddressBytes(t),
		Amount:      100,
	}
	// convert the message to 'any' for transaction wrapping
	msgSendAny, e := lib.NewAny(sendMsg)
	require.NoError(t, e)
	tests := []struct {
		name     string
		detail   string
		msg      *anypb.Any
		expected lib.MessageI
		error    string
	}{
		{
			name:   "non message any",
			detail: "a non message any will fail",
			msg:    nonTxAny,
			error:  "invalid transaction message",
		},
		{
			name:   "check() invalid message",
			detail: "a invalid message that fails check()",
			msg:    invalidMsgSendAny,
			error:  "recipient address is empty",
		},
		{
			name:     "valid message",
			detail:   "a valid message passes",
			msg:      msgSendAny,
			expected: sendMsg,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a state machine instance with default parameters
			sm := newTestStateMachine(t)
			// execute the function call
			got, err := sm.CheckMessage(test.msg)
			// validate the expected error
			require.Equal(t, test.error != "", err != nil, err)
			if err != nil {
				require.ErrorContains(t, err, test.error)
				return
			}
			// validate got vs expected
			require.EqualExportedValues(t, test.expected, got)
		})
	}
}

func TestCheckFee(t *testing.T) {
	tests := []struct {
		name       string
		detail     string
		stateLimit uint64
		fee        uint64
		msg        lib.MessageI
		error      string
	}{
		{
			name:       "fee < minimum",
			detail:     "the fee is less than the parameter",
			stateLimit: 2,
			fee:        1,
			msg:        &types.MessageSend{},
			error:      "below state limit",
		},
		{
			name:       "fee = minimum",
			detail:     "the fee is equal to the parameter",
			stateLimit: 2,
			fee:        2,
			msg:        &types.MessageSend{},
		},
		{
			name:       "fee > minimum",
			detail:     "the fee is greater than the parameter",
			stateLimit: 2,
			fee:        3,
			msg:        &types.MessageSend{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a state machine instance with default parameters
			sm := newTestStateMachine(t)
			// preset the state limit
			require.NoError(t, sm.UpdateParam("fee", types.ParamMessageSendFee, &lib.UInt64Wrapper{Value: test.stateLimit}))
			// execute the function call
			err := sm.CheckFee(test.fee, test.msg)
			// validate the expected error
			require.Equal(t, test.error != "", err != nil, err)
			if err != nil {
				require.ErrorContains(t, err, test.error)
				return
			}
		})
	}
}
