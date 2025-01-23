package lib

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/canopy-network/canopy/lib/crypto"
	"github.com/stretchr/testify/require"
)

func TestTransactionCheckBasic(t *testing.T) {
	// pre-define a test message
	sig := &Signature{
		PublicKey: newTestPublicKeyBytes(t),
		Signature: newTestPublicKeyBytes(t),
	}
	// pre-define an any for testing
	a, e := NewAny(sig)
	require.NoError(t, e)
	// define test cases
	tests := []struct {
		name        string
		detail      string
		transaction *Transaction
		error       string
	}{
		{
			name:        "nil transaction",
			detail:      "a nil or empty transaction",
			transaction: nil,
			error:       "transaction is empty",
		},
		{
			name:   "nil message",
			detail: "a nil or empty message",
			transaction: &Transaction{
				MessageType: "",
				Msg:         nil,
				Signature:   nil,
				Time:        0,
				Fee:         0,
				Memo:        "",
			},
			error: "message is empty",
		},
		{
			name:   "empty signature",
			detail: "the signature is empty",
			transaction: &Transaction{
				MessageType: testMessageName,
				Msg:         a,
				Signature:   nil,
				Time:        0,
				Fee:         0,
				Memo:        "",
			},
			error: "signature is empty",
		},
		{
			name:   "empty signature",
			detail: "the signature is empty",
			transaction: &Transaction{
				MessageType: testMessageName,
				Msg:         a,
				Signature:   sig,
				Time:        0,
				Fee:         0,
				Memo:        "",
			},
			error: "invalid tx time",
		},
		{
			name:   "memo is invalid",
			detail: "the memo is too long",
			transaction: &Transaction{
				MessageType: testMessageName,
				Msg:         a,
				Signature:   sig,
				Time:        uint64(time.Now().UnixMicro()),
				Fee:         0,
				Memo:        strings.Repeat("F", 201),
			},
			error: "invalid memo",
		},
		{
			name:   "no error",
			detail: "the transaction is valid",
			transaction: &Transaction{
				MessageType: testMessageName,
				Msg:         a,
				Signature:   sig,
				Time:        uint64(time.Now().UnixMicro()),
				Fee:         0,
				Memo:        "",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// execute function call
			err := test.transaction.CheckBasic()
			// validate if an error is expected
			require.Equal(t, err != nil, test.error != "", err)
			// validate actual error if any
			if err != nil {
				require.ErrorContains(t, err, test.error, err)
				return
			}
		})
	}
}

func TestGetHash(t *testing.T) {
	// pre-define a test message
	sig := &Signature{
		PublicKey: newTestPublicKeyBytes(t),
		Signature: newTestPublicKeyBytes(t),
	}
	// pre-define an any for testing
	a, e := NewAny(sig)
	require.NoError(t, e)
	// pre-define a transaction
	tx := &Transaction{
		MessageType: testMessageName,
		Msg:         a,
		Signature:   sig,
		Time:        uint64(time.Now().UnixMicro()),
		Fee:         1,
		Memo:        "memo",
	}
	// calculate expected
	bz, err := Marshal(tx)
	require.NoError(t, err)
	expected := crypto.Hash(bz)
	// execute function call
	got, err := tx.GetHash()
	require.NoError(t, err)
	// compare got vs expected
	require.Equal(t, expected, got)
}

func TestGetSignBytes(t *testing.T) {
	// pre-define a test message
	sig := &Signature{
		PublicKey: newTestPublicKeyBytes(t),
		Signature: newTestPublicKeyBytes(t),
	}
	// pre-define an any for testing
	a, e := NewAny(sig)
	require.NoError(t, e)
	// pre-define a transaction
	tx := &Transaction{
		MessageType: testMessageName,
		Msg:         a,
		Signature:   sig,
		Time:        uint64(time.Now().UnixMicro()),
		Fee:         1,
		Memo:        "memo",
	}
	// calculate expected
	expected, err := Marshal(&Transaction{
		MessageType: tx.MessageType,
		Msg:         tx.Msg,
		Time:        tx.Time,
		Fee:         tx.Fee,
		Memo:        tx.Memo,
	})
	require.NoError(t, err)
	// execute function call
	got, err := tx.GetSignBytes()
	require.NoError(t, err)
	// compare got vs expected
	require.Equal(t, expected, got)
}

func TestSign(t *testing.T) {
	// pre-define a private key
	pk, err := crypto.NewBLS12381PrivateKey()
	require.NoError(t, err)
	// pre-define a test message
	sig := &Signature{
		PublicKey: newTestPublicKeyBytes(t),
		Signature: newTestPublicKeyBytes(t),
	}
	// pre-define an any for testing
	a, e := NewAny(sig)
	require.NoError(t, e)
	// pre-define a transaction
	tx := &Transaction{
		MessageType: testMessageName,
		Msg:         a,
		Signature:   sig,
		Time:        uint64(time.Now().UnixMicro()),
		Fee:         1,
		Memo:        "memo",
	}
	// get sign bytes
	bz, err := tx.GetSignBytes()
	require.NoError(t, err)
	// calculate expected
	expected := &Signature{
		PublicKey: pk.PublicKey().Bytes(),
		Signature: pk.Sign(bz),
	}
	// execute function call
	require.NoError(t, tx.Sign(pk))
	// compare got vs expected
	require.EqualExportedValues(t, expected, tx.Signature)
}

