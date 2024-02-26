package store

import (
	"fmt"
	"github.com/ginchuco/ginchu/types"
)

func newOpenStoreError(err error) types.ErrorI {
	return types.NewError(types.NoCode, types.StorageModule, fmt.Sprintf("open store failed with err: %s", err.Error()))
}

func newNilKeyError() types.ErrorI {
	return types.NewError(types.NilKeyCode, types.StorageModule, "nil key was passed to the store")
}

func newNilValueError() types.ErrorI {
	return types.NewError(types.NilValueCode, types.StorageModule, "nil value was passed to the store")
}

func newUnmarshalCompactProofError(err error) types.ErrorI {
	return types.NewError(types.NoCode, types.StorageModule, fmt.Sprintf("unmarshalCompactProof failed with err: %s", err.Error()))
}

func newDecompactProofError(err error) types.ErrorI {
	return types.NewError(types.NoCode, types.StorageModule, fmt.Sprintf("decompactProof failed with err: %s", err.Error()))
}
