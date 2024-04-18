package lib

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

	MainModule               ErrorModule = "main"
	CodeUnmarshal            ErrorCode   = 9
	CodeMarshal              ErrorCode   = 10
	CodeFromAny              ErrorCode   = 11
	CodeToAny                ErrorCode   = 12
	CodeStringToBigInt       ErrorCode   = 13
	CodeStringToBytes        ErrorCode   = 14
	CodeNilBlock             ErrorCode   = 15
	CodeNilBlockHeader       ErrorCode   = 16
	CodeNilBlockProposer     ErrorCode   = 17
	CodeNilQuorumCertificate ErrorCode   = 18
	CodeNilBlockHash         ErrorCode   = 19
	CodeNilBlockTime         ErrorCode   = 20
	CodeNilLastBlockHash     ErrorCode   = 21
	CodeNilNetworkID         ErrorCode   = 22
	CodeNilStateRoot         ErrorCode   = 23
	CodeNilTxRoot            ErrorCode   = 24
	CodeNilValRoot           ErrorCode   = 25
	CodeNilNextValRoot       ErrorCode   = 26
	CodeMerkleTree           ErrorCode   = 27
	CodeUnequalBlockHash     ErrorCode   = 28
	CodeNewPubKeyFromBytes   ErrorCode   = 29
	CodeNewMultiPubKey       ErrorCode   = 30
	CodeStringToBigFloat     ErrorCode   = 31

	ConsensusModule                     ErrorModule = "consensus"
	CodeDuplicateTransaction            ErrorCode   = 6
	CodeTxFoundInMempool                ErrorCode   = 7
	CodeEmptyTxMessage                  ErrorCode   = 8
	CodeDuplicateLeaderMessage          ErrorCode   = 9
	CodeDuplicateVote                   ErrorCode   = 10
	CodeInvalidSignatureLength          ErrorCode   = 11
	CodeInvalidPubKey                   ErrorCode   = 12
	CodeEmptyView                       ErrorCode   = 13
	CodeUnknownConsensusMessage         ErrorCode   = 14
	CodeValidatorNotInSet               ErrorCode   = 15
	CodeWrongHeight                     ErrorCode   = 16
	CodeWrongRound                      ErrorCode   = 17
	CodeWrongPhase                      ErrorCode   = 18
	CodePartialSignatureEmpty           ErrorCode   = 19
	CodeInvalidPartialSignature         ErrorCode   = 20
	CodeEmptyBlock                      ErrorCode   = 21
	CodeEmptyPayload                    ErrorCode   = 22
	CodeInvalidPayload                  ErrorCode   = 23
	CodeInvalidLeaderPubKey             ErrorCode   = 24
	CodeNoMaj23                         ErrorCode   = 25
	CodeEmptyAggregateSignature         ErrorCode   = 26
	CodeInvalidAggregateSignature       ErrorCode   = 27
	CodeInvalidAggregateSignatureLen    ErrorCode   = 28
	CodeEmptyAggregateSignatureBitmap   ErrorCode   = 29
	CodeInvalidAggregateSignatureBitmap ErrorCode   = 30
	CodeVRFEmpty                        ErrorCode   = 31
	CodeNotVRFCandidate                 ErrorCode   = 32
	CodeMismatchPublicKeys              ErrorCode   = 33
	CodeEmptyPreviousAggregateSignature ErrorCode   = 34
	CodeEmptyEvidence                   ErrorCode   = 35
	CodeAggregateSignature              ErrorCode   = 36
	CodeEmptyQuorumCertificate          ErrorCode   = 37
	CodeEmptyQuorumCertificatePayload   ErrorCode   = 38
	CodeMismatchBlocks                  ErrorCode   = 39
	CodeFailedSafeNode                  ErrorCode   = 40
	CodeInvalidValidatorIndex           ErrorCode   = 41
	CodeUnableToAddSigner               ErrorCode   = 42
	CodeEmptyLeaderMessage              ErrorCode   = 43
	CodeInvalidBlockTime                ErrorCode   = 44
	CodeInvalidEvidence                 ErrorCode   = 45
	CodeDuplicateEvidence               ErrorCode   = 46
	CodeMismatchDoubleSignerCount       ErrorCode   = 47
	CodeMismatchEvidenceAndHeader       ErrorCode   = 48
	CodeMismatchBadProposerCount        ErrorCode   = 49
	CodeWrongMaxHeight                  ErrorCode   = 50

	StateMachineModule    ErrorModule = "state_machine"
	CodeFeeBelowState     ErrorCode   = 2
	CodeUnauthorizedTx    ErrorCode   = 3
	CodeEmptySignature    ErrorCode   = 4
	CodeTxSignBytes       ErrorCode   = 5
	CodeInvalidTxMessage  ErrorCode   = 6
	CodeInvalidTxSequence ErrorCode   = 7

	CodeGetTransaction ErrorCode = 9

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
	CodeWrongStoreType          ErrorCode = 58

	P2PModule                        ErrorModule = "p2p"
	CodeUnknownP2PMessage            ErrorCode   = 1
	CodeFailedRead                   ErrorCode   = 2
	CodeFailedWrite                  ErrorCode   = 3
	CodeFailedReadFull               ErrorCode   = 4
	CodeMaxMessageSize               ErrorCode   = 7
	CodePongTimeout                  ErrorCode   = 8
	CodeBlacklisted                  ErrorCode   = 9
	CodeErrorGroup                   ErrorCode   = 10
	CodeConnDecrypt                  ErrorCode   = 11
	CodeChunkLargerThanMax           ErrorCode   = 12
	CodeFailedChallenge              ErrorCode   = 13
	CodeFailedDiffieHellman          ErrorCode   = 14
	CodeFailedHKDF                   ErrorCode   = 15
	CodePeerAlreadyExists            ErrorCode   = 16
	CodePeerNotFound                 ErrorCode   = 17
	CodeFailedDial                   ErrorCode   = 18
	CodeMismatchPeerPublicKey        ErrorCode   = 19
	CodeFailedListen                 ErrorCode   = 20
	CodeHostAndPortFromRemote        ErrorCode   = 21
	CodeIPLookup                     ErrorCode   = 22
	CodeBannedIP                     ErrorCode   = 23
	CodeNonTCPAddr                   ErrorCode   = 24
	CodeInvalidNetAddressString      ErrorCode   = 25
	CodeInvalidNetAddressPubKey      ErrorCode   = 26
	CodeInvalidNetAddressHostAndPort ErrorCode   = 27
	CodeMaxOutbound                  ErrorCode   = 28
	CodeMaxInbound                   ErrorCode   = 29
	CodeBannedID                     ErrorCode   = 30

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
	return fmt.Sprintf("Code:\t%d\nModule:\t%s\nMessage:\t%s", p.code, p.module, p.msg)
}

