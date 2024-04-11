package consensus

import (
	"bytes"
	"github.com/ginchuco/ginchu/crypto"
	lib "github.com/ginchuco/ginchu/types"
)

func (x *Message) SignBytes() (signBytes []byte, err lib.ErrorI) {
	switch {
	case x.IsLeaderMessage():
		return lib.Marshal(&Message{
			Header: x.Header,
			Vrf:    x.Vrf,
			Qc: &QuorumCertificate{
				Header:          x.Qc.Header,
				Block:           x.Qc.Block,
				LeaderPublicKey: x.Qc.LeaderPublicKey,
				Signature:       x.Qc.Signature,
			},
			HighQc: x.HighQc,
		})
	case x.IsReplicaMessage():
		return lib.Marshal(&QuorumCertificate{
			Header:          x.Qc.Header,
			Block:           x.Qc.Block,
			LeaderPublicKey: x.Qc.LeaderPublicKey,
		})
	case x.IsPacemakerMessage():
		return lib.Marshal(&Message{Header: x.Header})
	default:
		return nil, ErrUnknownConsensusMsg(x)
	}
}

func (x *Message) Check(view *lib.View, vs lib.ValidatorSetWrapper) lib.ErrorI {
	if x == nil {
		return ErrEmptyLeaderMessage()
	}
	if err := checkSignature(x.Signature, x); err != nil {
		return err
	}
	if err := x.Header.Check(view); err != nil {
		return err
	}
	switch {
	case x.IsLeaderMessage():
		if x.Header.Phase != lib.Phase_ELECTION {
			if err := x.Qc.Check(view, vs); err != nil {
				return err
			}
			if err := x.Qc.Block.Check(); err != nil {
				return err
			}
		}
		switch x.Header.Phase {
		case lib.Phase_ELECTION:
			if err := checkSignatureBasic(x.Vrf); err != nil {
				return err
			}
			if !bytes.Equal(x.Signature.PublicKey, x.Vrf.PublicKey) {
				return ErrMismatchPublicKeys()
			}
		case lib.Phase_PROPOSE:
			if len(x.Qc.LeaderPublicKey) != crypto.BLS12381PubKeySize {
				return ErrInvalidLeaderPublicKey()
			}
		case lib.Phase_PRECOMMIT, lib.Phase_COMMIT: // nothing
		}
	case x.IsReplicaMessage():
		if err := x.Qc.Check(view, vs); err != nil {
			return err
		}
		if x.Qc.Header.Phase == lib.Phase_PROPOSE_VOTE {
			if !bytes.Equal(x.Signature.PublicKey, x.Qc.LeaderPublicKey) {
				return ErrMismatchPublicKeys()
			}
		}
		switch x.Qc.Header.Phase {
		case lib.Phase_ELECTION_VOTE:
			if len(x.Qc.LeaderPublicKey) != crypto.BLS12381PubKeySize {
				return ErrInvalidLeaderPublicKey()
			}
		case lib.Phase_PROPOSE_VOTE, lib.Phase_PRECOMMIT_VOTE:
			if err := x.Qc.Block.Check(); err != nil {
				return err
			}
		}
	default:
		if !x.IsPacemakerMessage() {
			return ErrUnknownConsensusMsg(x)
		}
	}
	return nil
}

func (x *Message) Sign(privateKey crypto.PrivateKeyI) lib.ErrorI {
	bz, err := x.SignBytes()
	if err != nil {
		return err
	}
	x.Signature = new(lib.Signature)
	x.Signature.PublicKey = privateKey.PublicKey().Bytes()
	x.Signature.Signature = privateKey.Sign(bz)
	return nil
}

func (x *Message) IsReplicaMessage() bool {
	if x.Header != nil {
		return false
	}
	h := x.Qc.Header
	return h.Phase == lib.Phase_ELECTION_VOTE || h.Phase == lib.Phase_PROPOSE_VOTE || h.Phase == lib.Phase_PRECOMMIT_VOTE
}

func (x *Message) IsLeaderMessage() bool {
	h := x.Header
	if h == nil {
		return false
	}
	return h.Phase == lib.Phase_ELECTION || h.Phase == lib.Phase_PROPOSE || h.Phase == lib.Phase_PRECOMMIT || h.Phase == lib.Phase_COMMIT
}

func (x *Message) IsPacemakerMessage() bool {
	h := x.Header
	if h == nil {
		return false
	}
	return h.Phase == lib.Phase_ROUND_INTERRUPT
}

func checkSignature(signature *lib.Signature, sb lib.SignByte) lib.ErrorI {
	if err := checkSignatureBasic(signature); err != nil {
		return err
	}
	publicKey, err := lib.PublicKeyFromBytes(signature.PublicKey)
	if err != nil {
		return err
	}
	msg, err := sb.SignBytes()
	if err != nil {
		return err
	}
	if !publicKey.VerifyBytes(msg, signature.Signature) {
		return ErrInvalidPartialSignature()
	}
	return nil
}

func checkSignatureBasic(signature *lib.Signature) lib.ErrorI {
	if signature == nil || len(signature.PublicKey) == 0 || len(signature.Signature) == 0 {
		return ErrPartialSignatureEmpty()
	}
	if len(signature.PublicKey) != crypto.BLS12381PubKeySize {
		return ErrInvalidPublicKey()
	}
	if len(signature.Signature) != crypto.BLS12381SignatureSize {
		return ErrInvalidSignatureLength()
	}
	return nil
}
