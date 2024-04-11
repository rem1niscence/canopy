package types

import (
	"bytes"
	"github.com/ginchuco/ginchu/crypto"
	"google.golang.org/protobuf/proto"
)

const MaxRound = 1000

func (x *QuorumCertificate) SignBytes() (signBytes []byte, err ErrorI) {
	aggregateSignature := x.Signature
	block := x.Block
	if x.Header.Phase == Phase_PROPOSE_VOTE { // omit because in EV no block was included in the message, but proposer inserts one in PROPOSE
		x.Block = nil
	}
	x.Signature = nil
	signBytes, err = Marshal(x)
	x.Signature = aggregateSignature
	x.Block = block
	return
}

func (x *QuorumCertificate) Check(view *View, vs ValidatorSetWrapper) ErrorI {
	if x == nil {
		return ErrEmptyQuorumCertificate()
	}
	if err := x.Header.Check(view); err != nil {
		return err
	}
	return x.Signature.Check(x, vs)
}

func (x *QuorumCertificate) CheckHighQC(view *View, vs ValidatorSetWrapper) ErrorI {
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
	return x.Signature.Check(x, vs)
}

func (x *QuorumCertificate) Equals(qc *QuorumCertificate) bool {
	if x == nil || qc == nil {
		return false
	}
	if !x.Header.Equals(qc.Header) {
		return false
	}
	if !bytes.Equal(x.LeaderPublicKey, qc.LeaderPublicKey) {
		return false
	}
	if !x.Block.Equals(qc.Block) {
		return false
	}
	return x.Signature.Equals(qc.Signature)
}

func (x *QuorumCertificate) GetNonSigners(vs *ValidatorSet) ([][]byte, ErrorI) {
	return x.Signature.GetNonSigners(vs)
}

func (x *DoubleSignEvidence) CheckBasic() ErrorI {
	if x == nil {
		return ErrEmptyEvidence()
	}
	if x.VoteA == nil || x.VoteB == nil || x.VoteA.Header == nil || x.VoteB.Header == nil {
		return ErrEmptyQuorumCertificate()
	}
	return nil
}

func (x *DoubleSignEvidence) Check(app App) ErrorI {
	if err := x.CheckBasic(); err != nil {
		return err
	}
	valSet, err := app.GetBeginStateValSet(x.VoteA.Header.Height)
	if err != nil {
		return err
	}
	vs, err := NewValidatorSet(valSet)
	if err != nil {
		return err
	}
	if err = x.VoteA.Check(x.VoteA.Header, vs); err != nil {
		return err
	}
	if err = x.VoteB.Check(x.VoteB.Header, vs); err != nil {
		return err
	}
	if x.VoteA.Header.Equals(x.VoteB.Header) && !x.VoteA.Equals(x.VoteB) {
		return nil
	}
	exists, err := app.EvidenceExists(x)
	if err != nil {
		return err
	}
	if exists {
		return ErrDuplicateEvidence()
	}
	return ErrInvalidEvidence()
}

func (x *DoubleSignEvidence) Equals(y *DoubleSignEvidence) bool {
	if x == nil || y == nil {
		return false
	}
	if x.VoteA.Equals(y.VoteA) && x.VoteB.Equals(y.VoteB) {
		return true
	}
	if x.VoteB.Equals(y.VoteA) && x.VoteA.Equals(y.VoteB) {
		return true
	}
	return false
}

type Evidence []*DoubleSignEvidence

func (e Evidence) GetDoubleSigners(app App) (pubKeys [][]byte, error ErrorI) {
	if e == nil {
		return nil, nil
	}
	for _, evidence := range e {
		if err := evidence.Check(app); err != nil {
			return nil, err
		}
		height := evidence.VoteA.Header.Height
		aggSig1 := evidence.VoteA.Signature
		aggSig2 := evidence.VoteB.Signature
		valSet, err := app.GetBeginStateValSet(height)
		if err != nil {
			return nil, err
		}
		doubleSigners, err := aggSig1.GetDoubleSigners(aggSig2, valSet)
		if err != nil {
			return nil, err
		}
		pubKeys = append(pubKeys, doubleSigners...)
	}
	return
}

