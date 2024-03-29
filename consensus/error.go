package consensus

import (
	"fmt"
	lib "github.com/ginchuco/ginchu/types"
	"github.com/gogo/protobuf/proto"
)

func ErrUnknownConsensusMsg(t proto.Message) lib.ErrorI {
	return lib.NewError(lib.CodeUnknownConsensusMessage, lib.ConsensusModule, fmt.Sprintf("unknown consensus message: %T", t))
}

func ErrValidatorNotInSet(publicKey []byte) lib.ErrorI {
	return lib.NewError(lib.CodeValidatorNotInSet, lib.ConsensusModule, fmt.Sprintf("validator %s not found in validator set", lib.BytesToString(publicKey)))
}

func ErrWrongHeight(msgType string, publicKey []byte, wrongHeight, rightHeight uint64) lib.ErrorI {
	return lib.NewError(lib.CodeWrongHeight, lib.ConsensusModule, fmt.Sprintf("msg %s from %s has wrong height %d should be %d", msgType, lib.BytesToString(publicKey), wrongHeight, rightHeight))
}

func ErrWrongView(msgType string, publicKey []byte, wrongView, rightView uint64) lib.ErrorI {
	return lib.NewError(lib.CodeWrongHeight, lib.ConsensusModule, fmt.Sprintf("msg %s from %s has wrong view %d should be %d", msgType, lib.BytesToString(publicKey), wrongView, rightView))
}

func ErrPartialSignatureEmpty(msgType string) lib.ErrorI {
	return lib.NewError(lib.CodePartialSignatureEmpty, lib.ConsensusModule, msgType+" has an empty signature")
}

func ErrInvalidPartialSignature(msgType string, publicKey []byte) lib.ErrorI {
	return lib.NewError(lib.CodeErrInvalidPartialSignature, lib.ConsensusModule, fmt.Sprintf("%s from %s has an invalid partial signature", msgType, lib.BytesToString(publicKey)))
}
