package lib

import (
	"encoding/json"
	"fmt"
	"github.com/ginchuco/ginchu/lib/crypto"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"time"
)

var _ TransactionI = &Transaction{}

const (
	TxResultsPageName      = "tx-results-page"
	PendingResultsPageName = "pending-results-page"
)

func init() {
	RegisteredPageables[TxResultsPageName] = new(TxResults)
	RegisteredPageables[PendingResultsPageName] = new(TxResults)
}

type TransactionI interface {
	proto.Message
	GetMsg() *anypb.Any
	GetSig() SignatureI
	GetTime() uint64
	GetBytes() ([]byte, ErrorI)
	GetSignBytes() ([]byte, ErrorI)
	GetHash() ([]byte, ErrorI)
}

type MessageI interface {
	proto.Message

	Check() ErrorI
	Bytes() ([]byte, ErrorI)
	Name() string
	New() MessageI
	Recipient() []byte // for transaction indexing by recipient
	MarshalJSON() ([]byte, error)
	UnmarshalJSON([]byte) error
}

func (x *Transaction) GetHash() ([]byte, ErrorI) {
	bz, err := x.GetBytes()
	if err != nil {
		return nil, err
	}
	return crypto.Hash(bz), nil
}

func (x *Transaction) GetBytes() ([]byte, ErrorI) {
	return Marshal(x)
}

func (x *Transaction) GetSig() SignatureI {
	return x.Signature
}

func (x *Transaction) GetSignBytes() ([]byte, ErrorI) {
	return Marshal(&Transaction{
		Msg:       x.Msg,
		Signature: nil,
		Time:      x.Time,
	})
}

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

func (x *Transaction) Check() ErrorI {
	if x.Msg == nil {
		return ErrEmptyMessage()
	}
	if x.Type == "" {
		return ErrUnknownMessageName(x.Type)
	}
	if x.Signature == nil {
		return ErrEmptySignature()
	}
	return nil
}

var _ SignatureI = &Signature{}

type SignatureI interface {
	proto.Message
	GetPublicKey() []byte
	GetSignature() []byte
}

var _ TxResultI = &TxResult{}

func (x *TxResult) GetTx() TransactionI        { return x.Transaction }
func (x *TxResult) GetBytes() ([]byte, ErrorI) { return Marshal(x) }

type TxResultI interface {
	proto.Message
	GetSender() []byte
	GetRecipient() []byte
	GetMessageType() string
	GetHeight() uint64
	GetIndex() uint64
	GetBytes() ([]byte, ErrorI)
	GetTxHash() string
	GetTx() TransactionI
}

func (x *Signature) Copy() *Signature {
	return &Signature{
		PublicKey: CopyBytes(x.PublicKey),
		Signature: CopyBytes(x.Signature),
	}
}

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

type Mempool interface {
	Contains(hash string) bool
	AddTransaction(tx []byte, fee uint64) (recheck bool, err ErrorI)
	DeleteTransaction(tx []byte)
	GetTransactions(maxBytes uint64) (int, [][]byte)

	Clear()
	Size() int
	TxsBytes() int
	Iterator() IteratorI
}

type Signable interface {
	proto.Message
	Sign(p crypto.PrivateKeyI) ErrorI
}

type SignByte interface{ SignBytes() []byte }

var RegisteredMessages map[string]MessageI

type jsonTx struct {
	Type      string          `json:"type,omitempty"`
	Msg       json.RawMessage `json:"msg,omitempty"`
	Signature *Signature      `json:"signature,omitempty"`
	Time      string          `json:"time,omitempty"`
	Fee       uint64          `json:"fee,omitempty"`
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
		Type:      x.Type,
		Msg:       bz,
		Signature: x.Signature,
		Time:      time.UnixMicro(int64(x.Time)).Format(time.DateTime),
		Fee:       x.Fee,
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
		Type:      j.Type,
		Msg:       a,
		Signature: j.Signature,
		Time:      uint64(t.UnixMicro()),
		Fee:       j.Fee,
	}
	return nil
}

type jsonTxResult struct {
	Sender      HexBytes     `json:"sender,omitempty"`
	Recipient   HexBytes     `json:"recipient,omitempty"`
	MessageType string       `json:"message_type,omitempty"`
	Height      uint64       `json:"height,omitempty"`
	Index       uint64       `json:"index,omitempty"`
	Transaction *Transaction `json:"transaction,omitempty"`
	TxHash      string       `json:"tx_hash,omitempty"`
}

var _ Pageable = new(TxResults)

type TxResults []*TxResult

func (t *TxResults) Len() int { return len(*t) }

func (t *TxResults) New() Pageable {
	return &TxResults{}
}

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
