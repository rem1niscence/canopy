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

	CodeUnmarshal      ErrorCode = 9
	CodeMarshal        ErrorCode = 10
	CodeFromAny        ErrorCode = 11
	CodeToAny          ErrorCode = 12
	CodeStringToBigInt ErrorCode = 13
	CodeStringToBytes  ErrorCode = 14

	StateMachineModule ErrorModule = "state_machine"

	CodeFeeBelowState           ErrorCode = 2
	CodeUnauthorizedTx          ErrorCode = 3
	CodeEmptySignature          ErrorCode = 4
	CodeTxSignBytes             ErrorCode = 5
	CodeInvalidTxMessage        ErrorCode = 6
	CodeInvalidTxSequence       ErrorCode = 7
	CodeDuplicateTransaction    ErrorCode = 8
	CodeGetTransaction          ErrorCode = 9
	CodeTxFoundInMempool        ErrorCode = 10
	CodeInvalidSignature        ErrorCode = 11
	CodeAddressEmpty            ErrorCode = 12
	CodeAddressSize             ErrorCode = 13
	CodeRecipientAddressEmpty   ErrorCode = 14
	CodeRecipientAddressSize    ErrorCode = 15
	CodeOutputAddressEmpty      ErrorCode = 20
	CodeOutputAddressSize       ErrorCode = 21
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

	StorageModule      ErrorModule = "store"
	CodeOpenDB         ErrorCode   = 1
	CodeCloseDB        ErrorCode   = 2
	CodeStoreSet       ErrorCode   = 3
	CodeStoreGet       ErrorCode   = 4
	CodeStoreDelete    ErrorCode   = 5
	CodeStoreIter      ErrorCode   = 6
	CodeStoreRevIter   ErrorCode   = 7
	CodeCopyStore      ErrorCode   = 8
	CodeWriteTxn       ErrorCode   = 9
	CodeDecompactProof ErrorCode   = 10
	CodeCommitDB       ErrorCode   = 11
	CodeCommitTree     ErrorCode   = 12
	CodeProve          ErrorCode   = 13
	CodeCompactProof   ErrorCode   = 14
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

func ErrUnmarshal(err error) ErrorI {
	return NewError(CodeUnmarshal, MainModule, fmt.Sprintf("unmarshal() failed with err: %s", err.Error()))
}

func ErrFromAny(err error) ErrorI {
	return NewError(CodeFromAny, MainModule, fmt.Sprintf("fromAny() failed with err: %s", err.Error()))
}

func ErrToAny(err error) ErrorI {
	return NewError(CodeToAny, MainModule, fmt.Sprintf("toAny() failed with err: %s", err.Error()))
}

func ErrMarshal(err error) ErrorI {
	return NewError(CodeMarshal, MainModule, fmt.Sprintf("marshal() failed with err: %s", err.Error()))
}

func ErrStringToBytes(err error) ErrorI {
	return NewError(CodeStringToBytes, MainModule, fmt.Sprintf("stringToBytes() failed with err: %s", err.Error()))
}
