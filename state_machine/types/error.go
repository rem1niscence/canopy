package types

import (
	"encoding/hex"
	"fmt"
	lib "github.com/ginchuco/ginchu/types"
)

func ErrUnauthorizedTx() lib.ErrorI {
	return lib.NewError(lib.CodeUnauthorizedTx, lib.StateMachineModule, "unauthorized tx")
}

func ErrInvalidTxSequence() lib.ErrorI {
	return lib.NewError(lib.CodeInvalidTxSequence, lib.StateMachineModule, "invalid sequence")
}

func ErrInvalidTxMessage() lib.ErrorI {
	return lib.NewError(lib.CodeInvalidTxMessage, lib.StateMachineModule, "invalid transaction message")
}

func ErrDuplicateTx(hash []byte) lib.ErrorI {
	return lib.NewError(lib.CodeDuplicateTransaction, lib.StateMachineModule, fmt.Sprintf("tx %s is a duplicate", hex.EncodeToString(hash)))
}

func ErrTxFoundInMempool(hash string) lib.ErrorI {
	return lib.NewError(lib.CodeTxFoundInMempool, lib.StateMachineModule, fmt.Sprintf("tx %s already found in mempool", hash))
}

func ErrGetTransaction(err error) lib.ErrorI {
	return lib.NewError(lib.CodeGetTransaction, lib.StateMachineModule, fmt.Sprintf("getTransaction() failed with err: %s", err.Error()))
}

func ErrAddressFromString(err error) lib.ErrorI {
	return lib.NewError(lib.CodeAddressFromString, lib.StateMachineModule, fmt.Sprintf("addressToString() failed with err: %s", err.Error()))
}

func ErrStoreGet(err error) lib.ErrorI {
	return lib.NewError(lib.CodeStoreGet, lib.StateMachineModule, fmt.Sprintf("store.Get() failed with err: %s", err.Error()))
}

func ErrStoreDelete(err error) lib.ErrorI {
	return lib.NewError(lib.CodeStoreDelete, lib.StateMachineModule, fmt.Sprintf("store.Delete() failed with err: %s", err.Error()))
}

func ErrStoreIter(err error) lib.ErrorI {
	return lib.NewError(lib.CodeStoreIter, lib.StateMachineModule, fmt.Sprintf("store.Iter() failed with err: %s", err.Error()))
}

func ErrStoreRevIter(err error) lib.ErrorI {
	return lib.NewError(lib.CodeStoreRevIter, lib.StateMachineModule, fmt.Sprintf("store.RevIter() failed with err: %s", err.Error()))
}

func ErrStoreSet(err error) lib.ErrorI {
	return lib.NewError(lib.CodeStoreSet, lib.StateMachineModule, fmt.Sprintf("store.Set() failed with err: %s", err.Error()))
}

func ErrUnmarshal(err error) lib.ErrorI {
	return lib.NewError(lib.CodeUnmarshal, lib.StateMachineModule, fmt.Sprintf("unmarshal() failed with err: %s", err.Error()))
}

func ErrFromAny(err error) lib.ErrorI {
	return lib.NewError(lib.CodeFromAny, lib.StateMachineModule, fmt.Sprintf("fromAny() failed with err: %s", err.Error()))
}

func ErrMarshal(err error) lib.ErrorI {
	return lib.NewError(lib.CodeMarshal, lib.StateMachineModule, fmt.Sprintf("marshal() failed with err: %s", err.Error()))
}

func ErrAddressEmpty() lib.ErrorI {
	return lib.NewError(lib.CodeAddressEmpty, lib.StateMachineModule, "address is empty")
}

func ErrRecipientAddressEmpty() lib.ErrorI {
	return lib.NewError(lib.CodeRecipientAddressEmpty, lib.StateMachineModule, "recipient address is empty")
}

func ErrOutputAddressEmpty() lib.ErrorI {
	return lib.NewError(lib.CodeOutputAddressEmpty, lib.StateMachineModule, "output address is empty")
}

func ErrOutputAddressSize() lib.ErrorI {
	return lib.NewError(lib.CodeOutputAddressSize, lib.StateMachineModule, "output address size is invalid")
}

func ErrAddressSize() lib.ErrorI {
	return lib.NewError(lib.CodeAddressSize, lib.StateMachineModule, "address size is invalid")
}

func ErrRecipientAddressSize() lib.ErrorI {
	return lib.NewError(lib.CodeRecipientAddressSize, lib.StateMachineModule, "recipient address size is invalid")
}

func ErrInvalidAmount() lib.ErrorI {
	return lib.NewError(lib.CodeInvalidAmount, lib.StateMachineModule, "amount is invalid")
}

func ErrPublicKeyEmpty() lib.ErrorI {
	return lib.NewError(lib.CodePubKeyEmpty, lib.StateMachineModule, "public key is empty")
}

func ErrPublicKeySize() lib.ErrorI {
	return lib.NewError(lib.CodePubKeySize, lib.StateMachineModule, "public key size is invalid")
}

func ErrParamKeyEmpty() lib.ErrorI {
	return lib.NewError(lib.CodeParamKeyEmpty, lib.StateMachineModule, "the parameter key is empty")
}

func ErrParamValueEmpty() lib.ErrorI {
	return lib.NewError(lib.CodeParamValEmpty, lib.StateMachineModule, "the parameter value is empty")
}

func ErrVoteEmpty() lib.ErrorI {
	return lib.NewError(lib.CodeVoteEmpty, lib.StateMachineModule, "vote is empty")
}

func ErrHashEmpty() lib.ErrorI {
	return lib.NewError(lib.CodeHashEmpty, lib.StateMachineModule, "hash is empty")
}

