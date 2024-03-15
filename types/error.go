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

	CodeStringToBigInt ErrorCode = 11

	StateMachineModule ErrorModule = "state_machine"

	CodeInvalidNonce            ErrorCode = 3
	CodeDuplicateTransaction    ErrorCode = 4
	CodeGetTransaction          ErrorCode = 5
	CodeTxFoundInMempool        ErrorCode = 6
	CodeInvalidSignature        ErrorCode = 7
	CodeMarshal                 ErrorCode = 8
	CodeUnmarshal               ErrorCode = 9
	CodeFromAny                 ErrorCode = 10
	CodeStoreGet                ErrorCode = 11
	CodeStoreSet                ErrorCode = 12
	CodeStoreDelete             ErrorCode = 13
	CodeStoreIter               ErrorCode = 14
	CodeStoreRevIter            ErrorCode = 15
	CodeAddressEmpty            ErrorCode = 16
	CodeAddressSize             ErrorCode = 17
	CodeRecipientAddressEmpty   ErrorCode = 18
	CodeRecipientAddressSize    ErrorCode = 19
	CodeOutputAddressEmpty      ErrorCode = 20
	CodeOutputAddressSize       ErrorCode = 21
	CodeAddressFromString       ErrorCode = 22
	CodeInvalidAmount           ErrorCode = 23
	CodePubKeyEmpty             ErrorCode = 24
	CodePubKeySize              ErrorCode = 25
	CodeParamKeyEmpty           ErrorCode = 26
	CodeParamValEmpty           ErrorCode = 27
	CodeVoteEmpty               ErrorCode = 28
	CodeHashEmpty               ErrorCode = 29
	CodeHashSize                ErrorCode = 30
	CodeUnknownMsg              ErrorCode = 31
	CodeInsufficientFunds       ErrorCode = 32
	CodeValidatorExists         ErrorCode = 33
	CodeValidatorNotExists      ErrorCode = 34
	CodeValidatorUnstaking      ErrorCode = 35
	CodeValidatorPaused         ErrorCode = 36
	CodeValidatorNotPaused      ErrorCode = 37
	CodeEmptyConsParams         ErrorCode = 38
	CodeEmptyValParams          ErrorCode = 39
	CodeEmptyFeeParams          ErrorCode = 40
	CodeEmptyGovParams          ErrorCode = 41
	CodeUnknownParam            ErrorCode = 42
	CodeUnknownParamType        ErrorCode = 43
	CodeUnknownParamSpace       ErrorCode = 44
	CodeUnauthorizedParamChange ErrorCode = 45
	CodeBelowMinimumStake       ErrorCode = 46
	CodeInvalidSlashPercentage  ErrorCode = 47
	CodePublicKeysNotEqual      ErrorCode = 48
	CodeHeightsNotEqual         ErrorCode = 49
	CodeRoundsNotEqual          ErrorCode = 50
	CodeVoteTypesNotEqual       ErrorCode = 51
	CodeIdenticalVotes          ErrorCode = 52
	CodeInvalidParamOwner       ErrorCode = 53
	CodeInvalidParam            ErrorCode = 54
	CodeInvalidPoolName         ErrorCode = 55
	CodeInvalidProtocolVersion  ErrorCode = 56
	CodeInvalidAddressKey       ErrorCode = 57

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

func errStringToBigInt() ErrorI {
	return NewError(CodeStringToBigInt, MainModule, "unable to convert string to big int")
}
