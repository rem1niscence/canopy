package lib

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/canopy-network/canopy/lib/crypto"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"time"
)

var (
	_ TransactionI = &Transaction{} // TransactionI interface enforcement of the Transaction struct
	_ SignatureI   = &Signature{}   // SignatureI interface enforcement of the Signature struct
	_ TxResultI    = &TxResult{}    // TxResultI interface enforcement of the TxResult struct
	_ Pageable     = new(TxResults) // Pageable interface enforcement of the TxResult struct
)

const (
	TxResultsPageName      = "tx-results-page"      // the name of a page of transactions
	PendingResultsPageName = "pending-results-page" //  the name of a page of mempool pending transactions
)

// Messages must be pre-registered for Transaction JSON unmarshalling
var RegisteredMessages map[string]MessageI

func init() {
	RegisteredMessages = make(map[string]MessageI)
	RegisteredPageables[TxResultsPageName] = new(TxResults)      // preregister the page type for unmarshalling
	RegisteredPageables[PendingResultsPageName] = new(TxResults) // preregister the page type for unmarshalling
}

// TRANSACTION INTERFACES BELOW

// TxResultI is the model of a completed transaction object after execution
type TxResultI interface {
	proto.Message
	GetSender() []byte      // the sender of the transaction
	GetRecipient() []byte   // the receiver of the transaction (i.e. the recipient of a 'send' transaction; empty of not applicable)
	GetMessageType() string // the type of message the transaction contains
	GetHeight() uint64      // the block number the transaction is included in
	GetIndex() uint64       // the index of the transaction in the block
	GetTxHash() string      // the cryptographic hash of the transaction
	GetTx() TransactionI
}

// TransactionI is the model of a transaction object (a record of an action or event)
type TransactionI interface {
	proto.Message
	GetMsg() *anypb.Any             // message payload (send, stake, edit-stake, etc.)
	GetSig() SignatureI             // digital signature allowing public key verification
	GetTime() uint64                // a stateless - prune friendly, replay attack / hash collision defense (opposed to sequence)
	GetSignBytes() ([]byte, ErrorI) // the canonical form the bytes were signed in
	GetHash() ([]byte, ErrorI)      // the computed cryptographic hash of the transaction bytes
	GetMemo() string                // an optional 100 character descriptive string - these are often used for polling
}

// SignatureI is the model of a signature object (a signature and public key bytes pair)
type SignatureI interface {
	proto.Message
	GetPublicKey() []byte
	GetSignature() []byte
}

// MessageI is the model of a message object (send, stake, edit-stake, etc.)
type MessageI interface {
	proto.Message

	New() MessageI     // new instance of the message type
	Name() string      // name of the message
	Check() ErrorI     // stateless validation of the message
	Recipient() []byte // for transaction indexing by recipient
	json.Marshaler     // json encoding
	json.Unmarshaler   // json decoding
}

// TRANSACTION CODE BELOW

// CheckBasic() is a stateless validation function for a Transaction object
func (x *Transaction) CheckBasic() ErrorI {
	if x == nil {
		return ErrEmptyTransaction()
	}
	if x.Msg == nil {
		return ErrEmptyMessage()
	}
	if x.MessageType == "" {
		return ErrUnknownMessageName(x.MessageType)
	}
	if x.Signature == nil || x.Signature.Signature == nil || x.Signature.PublicKey == nil {
		return ErrEmptySignature()
	}
	if x.Time == 0 {
		return ErrInvalidTxTime()
	}
	if len(x.Memo) > 200 {
		return ErrInvalidMemo()
	}
	return nil
}

// GetHash() returns the cryptographic hash of the Transaction
func (x *Transaction) GetHash() ([]byte, ErrorI) {
	bz, err := Marshal(x)
	if err != nil {
		return nil, err
	}
	return crypto.Hash(bz), nil
}

// GetSig() accessor for signature field (do not delete: needed to satisfy TransactionI)
func (x *Transaction) GetSig() SignatureI { return x.Signature }

// GetSignBytes() returns the canonical byte representation of the Transaction for signing and signature verification
func (x *Transaction) GetSignBytes() ([]byte, ErrorI) {
	return Marshal(&Transaction{
		MessageType: x.MessageType,
		Msg:         x.Msg,
		Signature:   nil,
		Time:        x.Time,
		Fee:         x.Fee,
		Memo:        x.Memo,
	})
}

// Sign() executes a digital signature on the transaction
func (x *Transaction) Sign(pk crypto.PrivateKeyI) ErrorI {
	bz, err := x.GetSignBytes()
	if err != nil {
		return err
	}
	x.Signature = &Signature{
		PublicKey: pk.PublicKey().Bytes(),
		Signature: pk.Sign(bz),
	}
	return nil
}

