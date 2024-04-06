package consensus

import (
	"fmt"
	lib "github.com/ginchuco/ginchu/types"
	"google.golang.org/protobuf/proto"
)

func ErrUnknownConsensusMsg(t proto.Message) lib.ErrorI {
	return lib.NewError(lib.CodeUnknownConsensusMessage, lib.ConsensusModule, fmt.Sprintf("unknown consensus message: %T", t))
}

func ErrValidatorNotInSet(publicKey []byte) lib.ErrorI {
	return lib.NewError(lib.CodeValidatorNotInSet, lib.ConsensusModule, fmt.Sprintf("validator %s not found in validator set", lib.BytesToString(publicKey)))
}

func ErrInvalidValidatorIndex() lib.ErrorI {
	return lib.NewError(lib.CodeInvalidValidatorIndex, lib.ConsensusModule, "invalid validator index")
}

func ErrDuplicateVote() lib.ErrorI {
	return lib.NewError(lib.CodeDuplicateVote, lib.ConsensusModule, "duplicate vote")
}

func ErrDuplicateLeaderMessage() lib.ErrorI {
	return lib.NewError(lib.CodeDuplicateLeaderMessage, lib.ConsensusModule, "duplicate leader message")
}

func ErrUnableToAddSigner(err error) lib.ErrorI {
	return lib.NewError(lib.CodeUnableToAddSigner, lib.ConsensusModule, fmt.Sprintf("multiKey.AddSigner() failed with err: %s", err.Error()))
}

func ErrWrongHeight() lib.ErrorI {
	return lib.NewError(lib.CodeWrongHeight, lib.ConsensusModule, "wrong height")
}

func ErrEmptyView() lib.ErrorI {
	return lib.NewError(lib.CodeEmptyView, lib.ConsensusModule, "empty view")
}

func ErrWrongRound() lib.ErrorI {
	return lib.NewError(lib.CodeWrongRound, lib.ConsensusModule, "wrong round")
}

func ErrWrongPhase() lib.ErrorI {
	return lib.NewError(lib.CodeWrongPhase, lib.ConsensusModule, "wrong phase")
}

func ErrEmptyQuorumCertificate() lib.ErrorI {
	return lib.NewError(lib.CodeEmptyQuorumCertificate, lib.ConsensusModule, "empty quorum certificate")
}

func ErrPartialSignatureEmpty() lib.ErrorI {
	return lib.NewError(lib.CodePartialSignatureEmpty, lib.ConsensusModule, "empty signature")
}

func ErrInvalidPublicKey() lib.ErrorI {
	return lib.NewError(lib.CodeInvalidPubKey, lib.ConsensusModule, "invalid public key")
}

func ErrInvalidSignatureLength() lib.ErrorI {
	return lib.NewError(lib.CodeInvalidSignatureLength, lib.ConsensusModule, "invalid signature length")
}

func ErrInvalidPartialSignature() lib.ErrorI {
	return lib.NewError(lib.CodeInvalidPartialSignature, lib.ConsensusModule, "invalid partial signature")
}

func ErrEmptyBlock() lib.ErrorI {
	return lib.NewError(lib.CodeEmptyBlock, lib.ConsensusModule, "block empty")
}

func ErrEmptyPayload() lib.ErrorI {
	return lib.NewError(lib.CodeEmptyPayload, lib.ConsensusModule, "empty payload")
}

func ErrEmptyReplicaMessage() lib.ErrorI {
	return lib.NewError(lib.CodeEmptyReplicaMessage, lib.ConsensusModule, "empty replica message")
}

func ErrEmptyPacemakerMessage() lib.ErrorI {
	return lib.NewError(lib.CodeEmptyPacemakerMessage, lib.ConsensusModule, "empty pacemaker message")
}

func ErrEmptyLeaderMessage() lib.ErrorI {
	return lib.NewError(lib.CodeEmptyLeaderMessage, lib.ConsensusModule, "empty leader message")
}

func ErrEmptyElectionMessage() lib.ErrorI {
	return lib.NewError(lib.CodeEmptyElectionMessage, lib.ConsensusModule, "empty election message")
}

func ErrEmptyQCPayload() lib.ErrorI {
	return lib.NewError(lib.CodeEmptyQuorumCertificatePayload, lib.ConsensusModule, "empty quorum certificate payload")
}

func ErrInvalidPayload() lib.ErrorI {
	return lib.NewError(lib.CodeInvalidPayload, lib.ConsensusModule, "invalid payload")
}

func ErrInvalidLeaderPublicKey() lib.ErrorI {
	return lib.NewError(lib.CodeInvalidLeaderPubKey, lib.ConsensusModule, "invalid leader public key")
}

func ErrInvalidMessageType(msgType string, publicKey []byte, expectedType string) lib.ErrorI {
	return lib.NewError(lib.CodeInvalidMessageType, lib.ConsensusModule, fmt.Sprintf("%s from %s has an invalid message type: expected %s", msgType, lib.BytesToString(publicKey), expectedType))
}

func ErrEmptyAggregateSignature() lib.ErrorI {
	return lib.NewError(lib.CodeEmptyAggregateSignature, lib.ConsensusModule, "empty aggregate signature")
}

func ErrInvalidAggrSignatureLength() lib.ErrorI {
	return lib.NewError(lib.CodeInvalidAggregateSignatureLen, lib.ConsensusModule, "invalid aggregate signature length")
}

func ErrEmptySignerBitmap() lib.ErrorI {
	return lib.NewError(lib.CodeEmptyAggregateSignatureBitmap, lib.ConsensusModule, "empty signer bitmap")
}

func ErrInvalidSignerBitmap(err error) lib.ErrorI {
	return lib.NewError(lib.CodeInvalidAggregateSignatureBitmap, lib.ConsensusModule, fmt.Sprintf("invalid signature bitmap: %s", err.Error()))
}

func ErrInvalidAggrSignature() lib.ErrorI {
	return lib.NewError(lib.CodeInvalidAggregateSignature, lib.ConsensusModule, "invalid aggregate signature")
}

func ErrEmptyVRF() lib.ErrorI {
	return lib.NewError(lib.CodeVRFEmpty, lib.ConsensusModule, "empty vrf")
}

func ErrNotVRFCandidate() lib.ErrorI {
	return lib.NewError(lib.CodeNotVRFCandidate, lib.ConsensusModule, "vrf not a candidate")
}

func ErrMismatchPublicKeys() lib.ErrorI {
	return lib.NewError(lib.CodeMismatchPublicKeys, lib.ConsensusModule, "mismatch public keys")
}

func ErrMismatchBlocks() lib.ErrorI {
	return lib.NewError(lib.CodeMismatchBlocks, lib.ConsensusModule, "mismatch blocks")
}

func ErrFailedSafeNodePredicate() lib.ErrorI {
	return lib.NewError(lib.CodeFailedSafeNode, lib.ConsensusModule, "safe node failed")
}

func ErrPubKeyFromBytes(err error) lib.ErrorI {
	return lib.NewError(lib.CodePublicKeyFromBytes, lib.ConsensusModule, fmt.Sprintf("publicKeyFromBytes() failed with err: %s", err.Error()))
}

func ErrAggregateSignature(err error) lib.ErrorI {
	return lib.NewError(lib.CodeAggregateSignature, lib.ConsensusModule, fmt.Sprintf("aggregateSignature() failed with err: %s", err.Error()))
}
