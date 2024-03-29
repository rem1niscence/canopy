package consensus

import (
	"github.com/ginchuco/ginchu/crypto"
	lib "github.com/ginchuco/ginchu/types"
)

const (
	newViewMsgName = "NEW_VIEW"
)

func (x *NewView) SignBytes() (signBytes []byte, err lib.ErrorI) {
	partialSignature := x.PartialSig
	x.PartialSig = nil
	signBytes, err = lib.Marshal(x)
	x.PartialSig = partialSignature
	return
}

func (x *NewView) Check(height, view uint64) lib.ErrorI {
	if err := checkPartialSignature(x.PartialSig, newViewMsgName, x); err != nil {
		return err
	}
	if x.Height != height {
		return ErrWrongHeight(newViewMsgName, x.PartialSig.PublicKey, x.Height, height)
	}
	if x.ViewNum != view {
		return ErrWrongView(newViewMsgName, x.PartialSig.PublicKey, x.ViewNum, view)
	} //TODO verify QC
	return nil
}

type SignByte interface {
	SignBytes() ([]byte, lib.ErrorI)
}

func checkPartialSignature(partialSig *lib.Signature, name string, sb SignByte) lib.ErrorI {
	if partialSig == nil || len(partialSig.PublicKey) == 0 || len(partialSig.Signature) == 0 {
		return ErrPartialSignatureEmpty(name)
	}
	publicKeyBytes := partialSig.PublicKey
	publicKey, err := crypto.NewBLSPublicKeyFromBytes(publicKeyBytes)
	if err != nil {
		return lib.ErrPubKeyFromBytes(err)
	}
	msg, libErr := sb.SignBytes()
	if libErr != nil {
		return libErr
	}
	if !publicKey.VerifyBytes(msg, partialSig.Signature) {
		return ErrInvalidPartialSignature(name, publicKeyBytes)
	}
	return nil
}
