package store

import (
	"fmt"
	"github.com/ginchuco/ginchu/types"
)

func ErrDecompactProof(err error) types.ErrorI {
	return types.NewError(types.CodeDecompactProof, types.StorageModule, fmt.Sprintf("decompactProof failed with err: %s", err.Error()))
}

func ErrTxWrite(err error) types.ErrorI {
	return types.NewError(types.CodeWriteTxn, types.StorageModule, fmt.Sprintf("tx write failed with err: %s", err.Error()))
}

func ErrCopyStore(err error) types.ErrorI {
	return types.NewError(types.CodeCopyStore, types.StorageModule, fmt.Sprintf("copy store failed with err: %s", err.Error()))
}

func ErrOpenDB(err error) types.ErrorI {
	return types.NewError(types.CodeOpenDB, types.StorageModule, fmt.Sprintf("openDB() failed with err: %s", err.Error()))
}

func ErrCloseDB(err error) types.ErrorI {
	return types.NewError(types.CodeCloseDB, types.StorageModule, fmt.Sprintf("closeDB() failed with err: %s", err.Error()))
}

func ErrCommitDB(err error) types.ErrorI {
	return types.NewError(types.CodeCommitDB, types.StorageModule, fmt.Sprintf("commitDB() failed with err: %s", err.Error()))
}

func ErrCommitTree(err error) types.ErrorI {
	return types.NewError(types.CodeCommitTree, types.StorageModule, fmt.Sprintf("commitTree() failed with err: %s", err.Error()))
}

func ErrStoreSet(err error) types.ErrorI {
	return types.NewError(types.CodeStoreSet, types.StorageModule, fmt.Sprintf("store.set() failed with err: %s", err.Error()))
}

func ErrStoreDelete(err error) types.ErrorI {
	return types.NewError(types.CodeStoreDelete, types.StorageModule, fmt.Sprintf("store.delete() failed with err: %s", err.Error()))
}

func ErrStoreGet(err error) types.ErrorI {
	return types.NewError(types.CodeStoreGet, types.StorageModule, fmt.Sprintf("store.get() failed with err: %s", err.Error()))
}

func ErrStoreIter(err error) types.ErrorI {
	return types.NewError(types.CodeStoreIter, types.StorageModule, fmt.Sprintf("store.Iter() failed with err: %s", err.Error()))
}

func ErrStoreRevIter(err error) types.ErrorI {
	return types.NewError(types.CodeStoreRevIter, types.StorageModule, fmt.Sprintf("store.RevIter() failed with err: %s", err.Error()))
}

func ErrProve(err error) types.ErrorI {
	return types.NewError(types.CodeProve, types.StorageModule, fmt.Sprintf("prove() failed with err: %s", err.Error()))
}

func ErrCompactProof(err error) types.ErrorI {
	return types.NewError(types.CodeCompactProof, types.StorageModule, fmt.Sprintf("compactProof() failed with err: %s", err.Error()))
}
