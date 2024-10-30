package lib

import (
	"encoding/hex"
	"fmt"
	"math"
	"runtime"
)

type ErrorI interface {
	Code() ErrorCode     // Returns the error code
	Module() ErrorModule // Returns the error module
	error                // Implements the built-in error interface
}

var _ ErrorI = &Error{} // Ensures *Error implements ErrorI

type ErrorCode uint32 // Defines a type for error codes

type ErrorModule string // Defines a type for error modules

type Error struct {
	ECode   ErrorCode   `json:"Code"`   // Error code
	EModule ErrorModule `json:"Module"` // Error module
	Msg     string      `json:"Msg"`    // Error message
}

func NewError(code ErrorCode, module ErrorModule, msg string) *Error {
	// Constructs a new Error instance
	return &Error{ECode: code, EModule: module, Msg: msg}
}

// Code() returns the associated error code
func (p *Error) Code() ErrorCode { return p.ECode }

// Module() returns module field
func (p *Error) Module() ErrorModule { return p.EModule }

// String() calls Error()
func (p *Error) String() string { return p.Error() }

// Error() returns a formatted string including module, code, message, and stack trace
func (p *Error) Error() string {
	stack, pc := "", make([]uintptr, 1000)
	_ = runtime.Callers(1, pc)
	frames := runtime.CallersFrames(pc)
	if frames == nil {
		return fmt.Sprintf("\nModule:  %s\nCode:    %d\nMessage: %s\n", p.EModule, p.ECode, p.Msg)
	}
	for f, again := frames.Next(); again; f, again = frames.Next() {
		stack += fmt.Sprintf("\n%s L%d", f.File, f.Line)
	}
	return fmt.Sprintf("\nModule:  %s\nCode:    %d\nMessage: %s\nStack: %s", p.EModule, p.ECode, p.Msg, stack)
}

