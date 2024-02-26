package types

import (
	"fmt"
	"math"
)

const (

	// error codes
	// any error with code='noCode' is a non-protocol
	// level error; all protocol level errors must have a code
	// assigned for consensus level issues

	NoCode ErrorCode = math.MaxUint32

	// error module
	// helps segment the error codes
	// the combination of error code + module must be conflict free
	// to avoid consensus level issues

	MainModule ErrorModule = "main"

	StorageModule ErrorModule = "store"
	NilKeyCode    ErrorCode   = 1
	NilValueCode  ErrorCode   = 2
)

type ErrorI interface {
	Code() ErrorCode
	Module() ErrorModule
	error
}

var _ ErrorI = &Error{}

type ErrorCode uint32

type ErrorModule string

type Error struct {
	code   ErrorCode
	module ErrorModule
	msg    string
}

func NewError(code ErrorCode, module ErrorModule, msg string) *Error {
	return &Error{code: code, module: module, msg: msg}
}

func (p *Error) Code() ErrorCode     { return p.code }
func (p *Error) Module() ErrorModule { return p.module }
func (p *Error) String() string      { return p.Error() }

func (p *Error) Error() string {
	return fmt.Sprintf("Code: %d\nModule: %s\nMessage:%s", p.code, p.module, p.msg)
}

// error implementations below for the `types` package
func newLogError(err error) ErrorI {
	return NewError(NoCode, MainModule, err.Error())
}
