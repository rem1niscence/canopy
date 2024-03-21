package store

import (
	"fmt"
	"github.com/ginchuco/ginchu/types"
)

// TODO convert all errors to lib.ErrorI

func newUnmarshalCompactProofError(err error) types.ErrorI {
	return types.NewError(types.NoCode, types.StorageModule, fmt.Sprintf("unmarshalCompactProof failed with err: %s", err.Error()))
}

func newDecompactProofError(err error) types.ErrorI {
	return types.NewError(types.NoCode, types.StorageModule, fmt.Sprintf("decompactProof failed with err: %s", err.Error()))
}

func NewTxWriteError(err error) types.ErrorI {
	return types.NewError(types.NoCode, types.StorageModule, fmt.Sprintf("tx write failed with err: %s", err.Error()))
}
