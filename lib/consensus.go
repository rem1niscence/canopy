package lib

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/alecthomas/units"
	"github.com/canopy-network/canopy/lib/crypto"
	"slices"
)

const (
	GlobalMaxBlockSize = int(32 * units.MB)
)

// QUORUM CERTIFICATE CODE BELOW

// SignBytes() returns the canonical byte representation used to digitally sign the bytes of the structure
func (x *QuorumCertificate) SignBytes() (signBytes []byte) {
	if x.ProposerKey != nil {
		bz, _ := Marshal(&QuorumCertificate{Header: x.Header, ProposerKey: x.ProposerKey})
		return bz
	}
	// temp variables to save values
	results, block, aggregateSignature := x.Results, x.Block, x.Signature
	// remove the values from the struct
	x.Results, x.Block, x.Signature = nil, nil, nil
	// convert the structure into the sign bytes
	signBytes, _ = Marshal(x)
	// add back the removed values
	x.Results, x.Block, x.Signature = results, block, aggregateSignature
	return
}

// CheckBasic() performs 'sanity' checks on the Quorum Certificate structure
// height may be optionally passed for View checking
func (x *QuorumCertificate) CheckBasic() ErrorI {
	// a valid QC must have either the proposal hash or the proposer key set
	if x == nil || (x.ResultsHash == nil && x.ProposerKey == nil) {
		return ErrEmptyQuorumCertificate()
	}
	// sanity check the view of the QC
	if err := x.Header.CheckBasic(); err != nil {
		return err
	}
	// is QC with result (AFTER ELECTION)
	if x.ResultsHash != nil {
		// sanity check the hashes
		if len(x.BlockHash) != crypto.HashSize {
			return ErrInvalidBlockHash()
		}
		if len(x.ResultsHash) != crypto.HashSize {
			return ErrInvalidBlockHash()
		}
		// results may be omitted in certain cases like for integrated blockchain block storage
		if x.Results != nil {
			if err := x.Results.CheckBasic(); err != nil {
				return err
			}
			// validate the ProposalHash = the hash of the proposal sign bytes
			resultsBytes, err := Marshal(x.Results)
			if err != nil {
				return err
			}
			// check the results hash
			if !bytes.Equal(x.ResultsHash, crypto.Hash(resultsBytes)) {
				return ErrMismatchResultsHash()
			}
		}
		// block may be omitted in certain cases like the 'reward transaction'
		if x.Block != nil {
			// check the block hash
			if !bytes.Equal(x.BlockHash, crypto.Hash(x.Block)) {
				return ErrInvalidBlockHash()
			}
			blockSize := len(x.Block)
			// global max block size enforcement
			if blockSize > GlobalMaxBlockSize {
				return ErrExpectedMaxBlockSize()
			}
		}
	} else { // is QC with proposer key (ELECTION)
		if len(x.ProposerKey) != crypto.BLS12381PubKeySize {
			return ErrInvalidProposerPubKey()
		}
		if len(x.ResultsHash) != 0 || x.Results != nil {
			return ErrMismatchResultsHash()
		}
		if len(x.BlockHash) != 0 || len(x.Block) != 0 {
			return ErrMismatchBlockHash()
		}
	}
	// ensure a valid aggregate signature is possible
	return x.Signature.CheckBasic()
}

// Check() validates the QC by cross-checking the aggregate signature against the ValidatorSet
// isPartialQC means a valid aggregate signature, but not enough signers for +2/3 majority
func (x *QuorumCertificate) Check(vs ValidatorSet, maxBlockSize int, view *View, enforceHeights bool) (isPartialQC bool, error ErrorI) {
	if err := x.CheckBasic(); err != nil {
		return false, err
	}
	if err := x.Header.Check(view, enforceHeights); err != nil {
		return false, err
	}
	if x.Block != nil {
		blockSize := len(x.Block)
		// global max block size enforcement
		if blockSize > maxBlockSize {
			return false, ErrExpectedMaxBlockSize()
		}
	}
	return x.Signature.Check(x, vs)
}

