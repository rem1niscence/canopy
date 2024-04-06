package consensus

import (
	"bytes"
	"github.com/ginchuco/ginchu/crypto"
	lib "github.com/ginchuco/ginchu/types"
)

func (x *LeaderMessage) SignBytes() (signBytes []byte, err lib.ErrorI) {
	partialSignature := x.LeaderSignature
	x.LeaderSignature = nil
	signBytes, err = lib.Marshal(x)
	x.LeaderSignature = partialSignature
	return
}

func (x *LeaderMessage) Check(view *View, key crypto.MultiPublicKeyI, checkPhaseAndRound bool) lib.ErrorI {
	if x == nil {
		return ErrEmptyLeaderMessage()
	}
	if err := validateSignature(x.LeaderSignature, x); err != nil {
		return err
	}
	if x.Header == nil {
		return ErrEmptyView()
	}
	view.Phase--
	if err := x.Header.Check(view, checkPhaseAndRound); err != nil {
		return err
	}
	if x.Qc == nil {
		return ErrEmptyQuorumCertificate()
	}
	return x.Qc.Check(view, key.Copy(), checkPhaseAndRound)
}

func (x *PacemakerMessage) Check(view *View) lib.ErrorI {
	if x == nil {
		return ErrEmptyPacemakerMessage()
	}
	if err := validateSignature(x.PartialSignature, x); err != nil {
		return err
	}
	return x.Header.Check(view, false)
}

func (x *PacemakerMessage) SignBytes() ([]byte, lib.ErrorI) {
	sig := x.PartialSignature
	x.PartialSignature = nil
	bz, err := lib.Marshal(x)
	x.PartialSignature = sig
	return bz, err
}

func (x *View) Check(view *View, checkPhaseAndRound bool) lib.ErrorI {
	if x == nil {
		return ErrEmptyView()
	}
	if x.Height != view.Height {
		return ErrWrongHeight()
	}
	if checkPhaseAndRound {
		if x.Round != view.Round {
			return ErrWrongRound()
		}
		if x.Phase != view.Phase {
			return ErrWrongPhase()
		}
	}
	return nil
}

func (x *ReplicaMessage) Check(view *View, checkPhaseAndRound, checkSignature bool) lib.ErrorI {
	if x == nil {
		return ErrEmptyReplicaMessage()
	}
	if checkSignature {
		if err := validateSignature(x.PartialSignature, x); err != nil {
			return err
		}
	}
	if x.Header == nil {
		return ErrEmptyView()
	}
	view.Phase--
	if err := x.Header.Check(view, checkPhaseAndRound); err != nil {
		return err
	}
	if x.Payload == nil {
		return ErrEmptyPayload()
	}
	if view.Phase == Phase_ELECTION_VOTE {
		leaderPublicKey := x.GetLeaderPublicKey()
		if len(leaderPublicKey) != crypto.BLS12381PubKeySize {
			return ErrInvalidLeaderPublicKey()
		}
	} else {
		block := x.GetBlock()
		if err := block.Check(); err != nil {
			return err
		}
	}
	return nil
}

func (x *ElectionMessage) Check(view *View, checkPhaseAndRound bool) lib.ErrorI {
	if x == nil {
		return ErrEmptyElectionMessage()
	}
	if x.Header == nil {
		return ErrEmptyView()
	}
	if err := validateSignature(x.CandidateSignature, x); err != nil {
		return err
	}
	if !bytes.Equal(x.CandidateSignature.PublicKey, x.Vrf.PublicKey) {
		return ErrMismatchPublicKeys()
	}
	view.Phase--
	return x.Header.Check(view, checkPhaseAndRound)
}

func (x *QuorumCertificate) SignBytes() (signBytes []byte, err lib.ErrorI) {
	aggregateSignature := x.Signature
	x.Signature = nil
	signBytes, err = lib.Marshal(x)
	x.Signature = aggregateSignature
	return
}

