package lib

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func (x *QuorumCertificate) SignBytes() (signBytes []byte, err ErrorI) {
	aggregateSignature := x.Signature
	block := x.Block
	x.Block = nil
	x.Signature = nil
	signBytes, err = Marshal(x)
	x.Signature = aggregateSignature
	x.Block = block
	return
}

func (x *QuorumCertificate) Check(height uint64, vs ValidatorSet) (isPartialQC bool, error ErrorI) {
	if x == nil {
		return false, ErrEmptyQuorumCertificate()
	}
	if err := x.Header.Check(height); err != nil {
		return false, err
	}
	return x.Signature.Check(x, vs)
}

func (x *QuorumCertificate) CheckHighQC(height uint64, vs ValidatorSet) ErrorI {
	if x == nil {
		return ErrEmptyQuorumCertificate()
	}
	if err := x.Header.Check(height); err != nil {
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

func (x *QuorumCertificate) Equals(qc *QuorumCertificate) bool {
	if x == nil || qc == nil {
		return false
	}
	if !x.Header.Equals(qc.Header) {
		return false
	}
	if !bytes.Equal(x.ProposerKey, qc.ProposerKey) {
		return false
	}
	if !x.Block.Equals(qc.Block) {
		return false
	}
	return x.Signature.Equals(qc.Signature)
}

func (x *QuorumCertificate) GetNonSigners(vs *ConsensusValidators) ([][]byte, int, ErrorI) {
	if x == nil {
		return nil, 0, ErrEmptyQuorumCertificate()
	}
	return x.Signature.GetNonSigners(vs)
}

type jsonQC struct {
	Header      *View               `json:"header,omitempty"`
	Block       *Block              `json:"block,omitempty"`
	BlockHash   HexBytes            `json:"block_hash,omitempty"`
	ProposerKey HexBytes            `json:"proposer_key,omitempty"`
	Signature   *AggregateSignature `json:"signature,omitempty"`
}

// nolint:all
func (x QuorumCertificate) MarshalJSON() ([]byte, error) {
	return json.Marshal(jsonQC{
		Header:      x.Header,
		Block:       x.Block,
		BlockHash:   x.BlockHash,
		ProposerKey: x.ProposerKey,
		Signature:   x.Signature,
	})
}

func (x *QuorumCertificate) UnmarshalJSON(b []byte) error {
	var j jsonQC
	if err := json.Unmarshal(b, &j); err != nil {
		return err
	}
	x.Header, x.Block, x.BlockHash = j.Header, j.Block, j.BlockHash
	x.ProposerKey, x.Signature = j.ProposerKey, j.Signature
	return nil
}

func (x *DoubleSignEvidence) CheckBasic() ErrorI {
	if x == nil {
		return ErrEmptyEvidence()
	}
	if x.VoteA == nil || x.VoteB == nil || x.VoteA.Header == nil || x.VoteB.Header == nil {
		return ErrEmptyQuorumCertificate()
	}
	if !x.VoteA.Header.Equals(x.VoteB.Header) {
		return ErrMismatchEvidenceAndHeader()
	}
	if x.VoteA.Header.Round >= MaxRound {
		return ErrWrongRound()
	}
	return nil
}

func (x *DoubleSignEvidence) Check(vs ValidatorSet, minimumEvidenceHeight uint64) ErrorI {
	if x.VoteA.Header.Height < minimumEvidenceHeight {
		return ErrEvidenceTooOld()
	}
	if _, err := x.VoteA.Check(x.VoteA.Header.Height, vs); err != nil {
		return err
	}
	if _, err := x.VoteB.Check(x.VoteB.Header.Height, vs); err != nil {
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

func (x *View) Check(height uint64) ErrorI {
	if x == nil {
		return ErrEmptyView()
	}
	if x.Round >= MaxRound {
		return ErrWrongRound()
	}
	if height != 0 && x.Height != height {
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

func (x *View) ToString() string {
	return fmt.Sprintf("(H:%d, R:%d, P:%s)", x.Height, x.Round, x.Phase)
}

type jsonView struct {
	Height uint64 `json:"height"`
	Round  uint64 `json:"round"`
	Phase  string `json:"phase"`
}

func (x View) MarshalJSON() ([]byte, error) {
	return json.Marshal(jsonView{
		Height: x.Height,
		Round:  x.Round,
		Phase:  Phase_name[int32(x.Phase)],
	})
}

func (x *Phase) MarshalJSON() ([]byte, error) {
	return json.Marshal(Phase_name[int32(*x)])
}

func (x *Phase) UnmarshalJSON(b []byte) (err error) {
	j := new(string)
	if err = json.Unmarshal(b, j); err != nil {
		return
	}
	*x = Phase(Phase_value[*j])
	return
}

type jsonAggregateSig struct {
	Signature HexBytes `json:"signature,omitempty"`
	Bitmap    HexBytes `json:"bitmap,omitempty"`
}

// nolint:all
func (x AggregateSignature) MarshalJSON() ([]byte, error) {
	return json.Marshal(jsonAggregateSig{
		Signature: x.Signature,
		Bitmap:    x.Bitmap,
	})
}

func (x *AggregateSignature) UnmarshalJSON(b []byte) error {
	var j jsonAggregateSig
	if err := json.Unmarshal(b, &j); err != nil {
		return err
	}
	x.Signature, x.Bitmap = j.Signature, j.Bitmap
	return nil
}