// CheckHighQC() performs additional validation on the special `HighQC` (justify unlock QC)
func (x *QuorumCertificate) CheckHighQC(maxBlockSize int, view *View, stateCommitteeHeight uint64, vs ValidatorSet) ErrorI {
	isPartialQC, err := x.Check(vs, maxBlockSize, view, false)
	if err != nil {
		return err
	}
	// `highQCs` can't justify an unlock without +2/3 majority
	if isPartialQC {
		return ErrNoMaj23()
	}
	// invalid 'historical committee', must be before the last committee height saved in the state
	// if not, there is a potential for a long range attack
	if stateCommitteeHeight > x.Header.CommitteeHeight {
		return ErrInvalidQCCommitteeHeight()
	}
	// enforce same target height
	if x.Header.Height != view.Height {
		return ErrWrongHeight()
	}
	// a valid HighQC must have the phase must be PRECOMMIT_VOTE
	// as that's the phase where replicas 'Lock'
	if x.Header.Phase != Phase_PRECOMMIT_VOTE {
		return ErrWrongPhase()
	}
	// the block hash nor results hash cannot be nil for a HighQC
	// as it's after the election phase
	if x.BlockHash == nil || x.ResultsHash == nil {
		return ErrNilBlock()
	}
	return nil
}

// Equals() checks the equality of the current QC against the parameter QC
// equals rejects nil QCs
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
	if !bytes.Equal(x.BlockHash, qc.BlockHash) {
		return false
	}
	if !bytes.Equal(x.Block, qc.Block) {
		return false
	}
	if !bytes.Equal(x.ResultsHash, qc.ResultsHash) {
		return false
	}
	if !x.Results.Equals(qc.Results) {
		return false
	}
	return x.Signature.Equals(qc.Signature)
}

// GetNonSigners() returns the public keys and the percentage (of voting power out of total) of those who did not sign the QC
func (x *QuorumCertificate) GetNonSigners(vs *ConsensusValidators) (nonSigners [][]byte, nonSignerPercent int, err ErrorI) {
	if x == nil || x.Signature == nil {
		return nil, 0, ErrEmptyQuorumCertificate()
	}
	return x.Signature.GetNonSigners(vs)
}

// jsonQC represents the json.Marshaller and json.Unmarshaler implementation of QC
type jsonQC struct {
	Header       *View               `json:"header,omitempty"`
	Block        HexBytes            `json:"block,omitempty"`
	BlockHash    HexBytes            `json:"blockHash,omitempty"`
	ResultsHash  HexBytes            `json:"resultsHash,omitempty"`
	Results      *CertificateResult  `json:"results,omitempty"`
	ProposalHash HexBytes            `json:"block_hash,omitempty"`
	ProposerKey  HexBytes            `json:"proposer_key,omitempty"`
	Signature    *AggregateSignature `json:"signature,omitempty"`
}

// MarshalJSON() implements the json.Marshaller interface
func (x QuorumCertificate) MarshalJSON() ([]byte, error) {
	return json.Marshal(jsonQC{
		Header:      x.Header,
		Results:     x.Results,
		ResultsHash: x.ResultsHash,
		Block:       x.Block,
		BlockHash:   x.BlockHash,
		ProposerKey: x.ProposerKey,
		Signature:   x.Signature,
	})
}

// UnmarshalJSON() implements the json.Unmarshaler interface
func (x *QuorumCertificate) UnmarshalJSON(b []byte) (err error) {
	var j jsonQC
	if err = json.Unmarshal(b, &j); err != nil {
		return
	}
	*x = QuorumCertificate{
		Header:      j.Header,
		Results:     j.Results,
		ResultsHash: j.ResultsHash,
		Block:       j.Block,
		BlockHash:   j.BlockHash,
		ProposerKey: j.ProposerKey,
		Signature:   j.Signature,
	}
	return nil
}

// VIEW CODE BELOW

const MaxRound = 1000 // max round is arbitrarily chosen and may be modified safely

func (x *View) CheckBasic() ErrorI {
	if x == nil {
		return ErrEmptyView()
	}
	// round and phase are not further checked,
	// because peers may be sending valid messages
	// asynchronously from different views
	if x.Round >= MaxRound {
		return ErrWrongRound()
	}
	return nil
}

// Check() checks the validity of the view and optionally enforce *heights* (plugin height and committee height)
func (x *View) Check(view *View, enforceHeights bool) ErrorI {
	if err := x.CheckBasic(); err != nil {
		return err
	}
	if view.NetworkId != x.NetworkId {
		return ErrWrongNetworkID()
	}
	if view.CommitteeId != x.CommitteeId {
		return ErrWrongCommitteeID()
	}
	if enforceHeights && x.Height != view.Height {
		return ErrWrongHeight()
	}
	if enforceHeights && x.CommitteeHeight != view.CommitteeHeight {
		return ErrWrongCommitteeHeight()
	}
	return nil
}

