package lib

import (
	"bytes"
	"encoding/json"
	"github.com/canopy-network/canopy/lib/crypto"
	"slices"
)

// 	A CertificateResult contains Canopy information for what happens to stakeholders as a result of the BFT

// CERTIFICATE RESULT CODE BELOW

// CheckBasic() provides basic 'sanity' checks on the CertificateResult structure
func (x *CertificateResult) CheckBasic() ErrorI {
	if x == nil {
		return ErrNilProposal()
	}
	if err := x.RewardRecipients.CheckBasic(); err != nil {
		return err
	}
	if err := x.SlashRecipients.CheckBasic(); err != nil {
		return err
	}
	return nil
}

// Check() provides basic 'structural' checks on the CertificateResult
func (x *CertificateResult) Check() (err ErrorI) {
	if err = x.CheckBasic(); err != nil {
		return
	}
	return
}

// Hash() returns the cryptographic hash of the canonical Sign Bytes of the CertificateResult
func (x *CertificateResult) Hash() []byte {
	bz, _ := Marshal(x)
	return crypto.Hash(bz)
}

// Equals() checks the equality of this Proposal against the param Proposal
func (x *CertificateResult) Equals(b *CertificateResult) bool {
	if x == nil && b == nil {
		return true
	}
	if x == nil || b == nil {
		return false
	}
	if !x.SlashRecipients.Equals(b.SlashRecipients) {
		return false
	}
	return x.RewardRecipients.Equals(b.RewardRecipients)
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
func (x *RewardRecipients) CheckBasic() ErrorI {
	if x == nil {
		return ErrNilRewardRecipients()
	}
	return CheckPaymentPercentsSample(x.PaymentPercents)
}

// Equals() checks the equality of this RewardRecipients against the param RewardRecipients
func (x *RewardRecipients) Equals(p *RewardRecipients) bool {
	if x == nil && p == nil {
		return true
	}
	if x == nil || p == nil {
		return false
	}
	if !slices.Equal(x.PaymentPercents, p.PaymentPercents) {
		return false
	}
	return x.NumberOfSamples == p.NumberOfSamples
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

// CheckPaymentPercentsSample() validates the list of PaymentPercents structures
// CONTRACT: this function only works for a single sample
// if multiple samples combined, this function will error
func CheckPaymentPercentsSample(percent []*PaymentPercents) ErrorI {
	numProposalRecipients := len(percent)
	if numProposalRecipients == 0 || numProposalRecipients > 25 {
		return ErrInvalidNumOfRecipients()
	}
	totalPercent := uint64(0)
	for _, ep := range percent {
		if ep == nil {
			return ErrInvalidPercentAllocation()
		}
		if len(ep.Address) != crypto.AddressSize {
			return ErrInvalidAddress()
		}
		if ep.Percent == 0 {
			return ErrInvalidPercentAllocation()
		}
		totalPercent += ep.Percent
		if totalPercent > 100 {
			return ErrInvalidPercentAllocation()
		}
	}
	return nil
}

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

// PROPOSAL META CODE BELOW

// Check() validates the ProposalMeta structure
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

// Equals() checks the equality of this ProposalMeta against the param ProposalMeta
func (x *SlashRecipients) Equals(p *SlashRecipients) bool {
	if x == nil && p == nil {
		return true
	}
	if x == nil || p == nil {
		return false
	}
	if !slices.EqualFunc(x.BadProposers, p.BadProposers, func(a []byte, b []byte) bool {
		return bytes.Equal(a, b)
	}) {
		return false
	}
	return slices.EqualFunc(x.DoubleSigners, p.DoubleSigners, func(a *DoubleSigner, b *DoubleSigner) bool {
		return a.Equals(b)
	})
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
