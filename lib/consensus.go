package lib

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func (x *QuorumCertificate) SignBytes() (signBytes []byte) {
	proposal, aggregateSignature := x.Proposal, x.Signature
	x.Proposal, x.Signature = nil, nil
	signBytes, _ = Marshal(x)
	x.Proposal, x.Signature = proposal, aggregateSignature
	return
}

func (x *QuorumCertificate) Check(vs ValidatorSet, height ...uint64) (isPartialQC bool, error ErrorI) {
	if x == nil {
		return false, ErrEmptyQuorumCertificate()
	}
	if err := x.Header.Check(height...); err != nil {
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
	if x.Header.Phase != Phase_PRECOMMIT_VOTE {
		return ErrWrongPhase()
	}
	if x.Proposal == nil {
		return ErrNilProposal()
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
	if !bytes.Equal(x.Proposal, qc.Proposal) {
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
	block := new(Block)
	_ = Unmarshal(x.Proposal, block)
	return json.Marshal(jsonQC{
		Header:      x.Header,
		Block:       block,
		BlockHash:   x.ProposalHash,
		ProposerKey: x.ProposerKey,
		Signature:   x.Signature,
	})
}

func (x *QuorumCertificate) UnmarshalJSON(b []byte) (err error) {
	var j jsonQC
	if err = json.Unmarshal(b, &j); err != nil {
		return
	}
	x.Proposal, _ = Marshal(j.Block)
	x.Header, x.ProposalHash = j.Header, j.BlockHash
	x.ProposerKey, x.Signature = j.ProposerKey, j.Signature
	return nil
}

const MaxRound = 1000

func (x *View) Check(height ...uint64) ErrorI {
	if x == nil {
		return ErrEmptyView()
	}
	if x.Round >= MaxRound {
		return ErrWrongRound()
	}
	if height != nil && x.Height != height[0] {
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

// nolint:all
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
