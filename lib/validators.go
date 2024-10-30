package lib

import (
	"bytes"
	"encoding/json"
	"github.com/drand/kyber"
	"github.com/ginchuco/canopy/lib/crypto"
)

// ValidatorSet represents a collection of validators responsible for consensus
// It facilitates the creation and validation of +2/3 Majority agreements using multi-signatures
type ValidatorSet struct {
	ValidatorSet  *ConsensusValidators   // a list of validators participating in the consensus process
	MultiKey      crypto.MultiPublicKeyI // a composite public key derived from the individual public keys of all validators, used for verifying multi-signatures
	TotalPower    uint64                 // the aggregate voting power of all validators in the set, reflecting their influence on the consensus
	MinimumMaj23  uint64                 // the minimum voting power threshold required to achieve a two-thirds majority (2f+1), essential for consensus decisions
	NumValidators uint64                 // the total number of validators in the set, indicating the size of the validator pool
}

// NewValidatorSet() initializes a ValidatorSet from a given set of consensus validators
func NewValidatorSet(validators *ConsensusValidators) (vs ValidatorSet, err ErrorI) {
	totalPower, count, points := uint64(0), uint64(0), make([]kyber.Point, 0)
	// iterate through the ValidatorSet to get the count, total power, and convert
	// the public keys to 'points' on an elliptic curve for the BLS multikey
	for _, v := range validators.ValidatorSet {
		point, e := crypto.NewBLSPointFromBytes(v.PublicKey)
		if e != nil {
			return ValidatorSet{}, ErrPubKeyFromBytes(e)
		}
		points = append(points, point)
		totalPower += v.VotingPower
		count++
	}
	if totalPower == 0 {
		return ValidatorSet{}, ErrNoValidators()
	}
	// calculate the minimum power for a two-thirds majority (2f+1)
	minPowerFor23Maj := (2*totalPower)/3 + 1
	if err != nil {
		return
	}
	// create a composite multi-public key out of the public keys (in curve point format)
	mpk, e := crypto.NewMultiBLSFromPoints(points, nil)
	if e != nil {
		return ValidatorSet{}, ErrNewMultiPubKey(e)
	}
	// return the validator set
	return ValidatorSet{
		ValidatorSet:  validators,
		MultiKey:      mpk,
		TotalPower:    totalPower,
		MinimumMaj23:  minPowerFor23Maj,
		NumValidators: count,
	}, nil
}

// GetValidator() retrieves a validator from the ValidatorSet using the public key
func (vs *ValidatorSet) GetValidator(publicKey []byte) (val *ConsensusValidator, err ErrorI) {
	val, _, err = vs.GetValidatorAndIdx(publicKey)
	return
}

// GetValidatorAndIdx() retrieves a validator and its index in the ValidatorSet using the public key
func (vs *ValidatorSet) GetValidatorAndIdx(publicKey []byte) (val *ConsensusValidator, idx int, err ErrorI) {
	if vs == nil || vs.ValidatorSet == nil {
		return nil, 0, ErrInvalidValidatorIndex()
	}
	for i, v := range vs.ValidatorSet.ValidatorSet {
		if bytes.Equal(v.PublicKey, publicKey) {
			return v, i, nil
		}
	}
	return nil, 0, ErrValidatorNotInSet(publicKey)
}

// Equals() checks if two AggregateSignature instances are identical
func (x *AggregateSignature) Equals(a *AggregateSignature) bool {
	if x == nil || a == nil {
		return false
	}
	if !bytes.Equal(x.Signature, a.Signature) {
		return false
	}
	if !bytes.Equal(x.Bitmap, a.Bitmap) {
		return false
	}
	return true
}

