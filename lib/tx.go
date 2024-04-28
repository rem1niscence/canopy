package lib

import (
	"encoding/json"
	"fmt"
	"github.com/ginchuco/ginchu/lib/crypto"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

var _ TransactionI = &Transaction{}

type TransactionI interface {
	proto.Message
	GetMsg() *anypb.Any
	GetSig() SignatureI
	GetSequence() uint64
	GetBytes() ([]byte, error)
	GetSignBytes() ([]byte, error)
	GetHash() ([]byte, error)
}

type MessageI interface {
	proto.Message

	Check() ErrorI
	Bytes() ([]byte, ErrorI)
	Name() string
	Recipient() []byte // for transaction indexing by recipient
	MarshalJSON() ([]byte, error)
	UnmarshalJSON([]byte) error
}

func (x *Transaction) GetHash() ([]byte, error) {
	bz, err := x.GetBytes()
	if err != nil {
		return nil, err
	}
	return crypto.Hash(bz), nil
}

func (x *Transaction) GetBytes() ([]byte, error) {
	return cdc.Marshal(x)
}

func (x *Transaction) GetSig() SignatureI {
	return x.Signature
}

func (x *Transaction) GetSignBytes() ([]byte, error) {
	return cdc.Marshal(Transaction{
		Msg:       x.Msg,
		Signature: nil,
		Sequence:  x.Sequence,
	})
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

type SignByte interface{ SignBytes() ([]byte, ErrorI) }

type jsonTx struct {
	Msg       MessageI   `json:"msg,omitempty"`
	Signature *Signature `json:"signature,omitempty"`
	Sequence  uint64     `json:"sequence,omitempty"`
	Fee       uint64     `json:"fee,omitempty"`
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
	return json.Marshal(jsonTx{
		Msg:       msg,
		Signature: x.Signature,
		Sequence:  x.Sequence,
		Fee:       x.Fee,
	})
}

func (x *Transaction) UnmarshalJSON(b []byte) error {
	var j jsonTx
	if err := json.Unmarshal(b, &j); err != nil {
		return err
	}
	a, err := ToAny(j.Msg)
	if err != nil {
		return err
	}
	*x = Transaction{
		Msg:       a,
		Signature: j.Signature,
		Sequence:  j.Sequence,
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
		Sender:      x.Sender,
		Recipient:   x.Recipient,
		MessageType: x.MessageType,
		Height:      x.Height,
		Index:       x.Index,
		Transaction: x.Transaction,
		TxHash:      x.TxHash,
	}
	return nil
}
