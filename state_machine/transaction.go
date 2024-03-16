package state_machine

import (
	"encoding/hex"
	"github.com/ginchuco/ginchu/crypto"
	"github.com/ginchuco/ginchu/state_machine/types"
	lib "github.com/ginchuco/ginchu/types"
	"google.golang.org/protobuf/types/known/anypb"
)

// TODO should have a mempool state?
func (s *StateMachine) ApplyTransaction(index uint64, transaction []byte) (*lib.TransactionResult, lib.ErrorI) {
	sender, tx, msg, err := s.checkTransaction(crypto.Hash(transaction), transaction)
	if err != nil {
		return nil, err
	}
	if err = s.AccountSetSequence(sender, tx.Sequence); err != nil {
		return nil, err
	}
	fee, err := s.GetFeeForMessage(msg)
	if err != nil {
		return nil, err
	}
	if err = s.AccountDeductFees(sender, fee); err != nil {
		return nil, err
	}
	if err = s.HandleMessage(msg); err != nil {
		return nil, err
	}
	return &lib.TransactionResult{
		Sender:      sender.Bytes(),
		Recipient:   msg.Recipient(),
		MessageType: msg.Name(),
		Height:      s.Height(),
		Index:       index,
		Transaction: tx,
	}, nil
}

func (s *StateMachine) CheckTransaction(transaction []byte) error {
	hash := crypto.Hash(transaction)
	hashString := hex.EncodeToString(hash)
	if s.mempool.Contains(hashString) {
		return types.ErrTxFoundInMempool(hashString)
	}
	if _, _, _, err := s.checkTransaction(hash, transaction); err != nil {
		return err
	}
	return s.mempool.AddTransaction(transaction)
}

func (s *StateMachine) checkTransaction(hash []byte, transaction []byte) (a crypto.AddressI, tx *lib.Transaction, msg lib.MessageI, err lib.ErrorI) {
	txResult, err := s.GetTxByHash(hash)
	if err != nil {
		return
	}
	if txResult != nil {
		err = types.ErrDuplicateTx(hash)
		return
	}
	tx = new(lib.Transaction)
	if err = types.Unmarshal(transaction, tx); err != nil {
		return
	}
	if err = s.checkAccount(tx); err != nil {
		return
	}
	msg, err = s.castMessage(tx.Msg)
	if err != nil {
		return
	}
	if a, err = s.checkSignature(msg, tx); err != nil {
		return
	}
	return
}

func (s *StateMachine) checkSignature(msg lib.MessageI, tx *lib.Transaction) (crypto.AddressI, lib.ErrorI) {
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

func (s *StateMachine) checkAccount(tx *lib.Transaction) lib.ErrorI {
	txCount, err := s.GetAccountSequence(crypto.NewPublicKeyFromBytes(tx.Signature.PublicKey).Address())
	if err != nil {
		return err
	}
	if tx.GetSequence() <= txCount {
		return types.ErrInvalidTxSequence()
	}
	return nil
}

func (s *StateMachine) castMessage(msg *anypb.Any) (message lib.MessageI, err lib.ErrorI) {
	proto, err := types.FromAny(msg)
	if err != nil {
		return nil, err
	}
	message, ok := proto.(lib.MessageI)
	if !ok {
		return nil, types.ErrInvalidTxMessage()
	}
	return message, nil
}