// CheckBasic() validates the basic structure and length of the AggregateSignature
func (x *AggregateSignature) CheckBasic() ErrorI {
	if x == nil {
		return ErrEmptyAggregateSignature()
	}
	if len(x.Signature) != crypto.BLS12381SignatureSize {
		return ErrInvalidAggrSignatureLength()
	}
	if len(x.Bitmap) == 0 {
		return ErrEmptySignerBitmap()
	}
	return nil
}

// Check() validates a +2/3 majority of the signature using the payload bytes and the ValidatorSet
// NOTE: "partialQC" means the signature is valid but does not reach a +2/3 majority
func (x *AggregateSignature) Check(sb SignByte, vs ValidatorSet) (isPartialQC bool, err ErrorI) {
	if err = x.CheckBasic(); err != nil {
		return false, err
	}
	key := vs.MultiKey.Copy()
	// indicate which validator indexes have purportedly signed the payload
	// and are included in the aggregated signature
	if er := key.SetBitmap(x.Bitmap); er != nil {
		return false, ErrInvalidSignerBitmap(er)
	}
	// use the composite public key to verify the aggregate signature
	if !key.VerifyBytes(sb.SignBytes(), x.Signature) {
		return false, ErrInvalidAggrSignature()
	}
	// get the total power and the min +2/3 majority from the bitmap and ValSet
	_, totalSignedPower, err := x.GetSigners(vs)
	if err != nil {
		return false, err
	}
	// ensure the signers reach a +2/3 majority
	if totalSignedPower < vs.MinimumMaj23 {
		return true, nil
	}
	return false, nil
}

// GetSigners() returns the public keys and corresponding combined voting power of those who signed
func (x *AggregateSignature) GetSigners(vs ValidatorSet) (signers [][]byte, signedPower uint64, err ErrorI) {
	signers, signedPower, err = x.getSigners(vs, false)
	return
}

// GetNonSigners() returns the public keys and corresponding percentage of voting power who are not included in the AggregateSignature
func (x *AggregateSignature) GetNonSigners(valSet *ConsensusValidators) (nonSigners [][]byte, nonSignerPercent int, err ErrorI) {
	vs, err := NewValidatorSet(valSet)
	if err != nil {
		return nil, 0, err
	}
	nonSigners, nonSignerPower, err := x.getSigners(vs, true)
	nonSignerPercent = int(Uint64PercentageDiv(nonSignerPower, vs.TotalPower))
	return
}

// getSigners() returns the public keys and corresponding combined voting power of signers or nonsigners
func (x *AggregateSignature) getSigners(vs ValidatorSet, nonSigners bool) (pubkeys [][]byte, power uint64, err ErrorI) {
	key := vs.MultiKey.Copy()
	// set the 'who signed' bitmap in a copy of the key
	if e := key.SetBitmap(x.Bitmap); e != nil {
		err = ErrInvalidSignerBitmap(e)
		return
	}
	// iterate through the ValSet to and see if the validator signed
	power = uint64(0)
	for i, val := range vs.ValidatorSet.ValidatorSet {
		// did they sign?
		signed, er := key.SignerEnabledAt(i)
		if er != nil {
			err = ErrInvalidSignerBitmap(er)
			return
		}
		// if so, add to the pubkeys and add to the power
		if signed && !nonSigners || !signed && nonSigners {
			pubkeys = append(pubkeys, val.PublicKey)
			power += val.VotingPower
		}
	}
	return
}

// GetDoubleSigners() compares the signers of two signatures and return who signed both
func (x *AggregateSignature) GetDoubleSigners(y *AggregateSignature, vs ValidatorSet) (doubleSigners [][]byte, err ErrorI) {
	key, key2 := vs.MultiKey.Copy(), vs.MultiKey.Copy()
	// set the 'who signed' bitmap in a copy of both keys
	if er := key.SetBitmap(x.Bitmap); er != nil {
		return nil, ErrInvalidSignerBitmap(er)
	}
	if er := key2.SetBitmap(y.Bitmap); er != nil {
		return nil, ErrInvalidSignerBitmap(er)
	}
	// iterate through the ValSet to and see if the validator signed
	for i, val := range vs.ValidatorSet.ValidatorSet {
		signed, e := key.SignerEnabledAt(i)
		if e != nil {
			return nil, ErrInvalidSignerBitmap(e)
		}
		// if signed 1, check if they signed 2 as well
		if signed {
			signed, e = key2.SignerEnabledAt(i)
			if e != nil {
				return nil, ErrInvalidSignerBitmap(e)
			}
			// if signed both, save as a double signer
			if signed {
				doubleSigners = append(doubleSigners, val.PublicKey)
			}
		}
	}
	return
}