// Copy() returns a reference to a clone of the View
func (x *View) Copy() *View {
	return &View{
		Height:          x.Height,
		Round:           x.Round,
		Phase:           x.Phase,
		CommitteeHeight: x.CommitteeHeight,
		NetworkId:       x.NetworkId,
		CommitteeId:     x.CommitteeId,
	}
}

// Equals() returns true if this view is equal to the parameter view
// nil views are always false
func (x *View) Equals(v *View) bool {
	if x == nil || v == nil {
		return false
	}
	if x.Height != v.Height {
		return false
	}
	if x.CommitteeHeight != v.CommitteeHeight {
		return false
	}
	if x.CommitteeId != v.CommitteeId {
		return false
	}
	if x.NetworkId != v.NetworkId {
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

// Less() returns true if this View is less than the parameter View
func (x *View) Less(v *View) bool {
	if v == nil {
		return false
	}
	if x == nil {
		return true
	}
	// if height is less
	if x.Height < v.Height {
		return true
	}
	if x.Height > v.Height {
		return false
	}
	// if Canopy height is less
	if x.CommitteeHeight < v.CommitteeHeight {
		return true
	}
	if x.CommitteeHeight > v.CommitteeHeight {
		return false
	}
	// if round is less
	if x.Round < v.Round {
		return true
	}
	if x.Round > v.Round {
		return false
	}
	// if phase is less
	if x.Phase < v.Phase {
		return true
	}
	return false
}

// ToString() returns the log string format of View
func (x *View) ToString() string {
	return fmt.Sprintf("(H:%d, CH:%d, R:%d, P:%s)", x.Height, x.CommitteeHeight, x.Round, x.Phase)
}

// jsonView represents the json.Marshaller and json.Unmarshaler implementation of View
type jsonView struct {
	Height          uint64 `json:"height"`
	CommitteeHeight uint64 `json:"committeeHeight"`
	Round           uint64 `json:"round"`
	Phase           string `json:"phase"` // string version of phase
	NetworkID       uint64 `json:"networkID"`
	CommitteeID     uint64 `json:"committeeID"`
}

// MarshalJSON() implements the json.Marshaller interface
func (x View) MarshalJSON() ([]byte, error) {
	return json.Marshal(jsonView{
		Height:          x.Height,
		CommitteeHeight: x.CommitteeHeight,
		Round:           x.Round,
		Phase:           Phase_name[int32(x.Phase)],
		NetworkID:       x.NetworkId,
		CommitteeID:     x.CommitteeId,
	})
}

// MarshalJSON() implements the json.Marshaller interface
func (x *Phase) MarshalJSON() ([]byte, error) {
	return json.Marshal(Phase_name[int32(*x)])
}

// UnmarshalJSON() implements the json.Unmarshaler interface
func (x *Phase) UnmarshalJSON(b []byte) (err error) {
	j := new(string)
	if err = json.Unmarshal(b, j); err != nil {
		return
	}
	*x = Phase(Phase_value[*j])
	return
}

// DOUBLE SIGNER CODE BELOW

// AddHeight() adds a height to the DoubleSigner
func (x *DoubleSigner) AddHeight(height uint64) {
	for _, h := range x.Heights {
		if h == height {
			return
		}
	}
	x.Heights = append(x.Heights, height)
}

// Equals() compares this DoubleSigner against the passed DoubleSigner
func (x *DoubleSigner) Equals(d *DoubleSigner) bool {
	if x == nil && d == nil {
		return true
	}
	if x == nil || d == nil {
		return false
	}
	if !bytes.Equal(x.PubKey, d.PubKey) {
		return false
	}
	return slices.Equal(x.Heights, d.Heights)
}

// MarshalJSON() implements the json.Marshaller interface
func (x *VDF) MarshalJSON() ([]byte, error) {
	return json.Marshal(&jsonVDF{
		Proof:      x.Proof,
		Iterations: x.Iterations,
	})
}

// UnmarshalJSON() implements the json.Unmarshaler interface
func (x *VDF) UnmarshalJSON(b []byte) error {
	j := new(jsonVDF)
	if err := json.Unmarshal(b, j); err != nil {
		return err
	}
	x.Proof, x.Iterations = j.Proof, j.Iterations
	return nil
}

// jsonVDF represents the json.Marshaller and json.Unmarshaler implementation of VDF
type jsonVDF struct {
	Proof      HexBytes `json:"proof,omitempty"`      // proof of function completion given a specific seed
	Iterations uint64   `json:"iterations,omitempty"` // number of iterations (proxy for time)
}
