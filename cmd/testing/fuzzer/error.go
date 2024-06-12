// nolint:all
package main

import (
	"fmt"
	"github.com/ginchuco/ginchu/lib"
)

const (
	CodeExpectedInvalidTransaction = 1
)

func ErrInvalidParams(txType, reason string) lib.ErrorI {
	return lib.NewError(CodeExpectedInvalidTransaction, lib.MainModule, fmt.Sprintf("expected invalid %s transaction due to %s but got no error", txType, reason))
}
