package lib

import (
	"bytes"
	"encoding/json"
	"github.com/alecthomas/units"
	"github.com/canopy-network/canopy/lib/crypto"
)

const (
	GlobalMaxBlockSize = int(32 * units.MB)
)

// QUORUM CERTIFICATE CODE BELOW

// SignBytes() returns the canonical byte representation used to digitally sign the bytes of the structure
func (x *QuorumCertificate) SignBytes() (signBytes []byte) {
	if x.Header != nil && x.Header.Phase == Phase_ELECTION_VOTE {
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
			return ErrInvalidResultsHash()
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
				return ErrMismatchBlockHash()
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
	if stateCommitteeHeight > x.Header.CanopyHeight {
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