func TestTransactionJSON(t *testing.T) {
	// pre-define a test message
	sig := &Signature{
		PublicKey: newTestPublicKeyBytes(t),
		Signature: newTestPublicKeyBytes(t),
	}
	// pre-define an any for testing
	a, e := NewAny(sig)
	require.NoError(t, e)
	// pre-define a transaction
	expected := &Transaction{
		MessageType: testMessageName,
		Msg:         a,
		Signature:   sig,
		Time:        uint64(time.Now().UnixMicro()),
		Fee:         1,
		Memo:        "memo",
	}
	// convert structure to json bytes
	gotBytes, err := json.Marshal(expected)
	require.NoError(t, err)
	// convert bytes to structure
	got := new(Transaction)
	// unmarshal into bytes
	require.NoError(t, json.Unmarshal(gotBytes, got))
	// hardcode time as we lose precision upon conversion
	got.Time = expected.Time
	// compare got vs expected
	require.EqualExportedValues(t, expected, got)
}

func TestTransactionResultJSON(t *testing.T) {
	// pre-define a test message
	sig := &Signature{
		PublicKey: newTestPublicKeyBytes(t),
		Signature: newTestPublicKeyBytes(t),
	}
	// pre-define an any for testing
	a, e := NewAny(sig)
	require.NoError(t, e)
	// pre-define a transaction
	expected := &TxResult{
		Sender:      newTestAddressBytes(t),
		Recipient:   newTestAddressBytes(t),
		MessageType: testMessageName,
		Height:      1,
		Index:       2,
		Transaction: &Transaction{
			MessageType: testMessageName,
			Msg:         a,
			Signature:   sig,
			Time:        uint64(time.Now().UnixMicro()),
			Fee:         1,
			Memo:        "memo",
		},
		TxHash: crypto.HashString([]byte("hash")),
	}
	// convert structure to json bytes
	gotBytes, err := json.Marshal(expected)
	require.NoError(t, err)
	// convert bytes to structure
	got := new(TxResult)
	// unmarshal into bytes
	require.NoError(t, json.Unmarshal(gotBytes, got))
	// hardcode time as we lose precision upon conversion
	got.Transaction.Time = expected.Transaction.Time
	// compare got vs expected
	require.EqualExportedValues(t, expected, got)
}

func TestSignatureJSON(t *testing.T) {
	// pre-define a signature
	expected := &Signature{
		PublicKey: newTestPublicKeyBytes(t),
		Signature: newTestPublicKeyBytes(t),
	}
	// convert structure to json bytes
	gotBytes, err := json.Marshal(expected)
	require.NoError(t, err)
	// convert bytes to structure
	got := new(Signature)
	// unmarshal into bytes
	require.NoError(t, json.Unmarshal(gotBytes, got))
	// compare got vs expected
	require.EqualExportedValues(t, expected, got)
}

// define a test message type to use in this test file

var _ MessageI = &Signature{}

const testMessageName = "signature"

func init() {
	RegisteredMessages[testMessageName] = &Signature{}
}

func (x *Signature) New() MessageI     { return &Signature{} }
func (x *Signature) Name() string      { return testMessageName }
func (x *Signature) Check() ErrorI     { return nil }
func (x *Signature) Recipient() []byte { return nil }

func TestFailedTxCache(t *testing.T) {
	// pre-define a test message
	sig := &Signature{
		PublicKey: newTestPublicKeyBytes(t),
		Signature: newTestPublicKeyBytes(t),
	}
	// pre-define an any for testing
	a, e := NewAny(sig)
	require.NoError(t, e)
	// pre-define a transaction
	tx := &Transaction{
		MessageType: testMessageName,
		Msg:         a,
		Signature:   sig,
		Time:        uint64(time.Now().UnixMicro()),
		Fee:         1,
		Memo:        "memo",
	}
	// marshal transaction to bytes
	txBytes, err := Marshal(tx)
	require.NoError(t, err)

	// define test cases
	tests := []struct {
		name                string
		allowedMessageTypes []string
		txBytes             []byte
		hash                string
		err                 string
		expectedResult      bool
	}{
		{
			name:                "valid transaction",
			allowedMessageTypes: []string{testMessageName},
			txBytes:             txBytes,
			hash:                "validHash",
			err:                 "some error",
			expectedResult:      true,
		},
		{
			name:                "invalid message type",
			allowedMessageTypes: []string{"invalidMessageType"},
			txBytes:             txBytes,
			hash:                "invalidHash",
			err:                 "some error",
			expectedResult:      false,
		},
		{
			name:                "unmarshal error",
			allowedMessageTypes: []string{testMessageName},
			txBytes:             []byte("invalidBytes"),
			hash:                "unmarshalErrorHash",
			err:                 "some error",
			expectedResult:      false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create a new failed tx cache
			cache := NewFailedTxCache(test.allowedMessageTypes)
			// add transaction to cache
			result := cache.Add(test.txBytes, test.hash, test.err)
			// validate result
			require.Equal(t, test.expectedResult, result)
			if test.expectedResult {
				// validate cache
				failedTx, ok := cache.Get(test.hash)
				require.True(t, ok)
				require.Equal(t, test.err, failedTx.Err)
				require.EqualExportedValues(t, tx, failedTx.Transaction)

				// validate get all
				failedTxs := cache.GetAll()
				require.Len(t, failedTxs, 1)
				require.Equal(t, failedTx, failedTxs[0])

				// validate removal
				cache.Remove(test.hash)
				_, ok = cache.Get(test.hash)
				require.False(t, ok)
			} else {
				// validate cache
				tx, ok := cache.Get(test.hash)
				require.False(t, ok)
				require.Nil(t, tx)
			}
		})
	}
}
