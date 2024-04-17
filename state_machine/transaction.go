package state_machine

import (
	"github.com/ginchuco/ginchu/state_machine/types"
	lib "github.com/ginchuco/ginchu/types"
	"github.com/ginchuco/ginchu/types/crypto"
	"google.golang.org/protobuf/types/known/anypb"
)

func (s *StateMachine) ApplyTransaction(index uint64, transaction []byte, txHash string) (*lib.TxResult, lib.ErrorI) {
	result, err := s.CheckTx(transaction)
	if err != nil {
		return nil, err
	}
	if err = s.AccountSetSequence(result.sender, result.tx.Sequence); err != nil {
		return nil, err
	}
	if err = s.AccountDeductFees(result.sender, result.tx.Fee); err != nil {
		return nil, err
	}
	if err = s.HandleMessage(result.msg); err != nil {
		return nil, err
	}
	return &lib.TxResult{
		Sender:      result.sender.Bytes(),
		Recipient:   result.msg.Recipient(),
		MessageType: result.msg.Name(),
		Height:      s.Height(),
		Index:       index,
		Transaction: result.tx,
		TxHash:      txHash,
	}, nil
}

func (s *StateMachine) CheckTx(transaction []byte) (result *CheckTxResult, err lib.ErrorI) {
	tx := new(lib.Transaction)
	if err = lib.Unmarshal(transaction, tx); err != nil {
		return
	}
	if err = s.CheckAccount(tx); err != nil {
		return
	}
	msg, err := s.CheckMessage(tx.Msg)
	if err != nil {
		return
	}
	sender, err := s.CheckSignature(msg, tx)
	if err != nil {
		return
	}
	stateLimitFee, err := s.GetFeeForMessage(msg)
	if err != nil {
		return
	}
	if err = s.CheckFee(tx.Fee, stateLimitFee); err != nil {
		return
	}
	return &CheckTxResult{
		tx:     tx,
		msg:    msg,
		sender: sender,
	}, nil
}

type CheckTxResult struct {
	tx     *lib.Transaction
	msg    lib.MessageI
	sender crypto.AddressI
}

func (s *StateMachine) CheckSignature(msg lib.MessageI, tx *lib.Transaction) (crypto.AddressI, lib.ErrorI) {
	if tx.Signature == nil {
		return nil, types.ErrEmptySignature()
	}
	if len(tx.Signature.Signature) == 0 || len(tx.Signature.PublicKey) != crypto.Ed25519PubKeySize {
		return nil, types.ErrEmptySignature()
	}
	signBytes, err := tx.GetSignBytes()
	if err != nil {
		return nil, types.ErrTxSignBytes(err)
	}
	publicKey := crypto.NewPublicKeyFromBytes(tx.Signature.PublicKey)
	if !publicKey.VerifyBytes(signBytes, tx.Signature.Signature) {
		return nil, types.ErrInvalidSignature()
	}
	address := publicKey.Address()
	signers, er := s.GetAuthorizedSignersFor(msg)
	if er != nil {
		return nil, er
	}
	for _, signer := range signers {
		if address.Equals(crypto.NewAddressFromBytes(signer)) {
			return address, nil
		}
	}
	return nil, types.ErrUnauthorizedTx()
}

func (s *StateMachine) CheckAccount(tx *lib.Transaction) lib.ErrorI {
	txCount, err := s.GetAccountSequence(crypto.NewPublicKeyFromBytes(tx.Signature.PublicKey).Address())
	if err != nil {
		return err
	}
	if tx.GetSequence() <= txCount {
		return types.ErrInvalidTxSequence()
	}
	return nil
}

func (s *StateMachine) CheckMessage(msg *anypb.Any) (message lib.MessageI, err lib.ErrorI) {
	proto, err := lib.FromAny(msg)
	if err != nil {
		return nil, err
	}
	message, ok := proto.(lib.MessageI)
	if !ok {
		return nil, types.ErrInvalidTxMessage()
	}
	if err = message.Check(); err != nil {
		return nil, err
	}
	return message, nil
}

func (s *StateMachine) CheckFee(fee, stateLimit string) lib.ErrorI {
	less, err := lib.StringsLess(fee, stateLimit)
	if err != nil {
		return err
	}
	if less {
		return types.ErrTxFeeBelowStateLimit()
	}
	return nil
}
