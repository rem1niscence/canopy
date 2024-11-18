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

// 	A CertificateResult contains Canopy information for what happens to stakeholders as a result of the BFT

// CERTIFICATE RESULT CODE BELOW

// CheckBasic() provides basic 'sanity' checks on the CertificateResult structure
func (x *CertificateResult) CheckBasic() ErrorI {
	if x == nil {
		return ErrNilCertificateResult()
	}
	if err := x.RewardRecipients.CheckBasic(); err != nil {
		return err
	}
	if err := x.SlashRecipients.CheckBasic(); err != nil {
		return err
	}
	if err := x.Orders.CheckBasic(); err != nil {
		return err
	}
	return x.Checkpoint.CheckBasic()
}

// Hash() returns the cryptographic hash of the canonical Sign Bytes of the CertificateResult
func (x *CertificateResult) Hash() []byte {
	bz, _ := Marshal(x)
	return crypto.Hash(bz)
}

// AwardPercents() adds reward distribution PaymentPercent samples to the CertificateResult structure
// NOTE: percents should not exceed 100% in a single sample
func (x *CertificateResult) AwardPercents(percents []*PaymentPercents) ErrorI {
	x.RewardRecipients.NumberOfSamples++
	for _, ep := range percents {
		x.addPercents(ep.Address, ep.Percent)
	}
	return nil
}

// addPercents() is a helper function that adds reward distribution percents on behalf of an address
func (x *CertificateResult) addPercents(address []byte, percent uint64) {
	// check to see if the address already has samples
	for i, ep := range x.RewardRecipients.PaymentPercents {
		if bytes.Equal(address, ep.Address) {
			x.RewardRecipients.PaymentPercents[i].Percent += ep.Percent
			return
		}
	}
	// if not, append a sample to PaymentPercents
	x.RewardRecipients.PaymentPercents = append(x.RewardRecipients.PaymentPercents, &PaymentPercents{
		Address: address,
		Percent: percent,
	})
}

// REWARD RECIPIENT CODE BELOW

// CheckBasic() performs a basic 'sanity check' on the structure
func (x *RewardRecipients) CheckBasic() (err ErrorI) {
	if x == nil {
		return ErrNilRewardRecipients()
	}
	// validate the number of recipients
	paymentRecipientCount := len(x.PaymentPercents)
	// ensure not zero or bigger than 25
	if paymentRecipientCount == 0 || paymentRecipientCount > 25 {
		return ErrPaymentRecipientsCount()
	}
	// validate the percents add up to 100 (or less)
	totalPercent := uint64(0)
	for _, pp := range x.PaymentPercents {
		// ensure each percent isn't nil
		if pp == nil {
			return ErrInvalidPercentAllocation()
		}
		// ensure each percent address is the right size
		if len(pp.Address) != crypto.AddressSize {
			return ErrInvalidAddress()
		}
		// ensure each percent isn't 0
		if pp.Percent == 0 {
			return ErrInvalidPercentAllocation()
		}
		// add to total
		totalPercent += pp.Percent
		// ensure the percent doesn't exceed 100
		if totalPercent > 100 {
			return ErrInvalidPercentAllocation()
		}
	}
	return
}

// jsonRewardRecipients is the RewardRecipients implementation of json.Marshaller and json.Unmarshaler
type jsonRewardRecipients struct {
	PaymentPercents []*PaymentPercents `json:"payment_percents,omitempty"` // recipients of the block reward by percentage
	NumberOfSamples uint64             `json:"number_of_samples,omitempty"`
}

// UnmarshalJSON() satisfies the json.Unmarshaler interface
func (x *RewardRecipients) UnmarshalJSON(i []byte) error {
	j := new(jsonRewardRecipients)
	if err := json.Unmarshal(i, j); err != nil {
		return err
	}
	*x = RewardRecipients{
		PaymentPercents: j.PaymentPercents,
		NumberOfSamples: j.NumberOfSamples,
	}
	return nil
}

// MarshalJSON() satisfies the json.Marshaller interface
func (x *RewardRecipients) MarshalJSON() ([]byte, error) {
	return json.Marshal(jsonRewardRecipients{
		PaymentPercents: x.PaymentPercents,
		NumberOfSamples: x.NumberOfSamples,
	})
}

// PAYMENT PERCENTS CODE BELOW

// paymentPercents is the PaymentPercents implementation of json.Marshaller and json.Unmarshaler
type paymentPercents struct {
	Address  HexBytes `json:"address"`
	Percents uint64   `json:"percents"`
}

// MarshalJSON() satisfies the json.Marshaller interface
func (x *PaymentPercents) MarshalJSON() ([]byte, error) {
	return json.Marshal(paymentPercents{
		Address:  x.Address,
		Percents: x.Percent,
	})
}

// UnmarshalJSON() satisfies the json.Unmarshaler interface
func (x *PaymentPercents) UnmarshalJSON(b []byte) error {
	var ep paymentPercents
	if err := json.Unmarshal(b, &ep); err != nil {
		return err
	}
	x.Address, x.Percent = ep.Address, ep.Percents
	return nil
}

// SLASH RECIPIENTS CODE BELOW

// CheckBasic() validates the ProposalMeta structure
func (x *SlashRecipients) CheckBasic() ErrorI {
	if x != nil {
		for _, r := range x.BadProposers {
			if r == nil {
				return ErrInvalidBadProposer()
			}
		}
		for _, r := range x.DoubleSigners {
			if r == nil {
				return ErrInvalidDoubleSigner()
			}
		}
	}
	return nil
}

// jsonSlashRecipients is the SlashRecipients implementation of json.Marshaller and json.Unmarshaler
type jsonSlashRecipients struct {
	DoubleSigners []*DoubleSigner `json:"double_signers,omitempty"` // who did the bft decide was a double signer
	BadProposers  []HexBytes      `json:"bad_proposers,omitempty"`  // who did the bft decide was a bad proposer
}

// UnmarshalJSON() satisfies the json.Unmarshaler interface
func (x *SlashRecipients) UnmarshalJSON(i []byte) error {
	j := new(jsonSlashRecipients)
	if err := json.Unmarshal(i, j); err != nil {
		return err
	}
	var badProposers [][]byte
	for _, bp := range j.BadProposers {
		badProposers = append(badProposers, bp)
	}
	*x = SlashRecipients{
		DoubleSigners: j.DoubleSigners,
		BadProposers:  badProposers,
	}
	return nil
}

// MarshalJSON() satisfies the json.Marshaller interface
func (x *SlashRecipients) MarshalJSON() ([]byte, error) {
	var badProposers []HexBytes
	for _, bp := range x.BadProposers {
		badProposers = append(badProposers, bp)
	}
	return json.Marshal(jsonSlashRecipients{
		DoubleSigners: x.DoubleSigners,
		BadProposers:  badProposers,
	})
}

// ORDERS CODE BELOW

// CheckBasic() performs stateless validation on an Orders object
func (x *Orders) CheckBasic() ErrorI {
	if x == nil {
		return nil
	}
	// check the buy orders
	for _, buy := range x.BuyOrders {
		if buy == nil {
			return ErrNilBuyOrder()
		}
		if buy.BuyerReceiveAddress == nil {
			return ErrInvalidBuyerReceiveAddress()
		}
	}
	return nil
}

// CHECKPOINT CODE BELOW

// CheckBasic() performs stateless validation on a Checkpoint object
func (x *Checkpoint) CheckBasic() ErrorI {
	if x == nil {
		return nil
	}
	if len(x.BlockHash) > 100 {
		return ErrInvalidBlockHash()
	}
	return nil
}