func (x *QuorumCertificate) Check(view *View, key crypto.MultiPublicKeyI, checkPhaseAndRound bool) lib.ErrorI {
	if x == nil {
		return ErrEmptyQuorumCertificate()
	}
	if view == nil {
		view = x.Header.Copy()
		if view.Phase != Phase_PRECOMMIT_VOTE {
			return ErrWrongPhase()
		}
		view.Phase++
	}
	view.Phase--
	if err := x.Header.Check(view, checkPhaseAndRound); err != nil {
		return err
	}
	if x.Payload == nil {
		return ErrEmptyPayload()
	}
	if err := x.Payload.Check(view.Copy(), checkPhaseAndRound, false); err != nil {
		return err
	}
	return x.Signature.Check(key, x.Payload)
}

func (x *ElectionMessage) SignBytes() (bz []byte, err lib.ErrorI) {
	cs := x.CandidateSignature
	x.CandidateSignature = nil
	bz, err = lib.Marshal(cs)
	x.CandidateSignature = cs
	return
}

func (x *ElectionMessage) Sign(privateKey crypto.PrivateKeyI) lib.ErrorI {
	bz, err := x.SignBytes()
	if err != nil {
		return err
	}
	x.CandidateSignature = new(lib.Signature)
	x.CandidateSignature.PublicKey = privateKey.PublicKey().Bytes()
	x.CandidateSignature.Signature = privateKey.Sign(bz)
	return nil
}

func (x *ReplicaMessage) Sign(privateKey crypto.PrivateKeyI) lib.ErrorI {
	bz, err := x.SignBytes()
	if err != nil {
		return err
	}
	x.PartialSignature = new(lib.Signature)
	x.PartialSignature.PublicKey = privateKey.PublicKey().Bytes()
	x.PartialSignature.Signature = privateKey.Sign(bz)
	return nil
}

func (x *ReplicaMessage) SignBytes() (signBytes []byte, err lib.ErrorI) {
	partialSignature, qc := x.PartialSignature, x.HighQC
	x.PartialSignature, x.HighQC = nil, nil
	signBytes, err = lib.Marshal(x)
	x.PartialSignature = partialSignature
	x.HighQC = qc
	return
}

func (x *LeaderMessage) Sign(privateKey crypto.PrivateKeyI) lib.ErrorI {
	bz, err := x.SignBytes()
	if err != nil {
		return err
	}
	x.LeaderSignature = new(lib.Signature)
	x.LeaderSignature.PublicKey = privateKey.PublicKey().Bytes()
	x.LeaderSignature.Signature = privateKey.Sign(bz)
	return nil
}

func (x *ElectionMessage) Copy() *ElectionMessage {
	return &ElectionMessage{
		Header:             x.Header.Copy(),
		Vrf:                x.Vrf.Copy(),
		CandidateSignature: x.CandidateSignature.Copy(),
	}
}

func (x *View) Copy() *View {
	return &View{
		Height: x.Height,
		Round:  x.Round,
		Phase:  x.Phase,
	}
}

type SignByte interface {
	SignBytes() ([]byte, lib.ErrorI)
}

func (x *AggregateSignature) Check(key crypto.MultiPublicKeyI, sb SignByte) lib.ErrorI {
	if x == nil {
		return ErrEmptyAggregateSignature()
	}
	if len(x.Signature) != crypto.BLS12381SignatureSize {
		return ErrInvalidAggrSignatureLength()
	}
	if x.Bitmap == nil {
		return ErrEmptySignerBitmap()
	}
	if err := key.SetBitmap(x.Bitmap); err != nil {
		return ErrInvalidSignerBitmap(err)
	}
	// TODO IMPORTANT check that bitmap to validator power has +2/3 quorum
	msg, err := sb.SignBytes()
	if err != nil {
		return err
	}
	if !key.VerifyBytes(msg, x.Signature) {
		return ErrInvalidAggrSignature()
	}
	return nil
}

func validateSignature(signature *lib.Signature, sb SignByte) lib.ErrorI {
	if signature == nil || len(signature.PublicKey) == 0 || len(signature.Signature) == 0 {
		return ErrPartialSignatureEmpty()
	}
	if len(signature.PublicKey) != crypto.BLS12381PubKeySize {
		return ErrInvalidPublicKey()
	}
	if len(signature.Signature) != crypto.BLS12381SignatureSize {
		return ErrInvalidSignatureLength()
	}
	publicKey, err := publicKeyFromBytes(signature.PublicKey)
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
