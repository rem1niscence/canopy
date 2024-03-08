package types

import (
	"fmt"
	lib "github.com/ginchuco/ginchu/types"
)

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

func ErrStoreSet(err error) lib.ErrorI {
	return lib.NewError(lib.CodeStoreSet, lib.StateMachineModule, fmt.Sprintf("store.Set() failed with err: %s", err.Error()))
}

func ErrUnmarshal(err error) lib.ErrorI {
	return lib.NewError(lib.CodeUnmarshal, lib.StateMachineModule, fmt.Sprintf("unmarshal() failed with err: %s", err.Error()))
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