func ErrHashSize() lib.ErrorI {
	return lib.NewError(lib.CodeHashSize, lib.StateMachineModule, "hash size is invalid")
}

func ErrUnknownMessage(x lib.MessageI) lib.ErrorI {
	return lib.NewError(lib.CodeUnknownMsg, lib.StateMachineModule, fmt.Sprintf("message %T is unknown", x))
}

func ErrInsufficientFunds() lib.ErrorI {
	return lib.NewError(lib.CodeInsufficientFunds, lib.StateMachineModule, "insufficient funds")
}

func ErrValidatorExists() lib.ErrorI {
	return lib.NewError(lib.CodeValidatorExists, lib.StateMachineModule, "validator exists")
}

func ErrValidatorNotExists() lib.ErrorI {
	return lib.NewError(lib.CodeValidatorNotExists, lib.StateMachineModule, "validator does not exist")
}

func ErrValidatorUnstaking() lib.ErrorI {
	return lib.NewError(lib.CodeValidatorUnstaking, lib.StateMachineModule, "validator is unstaking")
}

func ErrValidatorPaused() lib.ErrorI {
	return lib.NewError(lib.CodeValidatorPaused, lib.StateMachineModule, "validator paused")
}

func ErrValidatorNotPaused() lib.ErrorI {
	return lib.NewError(lib.CodeValidatorNotPaused, lib.StateMachineModule, "validator not paused")
}

func ErrEmptyConsParams() lib.ErrorI {
	return lib.NewError(lib.CodeEmptyConsParams, lib.StateMachineModule, "consensus params empty")
}

func ErrEmptyValParams() lib.ErrorI {
	return lib.NewError(lib.CodeEmptyValParams, lib.StateMachineModule, "validator params empty")
}

func ErrEmptyFeeParams() lib.ErrorI {
	return lib.NewError(lib.CodeEmptyFeeParams, lib.StateMachineModule, "fee params empty")
}

func ErrEmptyGovParams() lib.ErrorI {
	return lib.NewError(lib.CodeEmptyGovParams, lib.StateMachineModule, "governance params empty")
}

func ErrUnknownParam() lib.ErrorI {
	return lib.NewError(lib.CodeUnknownParamType, lib.StateMachineModule, "unknown param")
}

func ErrUnknownParamSpace() lib.ErrorI {
	return lib.NewError(lib.CodeUnknownParamSpace, lib.StateMachineModule, "unknown param space")
}

func ErrUnknownParamType(t any) lib.ErrorI {
	return lib.NewError(lib.CodeUnknownParamType, lib.StateMachineModule, fmt.Sprintf("unknown param type %T", t))
}

func ErrUnauthorizedParamChange() lib.ErrorI {
	return lib.NewError(lib.CodeUnauthorizedParamChange, lib.StateMachineModule, "unauthorized param change")
}

func ErrBelowMinimumStake() lib.ErrorI {
	return lib.NewError(lib.CodeBelowMinimumStake, lib.StateMachineModule, "less than minimum stake")
}

func ErrInvalidSlashPercentage() lib.ErrorI {
	return lib.NewError(lib.CodeInvalidSlashPercentage, lib.StateMachineModule, "slash percent invalid")
}

func ErrPublicKeysNotEqual() lib.ErrorI {
	return lib.NewError(lib.CodePublicKeysNotEqual, lib.StateMachineModule, "public keys not equal")
}

func ErrHeightsNotEqual() lib.ErrorI {
	return lib.NewError(lib.CodeHeightsNotEqual, lib.StateMachineModule, "heights not equal")
}

func ErrRoundsNotEqual() lib.ErrorI {
	return lib.NewError(lib.CodeRoundsNotEqual, lib.StateMachineModule, "rounds not equal")
}

func ErrVoteTypesNotEqual() lib.ErrorI {
	return lib.NewError(lib.CodeVoteTypesNotEqual, lib.StateMachineModule, "vote types not equal")
}

func ErrIdenticalVotes() lib.ErrorI {
	return lib.NewError(lib.CodeIdenticalVotes, lib.StateMachineModule, "identical votes")
}

func ErrInvalidSignature() lib.ErrorI {
	return lib.NewError(lib.CodeInvalidSignature, lib.StateMachineModule, "invalid signature")
}

func ErrEmptySignature() lib.ErrorI {
	return lib.NewError(lib.CodeEmptySignature, lib.StateMachineModule, "empty signature")
}

func ErrTxSignBytes(err error) lib.ErrorI {
	return lib.NewError(lib.CodeTxSignBytes, lib.StateMachineModule, fmt.Sprintf("tx.SignBytes() failed with err: %s", err.Error()))
}

func ErrInvalidProtocolVersion() lib.ErrorI {
	return lib.NewError(lib.CodeInvalidProtocolVersion, lib.StateMachineModule, "invalid protocol version")
}

func ErrInvalidAddressKey(key []byte) lib.ErrorI {
	return lib.NewError(lib.CodeInvalidAddressKey, lib.StateMachineModule, fmt.Sprintf("invalid address key: %s", key))
}

func ErrInvalidParam(paramName string) lib.ErrorI {
	return lib.NewError(lib.CodeInvalidParam, lib.StateMachineModule, fmt.Sprintf("invalid param: %s", paramName))
}

func ErrInvalidOwner(paramName string) lib.ErrorI {
	return lib.NewError(lib.CodeInvalidParamOwner, lib.StateMachineModule, fmt.Sprintf("invalid owner for %s", paramName))
}

func ErrInvalidPoolName() lib.ErrorI {
	return lib.NewError(lib.CodeInvalidPoolName, lib.StateMachineModule, "invalid pool name")
}