// jsonAggregateSig represents the json.Marshaller and json.Unmarshaler implementation of AggregateSignature
type jsonAggregateSig struct {
	Signature HexBytes `json:"signature,omitempty"`
	Bitmap    HexBytes `json:"bitmap,omitempty"`
}

// MarshalJSON() implements the json.Marshaller interface
func (x AggregateSignature) MarshalJSON() ([]byte, error) {
	return json.Marshal(jsonAggregateSig{
		Signature: x.Signature,
		Bitmap:    x.Bitmap,
	})
}

// UnmarshalJSON() implements the json.Unmarshaler interface
func (x *AggregateSignature) UnmarshalJSON(b []byte) error {
	var j jsonAggregateSig
	if err := json.Unmarshal(b, &j); err != nil {
		return err
	}
	x.Signature, x.Bitmap = j.Signature, j.Bitmap
	return nil
}

// CONSENSUS VALIDATOR LOGIC BELOW

// Root() calculates the Merkle root of the ConsensusValidators
func (x *ConsensusValidators) Root() ([]byte, ErrorI) {
	if x == nil || len(x.ValidatorSet) == 0 {
		return nil, nil
	}
	var b [][]byte
	for _, val := range x.ValidatorSet {
		bz, err := Marshal(val)
		if err != nil {
			return nil, err
		}
		b = append(b, bz)
	}
	root, _, err := MerkleTree(b)
	return root, err
}

// marshalling utility structure for the ConsensusValidator
// allows easy hex byte marshalling of the public key
type jsonConsValidator struct {
	PublicKey   HexBytes `json:"public_key,omitempty"`
	VotingPower uint64   `json:"voting_power,omitempty"`
	NetAddress  string   `json:"net_address,omitempty"`
}

// MarshalJSON() overrides and implements the json.Marshaller interface
func (x *ConsensusValidator) MarshalJSON() ([]byte, error) {
	return json.Marshal(&jsonConsValidator{
		PublicKey:   x.PublicKey,
		VotingPower: x.VotingPower,
		NetAddress:  x.NetAddress,
	})
}

// UnmarshalJSON() overrides and implements the json.Unmarshaller interface
func (x *ConsensusValidator) UnmarshalJSON(b []byte) (err error) {
	j := new(jsonConsValidator)
	if err = json.Unmarshal(b, j); err != nil {
		return err
	}
	*x = ConsensusValidator{
		PublicKey:   j.PublicKey,
		VotingPower: j.VotingPower,
		NetAddress:  j.NetAddress,
	}
	return
}

// ValidatorFilters are used to filter types of validators from a ValidatorPage
type ValidatorFilters struct {
	Unstaking FilterOption `json:"unstaking"`
	Paused    FilterOption `json:"paused"`
	Delegate  FilterOption `json:"delegate"`
}

// On() returns whether there exists any filters
func (v ValidatorFilters) On() bool {
	return v.Unstaking != Off || v.Paused != Off || v.Delegate != Off
}

// FilterOption symbolizes 'condition must be true (yes)' 'condition must be false (no)' or 'filter off (both)' for filters
type FilterOption int

// nolint:all
const (
	Off FilterOption = 0 // true or false condition
	Yes              = 1 // condition must be true
	No               = 2 // condition must be false
)
