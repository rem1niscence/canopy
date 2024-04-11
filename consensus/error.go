package consensus

import (
	"fmt"
	lib "github.com/ginchuco/ginchu/types"
	"google.golang.org/protobuf/proto"
)

func ErrUnknownConsensusMsg(t proto.Message) lib.ErrorI {
	return lib.NewError(lib.CodeUnknownConsensusMessage, lib.ConsensusModule, fmt.Sprintf("unknown consensus message: %T", t))
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
	return lib.NewError(lib.CodeEmptyPayload, lib.ConsensusModule, "empty vote")
}

func ErrEmptyLeaderMessage() lib.ErrorI {
	return lib.NewError(lib.CodeEmptyLeaderMessage, lib.ConsensusModule, "empty leader message")
}

func ErrEmptyQCPayload() lib.ErrorI {
	return lib.NewError(lib.CodeEmptyQuorumCertificatePayload, lib.ConsensusModule, "empty quorum certificate vote")
}

func ErrInvalidPayload() lib.ErrorI {
	return lib.NewError(lib.CodeInvalidPayload, lib.ConsensusModule, "invalid vote")
}

func ErrInvalidLeaderPublicKey() lib.ErrorI {
	return lib.NewError(lib.CodeInvalidLeaderPubKey, lib.ConsensusModule, "invalid leader public key")
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

func ErrAggregateSignature(err error) lib.ErrorI {
	return lib.NewError(lib.CodeAggregateSignature, lib.ConsensusModule, fmt.Sprintf("aggregateSignature() failed with err: %s", err.Error()))
}