type jsonTx struct {
	Type      string          `json:"type,omitempty"`
	Msg       json.RawMessage `json:"msg,omitempty"`
	Signature *Signature      `json:"signature,omitempty"`
	Time      string          `json:"time,omitempty"`
	Fee       uint64          `json:"fee,omitempty"`
	Memo      string          `json:"memo,omitempty"`
}

// nolint:all
func (x Transaction) MarshalJSON() ([]byte, error) {
	a, err := FromAny(x.Msg)
	if err != nil {
		return nil, err
	}
	msg, ok := a.(MessageI)
	if !ok {
		return nil, fmt.Errorf("couldn't convert %T to type MessageI", a)
	}
	bz, err := MarshalJSON(msg)
	if err != nil {
		return nil, err
	}
	return json.Marshal(jsonTx{
		Type:      x.MessageType,
		Msg:       bz,
		Signature: x.Signature,
		Time:      time.UnixMicro(int64(x.Time)).Format(time.DateTime),
		Fee:       x.Fee,
		Memo:      x.Memo,
	})
}

func (x *Transaction) UnmarshalJSON(b []byte) error {
	var j jsonTx
	if err := json.Unmarshal(b, &j); err != nil {
		return err
	}
	var msg MessageI
	m, ok := RegisteredMessages[j.Type]
	if !ok {
		return ErrUnknownMessageName(j.Type)
	}
	msg = m.New()
	if err := json.Unmarshal(j.Msg, msg); err != nil {
		return err
	}
	a, err := NewAny(msg)
	if err != nil {
		return err
	}
	t, e := time.Parse(time.DateTime, j.Time)
	if e != nil {
		return e
	}
	*x = Transaction{
		MessageType: j.Type,
		Msg:         a,
		Signature:   j.Signature,
		Time:        uint64(t.UnixMicro()),
		Fee:         j.Fee,
		Memo:        j.Memo,
	}
	return nil
}

// TRANSACTION RESULT CODE BELOW

type TxResults []*TxResult

func (t *TxResults) Len() int      { return len(*t) }
func (t *TxResults) New() Pageable { return &TxResults{} }

// GetTx() is an accessor for the Transaction field
func (x *TxResult) GetTx() TransactionI { return x.Transaction }

type jsonTxResult struct {
	Sender      HexBytes     `json:"sender,omitempty"`
	Recipient   HexBytes     `json:"recipient,omitempty"`
	MessageType string       `json:"message_type,omitempty"`
	Height      uint64       `json:"height,omitempty"`
	Index       uint64       `json:"index,omitempty"`
	Transaction *Transaction `json:"transaction,omitempty"`
	TxHash      string       `json:"tx_hash,omitempty"`
}

// TxResult satisfies the json.Marshaller and json.Unmarshaler interfaces

// nolint:all
func (x TxResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(jsonTxResult{
		Sender:      x.Sender,
		Recipient:   x.Recipient,
		MessageType: x.MessageType,
		Height:      x.Height,
		Index:       x.Index,
		Transaction: x.Transaction,
		TxHash:      x.TxHash,
	})
}

func (x *TxResult) UnmarshalJSON(b []byte) error {
	var j jsonTxResult
	if err := json.Unmarshal(b, &j); err != nil {
		return err
	}
	*x = TxResult{
		Sender:      j.Sender,
		Recipient:   j.Recipient,
		MessageType: j.MessageType,
		Height:      j.Height,
		Index:       j.Index,
		Transaction: j.Transaction,
		TxHash:      j.TxHash,
	}
	return nil
}

// SIGNATURE CODE BELOW

type Signable interface {
	proto.Message
	Sign(p crypto.PrivateKeyI) ErrorI
}

type SignByte interface{ SignBytes() []byte }

// Copy() returns a clone of the Signature object
func (x *Signature) Copy() *Signature {
	return &Signature{
		PublicKey: bytes.Clone(x.PublicKey),
		Signature: bytes.Clone(x.Signature),
	}
}

// Signature satisfies the json.Marshaller and json.Unmarshaler interfaces

// nolint:all
func (x Signature) MarshalJSON() ([]byte, error) {
	return json.Marshal(jsonSignature{x.PublicKey, x.Signature})
}

func (x *Signature) UnmarshalJSON(b []byte) (err error) {
	var j jsonSignature
	if err = json.Unmarshal(b, &j); err != nil {
		return err
	}
	x.PublicKey, x.Signature = j.PublicKey, j.Signature
	return
}

type jsonSignature struct {
	PublicKey HexBytes `json:"public_key,omitempty"`
	Signature HexBytes `json:"signature,omitempty"`
}
