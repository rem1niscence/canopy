package types

import (
	"bytes"
	"github.com/ginchuco/ginchu/types/crypto"
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

type ByzantineEvidence struct {
	DSE DoubleSignEvidences
	BPE BadProposerEvidences
}

type DoubleSignEvidences []*DoubleSignEvidence

func (e *DoubleSignEvidences) Add(app App, evidence *DoubleSignEvidence) {
	if evidence == nil {
		return
	}
	if err := evidence.Check(app); err != nil {
		return
	}
	if err := evidence.VoteA.CheckEvidence(app); err != nil { // TODO code de-dup
		return
	}
	if err := evidence.VoteB.CheckEvidence(app); err != nil {
		return
	}
	if equals := evidence.VoteA.Equals(evidence.VoteB); equals {
		return
	}
	duplicate := make(map[string]struct{})
	for _, ev := range *e {
		bz, _ := Marshal(ev)
		key := BytesToString(bz)
		duplicate[key] = struct{}{}
		bz = ev.FlippedBytes()
		key = BytesToString(bz)
		duplicate[key] = struct{}{}
	}
	bz, _ := Marshal(evidence)
	key1 := BytesToString(bz)
	bz = evidence.FlippedBytes()
	key2 := BytesToString(bz)
	if _, isDuplicate := duplicate[key1]; !isDuplicate {
		if _, isDuplicate = duplicate[key2]; !isDuplicate {
			*e = append(*e, evidence)
		}
	}
}

func (e *DoubleSignEvidences) FilterBad(app App) ErrorI {
	if e == nil {
		return ErrEmptyEvidence()
	}
	var goodEvidences DoubleSignEvidences
	for _, evidence := range *e {
		if evidence == nil {
			continue
		}
		if err := evidence.Check(app); err != nil { // TODO de-dup
			continue
		}
		if err := evidence.VoteA.CheckEvidence(app); err != nil {
			continue
		}
		if err := evidence.VoteB.CheckEvidence(app); err != nil {
			continue
		}
		if equals := evidence.VoteA.Equals(evidence.VoteB); equals {
			continue
		}
		goodEvidences = append(goodEvidences, evidence)
	}
	*e = goodEvidences
	return nil
}

func (e DoubleSignEvidences) GetDoubleSigners(app App) (pubKeys [][]byte, error ErrorI) {
	if err := e.FilterBad(app); err != nil {
		return nil, err
	}
	if len(e) == 0 {
		return
	}
	// one infraction per view
	deDupMap := make(map[string]map[string]struct{}) // view -> map[pubkey]
	for _, evidence := range e {
		if err := evidence.Check(app); err != nil {
			return nil, err
		}
		viewBytes, err := Marshal(evidence.VoteA.Header)
		if err != nil {
			return nil, err
		}
		viewKey := BytesToString(viewBytes)
		if _, ok := deDupMap[viewKey]; !ok {
			deDupMap[viewKey] = make(map[string]struct{})
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
		for _, ds := range doubleSigners {
			if _, ok := deDupMap[viewKey][BytesToString(ds)]; ok {
				continue
			} else {
				// add to de-dup map
				m := deDupMap[viewKey]
				m[BytesToString(ds)] = struct{}{}
				deDupMap[viewKey] = m
				// add to infraction
				pubKeys = append(pubKeys, ds)
			}
		}
	}
	return
}

type BadProposerEvidences []*BadProposerEvidence

func (bpe *BadProposerEvidences) Add(expectedLeader []byte, height uint64, vs ValidatorSetWrapper, evidence *BadProposerEvidence) {
	if evidence == nil {
		return
	}
	isPartialQC, err := evidence.ElectionVoteQc.Check(&View{Height: height}, vs)
	if err != nil {
		return
	}
	if isPartialQC {
		return
	}
	if bytes.Equal(evidence.ElectionVoteQc.LeaderPublicKey, expectedLeader) {
		return
	}
	duplicate := make(map[string]struct{})
	for _, e := range *bpe {
		bz, _ := Marshal(e.ElectionVoteQc.Header)
		key := BytesToString(bz) + BytesToString(e.ElectionVoteQc.LeaderPublicKey)
		duplicate[key] = struct{}{}
	}
	bz, _ := Marshal(evidence.ElectionVoteQc.Header)
	key := BytesToString(bz) + BytesToString(evidence.ElectionVoteQc.LeaderPublicKey)
	if _, isDuplicate := duplicate[key]; !isDuplicate {
		*bpe = append(*bpe, evidence)
	}
}

func (bpe *BadProposerEvidences) FilterBad(expectedLeader []byte, height uint64, vs ValidatorSetWrapper) ErrorI {
	if bpe == nil {
		return ErrEmptyEvidence()
	}
	var goodEvidence BadProposerEvidences
	for _, evidence := range *bpe {
		if evidence == nil {
			continue
		}
		isPartialQC, err := evidence.ElectionVoteQc.Check(&View{Height: height}, vs)
		if err != nil {
			continue
		}
		if isPartialQC {
			continue
		}
		if bytes.Equal(evidence.ElectionVoteQc.LeaderPublicKey, expectedLeader) {
			continue
		}
		goodEvidence = append(goodEvidence, evidence)
	}
	*bpe = goodEvidence
	return nil
}

func (bpe BadProposerEvidences) GetBadProposers(expectedLeader []byte, height uint64, vs ValidatorSetWrapper) (pubKeys [][]byte, error ErrorI) {
	if err := bpe.FilterBad(expectedLeader, height, vs); err != nil {
		return nil, err
	}
	deDupMap := make(map[string]map[string]struct{}) // view -> map[pubkey]
	for _, bp := range bpe {
		isPartialQC, err := bp.ElectionVoteQc.Check(&View{Height: height}, vs)
		if err != nil {
			continue // log error?
		}
		if isPartialQC {
			continue
		}
		if bp.ElectionVoteQc.Header.Phase != Phase_ELECTION_VOTE {
			continue
		}
		viewBytes, err := Marshal(bp.ElectionVoteQc.Header)
		if err != nil {
			return nil, err
		}
		viewKey := BytesToString(viewBytes)
		if _, ok := deDupMap[viewKey]; !ok {
			deDupMap[viewKey] = make(map[string]struct{})
		}
		leader := bp.ElectionVoteQc.LeaderPublicKey
		if _, ok := deDupMap[viewKey][BytesToString(leader)]; ok {
			continue
		} else {
			// add to de-dup map
			m := deDupMap[viewKey]
			m[BytesToString(leader)] = struct{}{}
			deDupMap[viewKey] = m
			// add to infraction
			pubKeys = append(pubKeys, leader)
		}
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

func (x *AggregateSignature) CheckBasic(sb SignByte, vs ValidatorSetWrapper) ErrorI {
	if x == nil {
		return ErrEmptyAggregateSignature()
	}
	if len(x.Signature) != crypto.BLS12381SignatureSize {
		return ErrInvalidAggrSignatureLength()
	}
	if len(x.Bitmap) == 0 {
		return ErrEmptySignerBitmap()
	}
	key := vs.Key.Copy()
	if er := key.SetBitmap(x.Bitmap); er != nil {
		return ErrInvalidSignerBitmap(er)
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

func (x *AggregateSignature) Check(sb SignByte, vs ValidatorSetWrapper) (isPartialQC bool, err ErrorI) {
	if err = x.CheckBasic(sb, vs); err != nil {
		return false, err
	}
	// check 2/3 maj
	key := vs.Key.Copy()
	if er := key.SetBitmap(x.Bitmap); er != nil {
		return false, ErrInvalidSignerBitmap(er)
	}
	totalSignedPower := "0"
	for i, val := range vs.ValidatorSet.ValidatorSet {
		signed, er := key.SignerEnabledAt(i)
		if er != nil {
			return false, ErrInvalidSignerBitmap(er)
		}
		if signed {
			totalSignedPower, err = StringAdd(totalSignedPower, val.VotingPower)
			if err != nil {
				return false, err
			}
		}
	}
	hasMaj23, err := StringsGTE(totalSignedPower, vs.MinimumMaj23)
	if err != nil {
		return false, err
	}
	if !hasMaj23 {
		return true, nil
	}
	return false, nil
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

func (x *AggregateSignature) GetNonSigners(valSet *ValidatorSet) (nonSigners [][]byte, nonSignerPercent int, err ErrorI) {
	vs, err := NewValidatorSet(valSet)
	if err != nil {
		return nil, 0, err
	}
	key := vs.Key.Copy()
	if er := key.SetBitmap(x.Bitmap); er != nil {
		return nil, 0, ErrInvalidSignerBitmap(er)
	}
	nonSignerPower := "0"
	for i, val := range vs.ValidatorSet.ValidatorSet {
		signed, er := key.SignerEnabledAt(i)
		if er != nil {
			return nil, 0, ErrInvalidSignerBitmap(er)
		}
		if !signed {
			nonSigners = append(nonSigners, val.PublicKey)
			nonSignerPower, err = StringAdd(nonSignerPower, val.VotingPower)
			if err != nil {
				return nil, 0, err
			}
		}
	}
	nonSignerPercent, err = StringPercentDiv(nonSignerPower, vs.TotalPower)
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