// error implementations below for the `types` package
func newLogError(err error) ErrorI {
	return NewError(NoCode, MainModule, err.Error())
}

func errStringToBigInt() ErrorI {
	return NewError(CodeStringToBigInt, MainModule, "unable to convert string to big int")
}

func errStringToBigFloat() ErrorI {
	return NewError(CodeStringToBigFloat, MainModule, "unable to convert string to big float")
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

func ErrNilBlock() ErrorI {
	return NewError(CodeNilBlock, MainModule, "block is nil")
}

func ErrNilBlockHeader() ErrorI {
	return NewError(CodeNilBlockHeader, MainModule, "block.header is nil")
}

func ErrNilBlockProposer() ErrorI {
	return NewError(CodeNilBlockProposer, MainModule, "block proposer is nil")
}

func ErrNilQuorumCertificate() ErrorI {
	return NewError(CodeNilQuorumCertificate, MainModule, "nil quorum certificate")
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

func ErrWrongHeight() ErrorI {
	return NewError(CodeWrongHeight, ConsensusModule, "wrong height")
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

func ErrDuplicateEvidence() ErrorI {
	return NewError(CodeDuplicateEvidence, ConsensusModule, "duplicate evidence")
}

func ErrEmptyAggregateSignature() ErrorI {
	return NewError(CodeEmptyAggregateSignature, ConsensusModule, "empty aggregate signature")
}

func ErrEmptyAggregatePreviousSignature() ErrorI {
	return NewError(CodeEmptyPreviousAggregateSignature, ConsensusModule, "empty previous aggregate signature")
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

func ErrEmptyEvidence() ErrorI {
	return NewError(CodeEmptyEvidence, ConsensusModule, "evidence is empty")
}

func ErrInvalidEvidence() ErrorI {
	return NewError(CodeInvalidEvidence, ConsensusModule, "evidence is invalid")
}

func ErrInvalidLeaderPublicKey() ErrorI {
	return NewError(CodeInvalidLeaderPubKey, ConsensusModule, "invalid leader public key")
}

func ErrMismatchDoubleSignerCount() ErrorI {
	return NewError(CodeMismatchDoubleSignerCount, ConsensusModule, "mismatch double signer count")
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
