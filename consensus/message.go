package consensus

import (
	"bytes"
	"github.com/ginchuco/ginchu/crypto"
	lib "github.com/ginchuco/ginchu/types"
	"google.golang.org/protobuf/proto"
)

func (x *Message) SignBytes() (signBytes []byte, err lib.ErrorI) {
	switch {
	case x.IsLeaderMessage():
		return lib.Marshal(&Message{
			Header: x.Header,
			Vrf:    x.Vrf,
			Qc: &QuorumCertificate{
				Header:                x.Qc.Header,
				Block:                 x.Qc.Block,
				LeaderPublicKey:       x.Qc.LeaderPublicKey,
				PreviousAggrSignature: x.Qc.PreviousAggrSignature,
				Signature:             x.Qc.Signature,
			},
			HighQc: x.HighQc,
		})
	case x.IsReplicaMessage():
		return lib.Marshal(&QuorumCertificate{
			Header:                x.Qc.Header,
			Block:                 x.Qc.Block,
			LeaderPublicKey:       x.Qc.LeaderPublicKey,
			PreviousAggrSignature: x.Qc.Signature,
		})
	case x.IsPacemakerMessage():
		return lib.Marshal(&Message{Header: x.Header})
	default:
		return nil, ErrUnknownConsensusMsg(x)
	}
}

