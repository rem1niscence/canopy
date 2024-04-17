package types

import (
	"bytes"
)

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

func (x *QuorumCertificate) Check(view *View, vs ValidatorSetWrapper) (isPartialQC bool, error ErrorI) {
	if x == nil {
		return false, ErrEmptyQuorumCertificate()
	}
	if err := x.Header.Check(view); err != nil {
		return false, err
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
	isPartialQC, err := x.Signature.Check(x, vs)
	if err != nil {
		return err
	}
	if isPartialQC {
		return ErrNoMaj23()
	}
	return nil
}

func (x *QuorumCertificate) CheckEvidence(app App) ErrorI {
	if x == nil {
		return ErrEmptyQuorumCertificate()
	}
	height := app.LatestHeight()
	var vs *ValidatorSet
	if err := x.Header.Check(&View{Height: height}); err != nil {
		if err = x.Header.Check(&View{Height: height - 1}); err != nil {
			return err
		}
		vs, err = app.GetBeginStateValSet(height - 1)
		if err != nil {
			return err
		}
	} else {
		vs, err = app.GetBeginStateValSet(height)
		if err != nil {
			return err
		}
	}
	valSet, err := NewValidatorSet(vs)
	if err != nil {
		return err
	}
	if err = x.Signature.CheckBasic(x, valSet); err != nil {
		return err
	}
	return nil
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

func (x *QuorumCertificate) GetNonSigners(vs *ValidatorSet) ([][]byte, int, ErrorI) {
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
	if _, err = x.VoteA.Check(x.VoteA.Header, vs); err != nil {
		return err
	}
	if _, err = x.VoteB.Check(x.VoteB.Header, vs); err != nil {
		return err
	}
	if x.VoteA.Header.Equals(x.VoteB.Header) && !x.VoteA.Equals(x.VoteB) {
		return ErrInvalidEvidence() // different heights
	}
	voteASignBytes, err := x.VoteA.SignBytes()
	if err != nil {
		return err
	}
	voteBSignBytes, err := x.VoteB.SignBytes()
	if err != nil {
		return err
	}
	if bytes.Equal(voteBSignBytes, voteASignBytes) {
		return ErrInvalidEvidence() // same payloads
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

func (x *DoubleSignEvidence) FlippedBytes() (bz []byte) {
	// flip it
	voteA := x.VoteA
	x.VoteA = x.VoteB
	x.VoteB = voteA
	bz, _ = Marshal(x)
	// flip it back
	voteA = x.VoteA
	x.VoteA = x.VoteB
	x.VoteB = voteA
	return
}

const MaxRound = 1000

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