const (
	NoCode ErrorCode = math.MaxUint32

	// Main Module
	MainModule ErrorModule = "main"

	// Main Module Error Codes
	CodeInvalidAddress              ErrorCode = 1
	CodeJSONMarshal                 ErrorCode = 2
	CodeJSONUnmarshal               ErrorCode = 3
	CodeUnmarshal                   ErrorCode = 4
	CodeMarshal                     ErrorCode = 5
	CodeFromAny                     ErrorCode = 6
	CodeToAny                       ErrorCode = 7
	CodeStringToBytes               ErrorCode = 8
	CodeNilBlock                    ErrorCode = 9
	CodeNilBlockHeader              ErrorCode = 10
	CodeInvalidBlockProposerAddress ErrorCode = 11
	CodeInvalidBlockHash            ErrorCode = 12
	CodeNilBlockHash                ErrorCode = 13
	CodeNilBlockTime                ErrorCode = 14
	CodeNilLastBlockHash            ErrorCode = 15
	CodeNilNetworkID                ErrorCode = 16
	CodeNilStateRoot                ErrorCode = 17
	CodeNilTxRoot                   ErrorCode = 18
	CodeNilValRoot                  ErrorCode = 19
	CodeNilNextValRoot              ErrorCode = 20
	CodeMerkleTree                  ErrorCode = 21
	CodeUnequalBlockHash            ErrorCode = 22
	CodeNewPubKeyFromBytes          ErrorCode = 23
	CodeNewMultiPubKey              ErrorCode = 24
	CodeWriteFile                   ErrorCode = 25
	CodeReadFile                    ErrorCode = 26
	CodeInvalidArgument             ErrorCode = 27
	CodeNilRewardRecipients         ErrorCode = 28
	CodeNoValidators                ErrorCode = 29

	// Consensus Module
	ConsensusModule ErrorModule = "consensus"

	// Consensus Module Error Codes
	CodeDuplicateTransaction            ErrorCode = 1
	CodeTxFoundInMempool                ErrorCode = 2
	CodeMismatchResultsHash             ErrorCode = 3
	CodeDuplicateProposerMessage        ErrorCode = 4
	CodeDuplicateVote                   ErrorCode = 5
	CodeInvalidSignatureLength          ErrorCode = 6
	CodeInvalidPubKey                   ErrorCode = 7
	CodeEmptyView                       ErrorCode = 8
	CodeUnknownConsensusMessage         ErrorCode = 9
	CodeValidatorNotInSet               ErrorCode = 10
	CodeWrongHeight                     ErrorCode = 11
	CodeWrongRound                      ErrorCode = 12
	CodeWrongPhase                      ErrorCode = 13
	CodePartialSignatureEmpty           ErrorCode = 14
	CodeInvalidPartialSignature         ErrorCode = 15
	CodeMismatchBlockHash               ErrorCode = 16
	CodeInvalidProposerPubKey           ErrorCode = 17
	CodeNoMaj23                         ErrorCode = 18
	CodeEmptyAggregateSignature         ErrorCode = 19
	CodeInvalidAggregateSignature       ErrorCode = 20
	CodeInvalidAggregateSignatureLen    ErrorCode = 21
	CodeEmptyAggregateSignatureBitmap   ErrorCode = 22
	CodeInvalidAggregateSignatureBitmap ErrorCode = 23
	CodeMismatchPublicKeys              ErrorCode = 24
	CodeEmptyEvidence                   ErrorCode = 25
	CodeAggregateSignature              ErrorCode = 26
	CodeEmptyQuorumCertificate          ErrorCode = 27
	CodeEvidenceTooOld                  ErrorCode = 28
	CodeMismatchProposals               ErrorCode = 29
	CodeFailedSafeNode                  ErrorCode = 30
	CodeInvalidValidatorIndex           ErrorCode = 31
	CodeUnableToAddSigner               ErrorCode = 32
	CodeEmptyMessage                    ErrorCode = 33
	CodeInvalidBlockTime                ErrorCode = 34
	CodeInvalidEvidence                 ErrorCode = 35
	CodeMismatchEvidenceAndHeader       ErrorCode = 36
	CodeMismatchBadProposerCount        ErrorCode = 37
	CodeWrongMaxHeight                  ErrorCode = 38
	CodeExpectedBlockSizeLimit          ErrorCode = 39
	CodeNonNilCertResults               ErrorCode = 40
	CodeInvalidMemo                     ErrorCode = 41
	CodeNilCertResults                  ErrorCode = 42

	// State Machine Module
	StateMachineModule ErrorModule = "state_machine"

	// State Machine Module Error Codes
	CodeReadGenesisFile                   ErrorCode = 1
	CodeFeeBelowState                     ErrorCode = 2
	CodeUnauthorizedTx                    ErrorCode = 3
	CodeEmptySignature                    ErrorCode = 4
	CodeTxSignBytes                       ErrorCode = 5
	CodeInvalidTxMessage                  ErrorCode = 6
	CodeErrInvalidTxTime                  ErrorCode = 7
	CodeMaxBlockSize                      ErrorCode = 8
	CodeMaxTxSize                         ErrorCode = 9
	CodeRejectProposal                    ErrorCode = 10
	CodeInvalidNetAddressLen              ErrorCode = 11
	CodeInvalidSignature                  ErrorCode = 12
	CodeAddressEmpty                      ErrorCode = 13
	CodeAddressSize                       ErrorCode = 14
	CodeRecipientAddressEmpty             ErrorCode = 15
	CodeRecipientAddressSize              ErrorCode = 16
	CodeOutputAddressEmpty                ErrorCode = 17
	CodeOutputAddressSize                 ErrorCode = 18
	CodeInvalidAmount                     ErrorCode = 19
	CodePubKeyEmpty                       ErrorCode = 20
	CodePubKeySize                        ErrorCode = 21
	CodeParamKeyEmpty                     ErrorCode = 22
	CodeParamValEmpty                     ErrorCode = 23
	CodeInvalidSubsidy                    ErrorCode = 24
	CodeInvalidOpcode                     ErrorCode = 25
	CodeWrongCommitteeID                  ErrorCode = 26
	CodeUnknownMsg                        ErrorCode = 27
	CodeInsufficientFunds                 ErrorCode = 28
	CodeValidatorExists                   ErrorCode = 29
	CodeValidatorNotExists                ErrorCode = 30
	CodeValidatorUnstaking                ErrorCode = 31
	CodeValidatorPaused                   ErrorCode = 32
	CodeValidatorNotPaused                ErrorCode = 33
	CodeEmptyConsParams                   ErrorCode = 34
	CodeEmptyValParams                    ErrorCode = 35
	CodeEmptyFeeParams                    ErrorCode = 36
	CodeEmptyGovParams                    ErrorCode = 37
	CodeUnknownParam                      ErrorCode = 38
	CodeUnknownParamType                  ErrorCode = 39
	CodeUnknownParamSpace                 ErrorCode = 40
	CodeInvalidProposalHash               ErrorCode = 41
	CodeBelowMinimumStake                 ErrorCode = 42
	CodeInvalidSlashPercentage            ErrorCode = 43
	CodeNonSubsidizedCommittee            ErrorCode = 44
	CodeInvalidNumberOfSamples            ErrorCode = 45
	CodeInvalidCertificateResults         ErrorCode = 46
	CodeInvalidNumRecipients              ErrorCode = 47
	CodeInvalidPercentAllocation          ErrorCode = 48
	CodeStringToInt                       ErrorCode = 49
	CodeInvalidParam                      ErrorCode = 50
	CodeInvalidPoolName                   ErrorCode = 51
	CodeInvalidProtocolVersion            ErrorCode = 52
	CodeInvalidDBKey                      ErrorCode = 53
	CodeWrongStoreType                    ErrorCode = 54
	CodeUnmarshalGenesis                  ErrorCode = 55
	CodeInsufficientSupply                ErrorCode = 56
	CodeUnknownMsgName                    ErrorCode = 57
	CodeUnknownPageable                   ErrorCode = 58
	CodePollValidator                     ErrorCode = 59
	CodeInvalidBlockRange                 ErrorCode = 60
	CodeInvalidPublicKey                  ErrorCode = 61
	CodeInvalidDoubleSignHeights          ErrorCode = 62
	CodeInvalidDoubleSigner               ErrorCode = 63
	CodeInvalidNumCommittees              ErrorCode = 64
	CodeInvalidCommitteeStakeDistribution ErrorCode = 65
	CodeValidatorIsADelegate              ErrorCode = 66
	CodeInvalidCommittee                  ErrorCode = 67
	CodeInvalidCommitteeID                ErrorCode = 68
	CodeWrongNetworkID                    ErrorCode = 69
	CodeInvalidSlashRecipients            ErrorCode = 70
	CodeCommitteeHeight                   ErrorCode = 71
	CodeInvalidQCCommitteeHeight          ErrorCode = 72
	CodeInvalidBadProposer                ErrorCode = 73
	CodeOrderNotFound                     ErrorCode = 74
	CodeUnauthorizedOrderChange           ErrorCode = 75
	CodeMinimumOrderSize                  ErrorCode = 76
	CodeOrderAlreadyAccepted              ErrorCode = 77
	CodeInvalidBuyOrder                   ErrorCode = 78
	CodeDuplicateBuyOrder                 ErrorCode = 79
	CodeInvalidBuyerDeadline              ErrorCode = 80
	CodeInvalidCloseOrder                 ErrorCode = 81
	CodeInvalidResetOrder                 ErrorCode = 82
	CodeInvalidCheckpoint                 ErrorCode = 83
	CodeInvalidSellOrder                  ErrorCode = 84

	// P2P Module
	P2PModule ErrorModule = "p2p"

	// P2P Module Error Codes
	CodeUnknownP2PMessage            ErrorCode = 1
	CodeFailedRead                   ErrorCode = 2
	CodeFailedWrite                  ErrorCode = 3
	CodeMaxMessageSize               ErrorCode = 4
	CodePongTimeout                  ErrorCode = 5
	CodeBlacklisted                  ErrorCode = 6
	CodeErrorGroup                   ErrorCode = 7
	CodeConnDecrypt                  ErrorCode = 8
	CodeChunkLargerThanMax           ErrorCode = 9
	CodeFailedChallenge              ErrorCode = 10
	CodeFailedDiffieHellman          ErrorCode = 11
	CodeFailedHKDF                   ErrorCode = 12
	CodePeerAlreadyExists            ErrorCode = 13
	CodePeerNotFound                 ErrorCode = 14
	CodeFailedDial                   ErrorCode = 15
	CodeMismatchPeerPublicKey        ErrorCode = 16
	CodeFailedListen                 ErrorCode = 17
	CodeInvalidPeerPublicKey         ErrorCode = 18
	CodeSignatureSwap                ErrorCode = 19
	CodeMetaSwap                     ErrorCode = 20
	CodeBadStream                    ErrorCode = 21
	CodeBannedCountry                ErrorCode = 22
	CodeIPLookup                     ErrorCode = 23
	CodeBannedIP                     ErrorCode = 24
	CodeNonTCPAddr                   ErrorCode = 25
	CodeInvalidNetAddressString      ErrorCode = 26
	CodeInvalidNetAddressPubKey      ErrorCode = 27
	CodeInvalidNetAddressHostAndPort ErrorCode = 28
	CodeMaxOutbound                  ErrorCode = 29
	CodeMaxInbound                   ErrorCode = 30
	CodeBannedID                     ErrorCode = 31
	CodeIncompatiblePeer             ErrorCode = 32

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
	CodeInvalidKey     ErrorCode   = 15

	RPCModule             ErrorModule = "rpc"
	CodeRPCTimeout        ErrorCode   = 1
	CodeInvalidParams     ErrorCode   = 2
	CodeNewFSM            ErrorCode   = 3
	CodeTimeMachine       ErrorCode   = 4
	CodePostRequest       ErrorCode   = 5
	CodeGetRequest        ErrorCode   = 6
	CodeHttpStatus        ErrorCode   = 7
	CodeReadBody          ErrorCode   = 8
	CodeStringToCommittee ErrorCode   = 9
)

