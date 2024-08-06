package controller

import (
	"encoding/hex"
	"fmt"
	"github.com/ginchuco/ginchu/lib"
)

func ErrDuplicateTx(hash []byte) lib.ErrorI {
	return lib.NewError(lib.CodeDuplicateTransaction, lib.ConsensusModule, fmt.Sprintf("tx %s is a duplicate", hex.EncodeToString(hash)))
}