func (x *Message) Check(view *View, vs ValidatorSet) lib.ErrorI {
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
		if x.Header.Phase != Phase_ELECTION {
			if err := x.Qc.Check(view, vs); err != nil {
				return err
			}
			if err := x.Qc.Block.Check(); err != nil {
				return err
			}
		}
		switch x.Header.Phase {
		case Phase_ELECTION:
			if err := checkSignatureBasic(x.Vrf); err != nil {
				return err
			}
			if !bytes.Equal(x.Signature.PublicKey, x.Vrf.PublicKey) {
				return ErrMismatchPublicKeys()
			}
		case Phase_PROPOSE:
			if len(x.Qc.LeaderPublicKey) != crypto.BLS12381PubKeySize {
				return ErrInvalidLeaderPublicKey()
			}
		case Phase_PRECOMMIT, Phase_COMMIT:
			if x.Qc.PreviousAggrSignature == nil {
				return ErrEmptyAggregatePreviousSignature()
			}
		}
	case x.IsReplicaMessage():
		if err := x.Qc.Check(view, vs); err != nil {
			return err
		}
		if x.Qc.Header.Phase == Phase_PROPOSE_VOTE {
			if !bytes.Equal(x.Signature.PublicKey, x.Qc.LeaderPublicKey) {
				return ErrMismatchPublicKeys()
			}
		}
		switch x.Qc.Header.Phase {
		case Phase_ELECTION_VOTE:
			if len(x.Qc.LeaderPublicKey) != crypto.BLS12381PubKeySize {
				return ErrInvalidLeaderPublicKey()
			}
		case Phase_PROPOSE_VOTE, Phase_PRECOMMIT_VOTE:
			if err := x.Qc.Block.Check(); err != nil {
				return err
			}
			if x.Qc.PreviousAggrSignature == nil {
				return ErrEmptyAggregatePreviousSignature()
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
	return h.Phase == Phase_ELECTION_VOTE || h.Phase == Phase_PROPOSE_VOTE || h.Phase == Phase_PRECOMMIT_VOTE
}

func (x *Message) IsLeaderMessage() bool {
	h := x.Header
	if h == nil {
		return false
	}
	return h.Phase == Phase_ELECTION || h.Phase == Phase_PROPOSE || h.Phase == Phase_PRECOMMIT || h.Phase == Phase_COMMIT
}

func (x *Message) IsPacemakerMessage() bool {
	h := x.Header
	if h == nil {
		return false
	}
	return h.Phase == Phase_ROUND_INTERRUPT
}

func (x *QuorumCertificate) SignBytes() (signBytes []byte, err lib.ErrorI) {
	aggregateSignature := x.Signature
	block := x.Block
	if x.Header.Phase == Phase_PROPOSE_VOTE { // omit because in EV no block was included in the message, but proposer inserts one in PROPOSE
		x.Block = nil
	}
	x.Signature = nil
	signBytes, err = lib.Marshal(x)
	x.Signature = aggregateSignature
	x.Block = block
	return
}

func (x *QuorumCertificate) Check(view *View, vs ValidatorSet) lib.ErrorI {
	if x == nil {
		return ErrEmptyQuorumCertificate()
	}
	if err := x.Header.Check(view); err != nil {
		return err
	}
	return x.Signature.Check(vs.key.Copy(), x, vs)
}

func (x *QuorumCertificate) CheckHighQC(view *View, vs ValidatorSet) lib.ErrorI {
	if x == nil {
		return ErrEmptyQuorumCertificate()
	}
	if err := x.Header.Check(view); err != nil {
		return err
	}
	if x.Header.Phase != Phase_PRECOMMIT {
		return ErrWrongPhase()
	}
	if err := x.Block.Check(); err != nil {
		return err
	}
	return x.Signature.Check(vs.key.Copy(), x, vs)
}

func (x *AggregateSignature) Check(key crypto.MultiPublicKeyI, sb SignByte, vs ValidatorSet) (err lib.ErrorI) {
	if x == nil {
		return ErrEmptyAggregateSignature()
	}
	if len(x.Signature) != crypto.BLS12381SignatureSize {
		return ErrInvalidAggrSignatureLength()
	}
	if x.Bitmap == nil {
		return ErrEmptySignerBitmap()
	}
	if er := key.SetBitmap(x.Bitmap); err != nil {
		return ErrInvalidSignerBitmap(er)
	}
	totalSignedPower := "0"
	for i, val := range vs.validatorSet.ValidatorSet {
		signed, er := key.SignerEnabledAt(i)
		if er != nil {
			return ErrInvalidSignerBitmap(er)
		}
		if signed {
			totalSignedPower, err = lib.StringAdd(totalSignedPower, val.VotingPower)
			if err != nil {
				return err
			}
		}
	}
	hasMaj23, err := lib.StringsGTE(totalSignedPower, vs.minimumPowerForQuorum)
	if err != nil {
		return err
	}
	if !hasMaj23 {
		return ErrNoMaj23()
	}
	msg, err := sb.SignBytes()
	if err != nil {
		return err
	}
	if !key.VerifyBytes(msg, x.Signature) {
		return ErrInvalidAggrSignature()
	}
	return nil
}

func (x *View) Check(view *View) lib.ErrorI {
	if x == nil {
		return ErrEmptyView()
	}
	if x.Round >= maxRounds {
		return ErrWrongRound()
	}
	if x.Height != view.Height {
		return ErrWrongHeight()
	}
	return nil
}

func (x *View) Copy() *View {
	return &View{
		Height: x.Height,
		Round:  x.Round,
		Phase:  x.Phase,
	}
}

func (x *View) Less(v *View) bool {
	if v == nil {
		return false
	}
	if x == nil {
		return true
	}
	if x.Height < v.Height || x.Round < v.Round || x.Phase < v.Phase {
		return true
	}
	return false
}

type Signable interface {
	proto.Message
	Sign(p crypto.PrivateKeyI) lib.ErrorI
}

type SignByte interface{ SignBytes() ([]byte, lib.ErrorI) }

func checkSignature(signature *lib.Signature, sb SignByte) lib.ErrorI {
	if err := checkSignatureBasic(signature); err != nil {
		return err
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

func publicKeyFromBytes(pubKey []byte) (crypto.PublicKeyI, lib.ErrorI) {
	publicKey, err := crypto.NewBLSPublicKeyFromBytes(pubKey)
	if err != nil {
		return nil, ErrPubKeyFromBytes(err)
	}
	return publicKey, nil
}