func (x *AggregateSignature) Equals(a2 *AggregateSignature) bool {
	if x == nil || a2 == nil {
		return false
	}
	if !bytes.Equal(x.Signature, a2.Signature) {
		return false
	}
	if !bytes.Equal(x.Bitmap, a2.Bitmap) {
		return false
	}
	return true
}

func (x *AggregateSignature) Check(sb SignByte, vs ValidatorSetWrapper) (err ErrorI) {
	if x == nil {
		return ErrEmptyAggregateSignature()
	}
	if len(x.Signature) != crypto.BLS12381SignatureSize {
		return ErrInvalidAggrSignatureLength()
	}
	if x.Bitmap == nil {
		return ErrEmptySignerBitmap()
	}
	key := vs.Key.Copy()
	if er := key.SetBitmap(x.Bitmap); er != nil {
		return ErrInvalidSignerBitmap(er)
	}
	totalSignedPower := "0"
	for i, val := range vs.ValidatorSet.ValidatorSet {
		signed, er := key.SignerEnabledAt(i)
		if er != nil {
			return ErrInvalidSignerBitmap(er)
		}
		if signed {
			totalSignedPower, err = StringAdd(totalSignedPower, val.VotingPower)
			if err != nil {
				return err
			}
		}
	}
	hasMaj23, err := StringsGTE(totalSignedPower, vs.MinimumMaj23)
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

func (x *AggregateSignature) GetDoubleSigners(y *AggregateSignature, valSet *ValidatorSet) (doubleSigners [][]byte, err ErrorI) {
	vs, err := NewValidatorSet(valSet)
	if err != nil {
		return nil, err
	}
	key, key2 := vs.Key.Copy(), vs.Key.Copy()
	if er := key.SetBitmap(x.Bitmap); er != nil {
		return nil, ErrInvalidSignerBitmap(er)
	}
	if er := key2.SetBitmap(y.Bitmap); er != nil {
		return nil, ErrInvalidSignerBitmap(er)
	}
	for i, val := range vs.ValidatorSet.ValidatorSet {
		signed, er := key.SignerEnabledAt(i)
		if er != nil {
			return nil, ErrInvalidSignerBitmap(er)
		}
		if signed {
			signed, er = key2.SignerEnabledAt(i)
			if er != nil {
				return nil, ErrInvalidSignerBitmap(er)
			}
			if signed {
				doubleSigners = append(doubleSigners, val.PublicKey)
			}
		}
	}
	return
}

func (x *AggregateSignature) GetNonSigners(valSet *ValidatorSet) (nonSigners [][]byte, err ErrorI) {
	vs, err := NewValidatorSet(valSet)
	if err != nil {
		return nil, err
	}
	key := vs.Key.Copy()
	if er := key.SetBitmap(x.Bitmap); er != nil {
		return nil, ErrInvalidSignerBitmap(er)
	}
	for i, val := range vs.ValidatorSet.ValidatorSet {
		signed, er := key.SignerEnabledAt(i)
		if er != nil {
			return nil, ErrInvalidSignerBitmap(er)
		}
		if !signed {
			nonSigners = append(nonSigners, val.PublicKey)
		}
	}
	return
}

func (x *View) Check(view *View) ErrorI {
	if x == nil {
		return ErrEmptyView()
	}
	if x.Round >= MaxRound {
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

func (x *View) Equals(v *View) bool {
	if x == nil || v == nil {
		return false
	}
	if x.Height != v.Height {
		return false
	}
	if x.Round != v.Round {
		return false
	}
	if x.Phase != v.Phase {
		return false
	}
	return true
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
	Sign(p crypto.PrivateKeyI) ErrorI
}

type SignByte interface{ SignBytes() ([]byte, ErrorI) }
