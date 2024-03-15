package types

import (
	"github.com/ginchuco/ginchu/crypto"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

var _ TransactionI = &Transaction{}

type TransactionI interface {
	proto.Message
	GetMsg() *anypb.Any
	GetSig() SignatureI
	GetNonce() string
	GetBytes() ([]byte, error)
	GetHash() ([]byte, error)
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

var _ SignatureI = &Signature{}

type SignatureI interface {
	proto.Message
	GetPublicKey() []byte
	GetSignature() []byte
}

var _ TransactionResultI = &TransactionResult{}

func (x *TransactionResult) GetTx() TransactionI {
	return x.Transaction
}
func (x *TransactionResult) GetBytes() ([]byte, error) {
	return cdc.Marshal(x)
}

func (x *TransactionResult) GetTxHash() ([]byte, error) {
	return x.GetTx().GetHash()
}

type TransactionResultI interface {
	proto.Message
	GetCode() uint64
	GetSender() []byte
	GetRecipient() []byte
	GetMessageType() string
	GetHeight() uint64
	GetIndex() uint64
	GetBytes() ([]byte, error)
	GetTxHash() ([]byte, error)
	GetTx() TransactionI
}
