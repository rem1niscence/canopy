package state_machine

import (
	"encoding/hex"
	"github.com/ginchuco/ginchu/crypto"
	"github.com/ginchuco/ginchu/state_machine/types"
	lib "github.com/ginchuco/ginchu/types"
	"google.golang.org/protobuf/types/known/anypb"
)

func (s *StateMachine) CheckTransaction(transaction []byte) error {
	hash := crypto.Hash(transaction)
	hashString := hex.EncodeToString(hash)
	if s.mempool.Contains(hashString) {
		return types.ErrTxFoundInMempool(hashString)
	}
	txResult, err := s.GetTxByHash(hash)
	if err != nil {
		return err
	}
	if txResult != nil {
		return types.ErrDuplicateTx(hashString)
	}
	// TODO non-ordered nonce requires non-pruned tx indexer
	tx := new(lib.Transaction)
	if err = types.Unmarshal(transaction, tx); err != nil {
		return err
	}
	if err = s.checkTransaction(tx); err != nil {
		return err
	}
	return s.mempool.AddTransaction(transaction)
}

func (s *StateMachine) checkTransaction(tx *lib.Transaction) lib.ErrorI {
	if err := s.checkNonce(tx.Nonce); err != nil {
		return err
	}
	if err := s.checkSignature(tx.Signature); err != nil {
		return err
	}
	return s.checkMessage(tx.Msg)
}

func (s *StateMachine) checkSignature(sig *lib.Signature) lib.ErrorI {
	if sig == nil {
		return types.ErrInvalidSignature()
	}
	if sig.Signature == nil {

	}
}

func (s *StateMachine) checkNonce(nonce string) lib.ErrorI {
	i, err := lib.StringToBigInt(nonce)
	if err != nil {
		return err
	}
}

func (s *StateMachine) checkMessage(msg *anypb.Any) lib.ErrorI {

}

func (s *StateMachine) ApplyTransaction(index int, tx *lib.Transaction) (lib.TransactionResult, lib.ErrorI) {

}