// error implementations below for the `types` package
func newLogError(err error) ErrorI {
	return NewError(NoCode, MainModule, err.Error())
}

func ErrUnmarshal(err error) ErrorI {
	return NewError(CodeUnmarshal, MainModule, fmt.Sprintf("unmarshal() failed with err: %s", err.Error()))
}

func ErrJSONUnmarshal(err error) ErrorI {
	return NewError(CodeJSONUnmarshal, MainModule, fmt.Sprintf("json.unmarshal() failed with err: %s", err.Error()))
}

func ErrJSONMarshal(err error) ErrorI {
	return NewError(CodeJSONMarshal, MainModule, fmt.Sprintf("json.marshal() failed with err: %s", err.Error()))
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

func ErrNilBlock() ErrorI {
	return NewError(CodeNilBlock, MainModule, "block is nil")
}

func ErrNilProposal() ErrorI {
	return NewError(CodeNilBlock, MainModule, "proposal is nil")
}

func ErrNilRewardRecipients() ErrorI {
	return NewError(CodeNilRewardRecipients, MainModule, "reward recipients is nil")
}

func ErrNilBlockHeader() ErrorI {
	return NewError(CodeNilBlockHeader, MainModule, "block.header is nil")
}

func ErrInvalidBlockProposerAddress() ErrorI {
	return NewError(CodeInvalidBlockProposerAddress, MainModule, "block proposer address is invalid")
}

func ErrInvalidBlockHash() ErrorI {
	return NewError(CodeInvalidBlockHash, MainModule, "invalid block hash")
}

func ErrNilBlockHash() ErrorI {
	return NewError(CodeNilBlockHash, MainModule, "nil block hash")
}

func ErrNilBlockTime() ErrorI {
	return NewError(CodeNilBlockTime, MainModule, "nil block time")
}

func ErrNilLastBlockHash() ErrorI {
	return NewError(CodeNilLastBlockHash, MainModule, "nil last block hash")
}

func ErrNilNetworkID() ErrorI {
	return NewError(CodeNilNetworkID, MainModule, "nil network id")
}

func ErrNilTransactionRoot() ErrorI {
	return NewError(CodeNilTxRoot, MainModule, "nil transaction root")
}

func ErrNilStateRoot() ErrorI {
	return NewError(CodeNilStateRoot, MainModule, "nil state root")
}

func ErrNilValidatorRoot() ErrorI {
	return NewError(CodeNilValRoot, MainModule, "nil validator root")
}

func ErrNilNextValidatorRoot() ErrorI {
	return NewError(CodeNilNextValRoot, MainModule, "nil next validator root")
}

func ErrMerkleTree(err error) ErrorI {
	return NewError(CodeMerkleTree, MainModule, fmt.Sprintf("merkle tree failed with err: %s", err.Error()))
}

func ErrUnequalBlockHash() ErrorI {
	return NewError(CodeUnequalBlockHash, MainModule, "unequal block hash")
}

func ErrPubKeyFromBytes(err error) ErrorI {
	return NewError(CodeNewPubKeyFromBytes, MainModule, fmt.Sprintf("publicKeyFromBytes() failed with err: %s", err.Error()))
}

func ErrNewMultiPubKey(err error) ErrorI {
	return NewError(CodeNewMultiPubKey, MainModule, fmt.Sprintf("newMultiPubKey() failed with err: %s", err.Error()))
}

func ErrNoValidators() ErrorI {
	return NewError(CodeNoValidators, MainModule, fmt.Sprintf("there are no validators in the set"))
}

func ErrWrongHeight() ErrorI {
	return NewError(CodeWrongHeight, ConsensusModule, "wrong height")
}

func ErrWrongCommitteeHeight() ErrorI {
	return NewError(CodeCommitteeHeight, ConsensusModule, "wrong committee height")
}

func ErrInvalidQCCommitteeHeight() ErrorI {
	return NewError(CodeInvalidQCCommitteeHeight, ConsensusModule, "invalid certificate committee height")
}

func ErrWrongMaxHeight() ErrorI {
	return NewError(CodeWrongMaxHeight, ConsensusModule, "wrong max height")
}

func ErrEmptyView() ErrorI {
	return NewError(CodeEmptyView, ConsensusModule, "empty view")
}

func ErrWrongRound() ErrorI {
	return NewError(CodeWrongRound, ConsensusModule, "wrong round")
}

func ErrWrongPhase() ErrorI {
	return NewError(CodeWrongPhase, ConsensusModule, "wrong phase")
}

func ErrEmptyQuorumCertificate() ErrorI {
	return NewError(CodeEmptyQuorumCertificate, ConsensusModule, "empty quorum certificate")
}

func ErrEmptyAggregateSignature() ErrorI {
	return NewError(CodeEmptyAggregateSignature, ConsensusModule, "empty aggregate signature")
}

func ErrInvalidAggrSignatureLength() ErrorI {
	return NewError(CodeInvalidAggregateSignatureLen, ConsensusModule, "invalid aggregate signature length")
}

func ErrEmptySignerBitmap() ErrorI {
	return NewError(CodeEmptyAggregateSignatureBitmap, ConsensusModule, "empty signer bitmap")
}

func ErrInvalidSignerBitmap(err error) ErrorI {
	return NewError(CodeInvalidAggregateSignatureBitmap, ConsensusModule, fmt.Sprintf("invalid signature bitmap: %s", err.Error()))
}

func ErrInvalidAggrSignature() ErrorI {
	return NewError(CodeInvalidAggregateSignature, ConsensusModule, "invalid aggregate signature")
}

func ErrNoMaj23() ErrorI {
	return NewError(CodeNoMaj23, ConsensusModule, "quorum not reached")
}

func ErrValidatorNotInSet(publicKey []byte) ErrorI {
	return NewError(CodeValidatorNotInSet, ConsensusModule, fmt.Sprintf("validator %s not found in validator set", BytesToString(publicKey)))
}

func ErrInvalidValidatorIndex() ErrorI {
	return NewError(CodeInvalidValidatorIndex, ConsensusModule, "invalid validator index")
}

func ErrInvalidBlockTime() ErrorI {
	return NewError(CodeInvalidBlockTime, ConsensusModule, "invalid block time")
}

func ErrInvalidMemo() ErrorI {
	return NewError(CodeInvalidMemo, ConsensusModule, "invalid memo")
}

func ErrEmptyEvidence() ErrorI {
	return NewError(CodeEmptyEvidence, ConsensusModule, "evidence is empty")
}

func ErrInvalidEvidence() ErrorI {
	return NewError(CodeInvalidEvidence, ConsensusModule, "evidence is invalid")
}

func ErrEvidenceTooOld() ErrorI {
	return NewError(CodeEvidenceTooOld, ConsensusModule, "evidence is too old")
}

func ErrInvalidProposerPubKey() ErrorI {
	return NewError(CodeInvalidProposerPubKey, ConsensusModule, "invalid proposer public key")
}

func ErrMismatchEvidenceAndHeader() ErrorI {
	return NewError(CodeMismatchEvidenceAndHeader, ConsensusModule, "mismatch evidence and block header")
}

func ErrMismatchBadProducerCount() ErrorI {
	return NewError(CodeMismatchBadProposerCount, ConsensusModule, "mismatch bad proposer count")
}

func ErrInvalidNetAddrString(s string) ErrorI {
	return NewError(CodeInvalidNetAddressString, P2PModule, fmt.Sprintf("invalid net address string: %s", s))
}

func ErrInvalidNetAddressPubKey(s string) ErrorI {
	return NewError(CodeInvalidNetAddressPubKey, P2PModule, fmt.Sprintf("invalid net address public key: %s", s))
}

func ErrInvalidNetAddressHostAndPort(s string) ErrorI {
	return NewError(CodeInvalidNetAddressHostAndPort, P2PModule, fmt.Sprintf("invalid net address host and port: %s", s))
}

func ErrTxFoundInMempool(hash string) ErrorI {
	return NewError(CodeTxFoundInMempool, ConsensusModule, fmt.Sprintf("tx %s already found in mempool", hash))
}

func ErrWriteFile(err error) ErrorI {
	return NewError(CodeWriteFile, MainModule, fmt.Sprintf("os.WriteFile() failed with err: %s", err.Error()))
}

func ErrReadFile(err error) ErrorI {
	return NewError(CodeReadFile, MainModule, fmt.Sprintf("os.ReadFile() failed with err: %s", err.Error()))
}

func ErrUnknownMessageName(s string) ErrorI {
	return NewError(CodeUnknownMsgName, StateMachineModule, fmt.Sprintf("message name %s is unknown", s))
}

func ErrUnknownPageable(s string) ErrorI {
	return NewError(CodeUnknownPageable, StateMachineModule, fmt.Sprintf("pageable %s is unknown", s))
}

func ErrEmptyMessage() ErrorI {
	return NewError(CodeEmptyMessage, StateMachineModule, "message is empty")
}

func ErrEmptySignature() ErrorI {
	return NewError(CodeEmptySignature, StateMachineModule, "signature is empty")
}

func ErrInvalidAddress() ErrorI {
	return NewError(CodeInvalidAddress, MainModule, "address is invalid")
}

func ErrInvalidDoubleSignHeights() ErrorI {
	return NewError(CodeInvalidDoubleSignHeights, ConsensusModule, "double sign heights are invalid")
}

func ErrInvalidDoubleSigner() ErrorI {
	return NewError(CodeInvalidDoubleSigner, ConsensusModule, "double signer is invalid")
}

func ErrInvalidBadProposer() ErrorI {
	return NewError(CodeInvalidBadProposer, ConsensusModule, "bad proposer is invalid")
}

func ErrMismatchResultsHash() ErrorI {
	return NewError(CodeMismatchResultsHash, ConsensusModule, "mismatch result hash")
}

func ErrMismatchBlockHash() ErrorI {
	return NewError(CodeMismatchBlockHash, ConsensusModule, "mismatch block hash")
}

func ErrInvalidPercentAllocation() ErrorI {
	return NewError(CodeInvalidPercentAllocation, StateMachineModule, "invalid percent allocation")
}

func ErrInvalidNumOfRecipients() ErrorI {
	return NewError(CodeInvalidNumRecipients, StateMachineModule, "invalid num payment recipients")
}

func ErrWrongNetworkID() ErrorI {
	return NewError(CodeWrongNetworkID, StateMachineModule, "wrong network id")
}

func ErrWrongCommitteeID() ErrorI {
	return NewError(CodeWrongCommitteeID, StateMachineModule, "wrong committee id")
}
func ErrDuplicateTx(hash []byte) ErrorI {
	return NewError(CodeDuplicateTransaction, ConsensusModule, fmt.Sprintf("tx %s is a duplicate", hex.EncodeToString(hash)))
}

func ErrMaxTxSize() ErrorI {
	return NewError(CodeMaxTxSize, StateMachineModule, "max tx size")
}
func ErrInvalidArgument() ErrorI {
	return NewError(CodeInvalidArgument, MainModule, "the argument is invalid")
}

func ErrExpectedMaxBlockSize() ErrorI {
	return NewError(CodeExpectedBlockSizeLimit, MainModule, "the block size exceeds the expected limit")
}
func ErrNonNilCertResults() ErrorI {
	return NewError(CodeNonNilCertResults, MainModule, "the certificate results is not empty")
}
func ErrNilCertResults() ErrorI {
	return NewError(CodeNilCertResults, MainModule, "the certificate results is empty")
}
