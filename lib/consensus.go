package lib

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/canopy-network/canopy/lib/crypto"
	"github.com/drand/kyber"
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
	// handle empty set
	if validators == nil {
		return ValidatorSet{}, ErrNoValidators()
	}
	// iterate through the ValidatorSet to get the count, total power, and convert
	// the public keys to 'points' on an elliptic curve for the BLS multikey
	for _, v := range validators.ValidatorSet {
		point, e := crypto.BytesToBLS12381Point(v.PublicKey)
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

// RootChainInfo maintains root-Chain data needed for consensus
type RootChainInfo struct {
	Height                 uint64         `json:"height"`
	ValidatorSet           ValidatorSet   `json:"validatorSet"`
	LastValidatorSet       ValidatorSet   `json:"lastValidatorSet"`
	LastProposers          *Proposers     `json:"lastProposers"`
	MinimumEvidenceHeight  uint64         `json:"minimumEvidenceHeight"`
	LastChainHeightUpdated uint64         `json:"lastRootHeightUpdated"`
	LotteryWinner          *LotteryWinner `json:"lotteryWinner"`
	Orders                 *OrderBook     `json:"orders"`
	RemoteCallbacks        *RemoteCallbacks
	Log                    LoggerI
}

// RemoteCallbacks are fallback rpc callbacks to the root-Chain
type RemoteCallbacks struct {
	Checkpoint            func(height, id uint64) (blockHash HexBytes, i ErrorI)
	ValidatorSet          func(height, id uint64) (ValidatorSet, ErrorI)
	IsValidDoubleSigner   func(height uint64, address string) (p *bool, err ErrorI)
	Transaction           func(tx TransactionI) (hash *string, err ErrorI)
	LastProposers         func(height uint64) (p *Proposers, err ErrorI)
	MinimumEvidenceHeight func(height uint64) (p *uint64, err ErrorI)
	CommitteeData         func(height, id uint64) (p *CommitteeData, err ErrorI)
	Lottery               func(height, id uint64) (p *LotteryWinner, err ErrorI)
	Orders                func(height, chainId uint64) (p *OrderBooks, err ErrorI)
}

// GetHeight() returns the height from the root-Chain
func (b *RootChainInfo) GetHeight() uint64 { return b.Height }

// GetValidatorSet() returns the validator set from the root-Chain
func (b *RootChainInfo) GetValidatorSet(id, height uint64) (ValidatorSet, ErrorI) {
	if height == b.Height {
		return NewValidatorSet(b.ValidatorSet.ValidatorSet)
	}
	if height == b.Height-1 {
		return NewValidatorSet(b.LastValidatorSet.ValidatorSet)
	}
	b.Log.Warnf("Executing remote GetValidatorSet call with requested height %d", height)
	return b.RemoteCallbacks.ValidatorSet(height, id)
}

// GetLastProposers() returns the last proposers from the root-Chain
func (b *RootChainInfo) GetLastProposers(height uint64) (*Proposers, ErrorI) {
	if height == b.Height {
		return b.LastProposers, nil
	}
	b.Log.Warnf("Executing remote GetLastProposers call with requested height %d", height)
	return b.RemoteCallbacks.LastProposers(height)
}

// GetOrders() returns the order book from the root-Chain
func (b *RootChainInfo) GetOrders(height, id uint64) (*OrderBook, ErrorI) {
	if height == b.Height {
		return b.Orders, nil
	}
	b.Log.Warnf("Executing remote GetOrders call with requested height %d", height)
	books, err := b.RemoteCallbacks.Orders(height, id)
	if err != nil {
		return nil, err
	}
	return books.OrderBooks[0], nil
}

// GetMinimumEvidenceHeight() returns the minimum evidence height from the root-Chain
func (b *RootChainInfo) GetMinimumEvidenceHeight(height uint64) (i uint64, err ErrorI) {
	if height == b.Height {
		return b.MinimumEvidenceHeight, nil
	}
	b.Log.Warnf("Executing remote GetMinimumEvidenceHeight call with requested height %d", height)
	res, err := b.RemoteCallbacks.MinimumEvidenceHeight(height)
	if err != nil {
		return
	}
	return *res, nil
}

// IsValidDoubleSigner() returns if an address is a valid double signer
func (b *RootChainInfo) IsValidDoubleSigner(height uint64, address string) (*bool, ErrorI) {
	b.Log.Warnf("Executing remote IsValidDoubleSigner call with requested height %d and address %s", height, address)
	return b.RemoteCallbacks.IsValidDoubleSigner(height, address)
}

// GetCheckpoint() returns the checkpoint if any for a specific chain height
func (b *RootChainInfo) GetCheckpoint(height, chainId uint64) (blockHash HexBytes, err ErrorI) {
	// TODO should be able to get these from the file or the root-Chain upon independence
	return b.RemoteCallbacks.Checkpoint(height, chainId)
}

// GetLastChainHeightUpdated() returns the last chain (target) height the committee (meta) data was updated from the root-Chain
func (b *RootChainInfo) GetLastChainHeightUpdated(height, id uint64) (uint64, ErrorI) {
	if height == b.Height {
		return b.LastChainHeightUpdated, nil
	}
	committeeData, err := b.RemoteCallbacks.CommitteeData(height, id)
	if err != nil {
		return 0, err
	}
	b.Log.Warnf("Executing remote GetLastChainHeightUpdated call with requested height %d", height)
	return committeeData.LastChainHeightUpdated, nil
}

// GetLotteryWinner() returns the winner of the delegate lottery from the root-Chain
func (b *RootChainInfo) GetLotteryWinner(height, id uint64) (*LotteryWinner, ErrorI) {
	if height == b.Height {
		return b.LotteryWinner, nil
	}
	b.Log.Warnf("Executing remote Lottery call with requested height %d", height)
	return b.RemoteCallbacks.Lottery(height, id)
}

// rootChainInfoJSON is the encoding structure used for json for RootChainInfo
type rootChainInfoJSON struct {
	Height                uint64               `json:"height"`
	Committee             *ConsensusValidators `json:"committee"`
	LastCommittee         *ConsensusValidators `json:"last_committee"`
	LastProposers         *Proposers           `json:"last_proposers"`
	LastRootHeightUpdated uint64               `json:"last_root_height_updated"`
	MinimumEvidenceHeight uint64               `json:"minimum_evidence_height"`
	LotteryWinner         *LotteryWinner       `json:"lottery_winner"`
	Orders                *OrderBook           `json:"orders"`
}

// MarshalJSON() implements the json.Marshaller for RootChainInfo
func (b *RootChainInfo) MarshalJSON() ([]byte, error) {
	return json.Marshal(rootChainInfoJSON{
		Height:                b.Height,
		Committee:             b.ValidatorSet.ValidatorSet,
		LastCommittee:         b.LastValidatorSet.ValidatorSet,
		LastProposers:         b.LastProposers,
		LastRootHeightUpdated: b.LastChainHeightUpdated,
		MinimumEvidenceHeight: b.MinimumEvidenceHeight,
		LotteryWinner:         b.LotteryWinner,
		Orders:                b.Orders,
	})
}

// UnmarshalJSON() implements the json.Unmarshaler for RootChainInfo
func (b *RootChainInfo) UnmarshalJSON(bz []byte) (err error) {
	j := new(rootChainInfoJSON)
	if err = json.Unmarshal(bz, j); err != nil {
		return
	}
	validatorSet, err := NewValidatorSet(j.Committee)
	if err != nil {
		return
	}
	lastValidatorSet, _ := NewValidatorSet(j.LastCommittee)
	*b = RootChainInfo{
		Height:                 j.Height,
		ValidatorSet:           validatorSet,
		LastValidatorSet:       lastValidatorSet,
		LastProposers:          j.LastProposers,
		MinimumEvidenceHeight:  j.MinimumEvidenceHeight,
		LastChainHeightUpdated: j.LastRootHeightUpdated,
		LotteryWinner:          j.LotteryWinner,
		Orders:                 j.Orders,
	}
	return
}

// LotteryWinner is a structure that holds the subject of a pseudorandom selection and their % cut of the reward
// This is used for delegation + sub-delegation + sub-validator earnings
type LotteryWinner struct {
	Winner HexBytes `json:"winner"`
	Cut    uint64   `json:"cut"`
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
func (x *AggregateSignature) GetNonSigners(valSet *ConsensusValidators) (nonSignerPubKeys [][]byte, nonSignerPercent int, err ErrorI) {
	vs, err := NewValidatorSet(valSet)
	if err != nil {
		return
	}
	nonSignerPubKeys, nonSignerPower, err := x.getSigners(vs, true)
	if err != nil {
		return
	}
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
	Committee uint64       `json:"committee"`
}

// On() returns whether there exists any filters
func (v ValidatorFilters) On() bool {
	return v.Unstaking != FilterOption_Off || v.Paused != FilterOption_Off || v.Delegate != FilterOption_Off || v.Committee != 0
}

// FilterOption symbolizes 'condition must be true (yes)' 'condition must be false (no)' or 'filter off (both)' for filters
type FilterOption int

// nolint:all
const (
	FilterOption_Off     FilterOption = 0 // true or false condition
	FilterOption_MustBe               = 1 // condition must be true
	FilterOption_Exclude              = 2 // condition must be false
)

// VIEW CODE BELOW

func (x *View) CheckBasic() ErrorI {
	if x == nil {
		return ErrEmptyView()
	}
	// round and phase are not further checked,
	// because peers may be sending valid messages
	// asynchronously from different views
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
	if view.ChainId != x.ChainId {
		return ErrWrongChainId()
	}
	if enforceHeights && x.Height != view.Height {
		return ErrWrongHeight()
	}
	if enforceHeights && x.RootHeight != view.RootHeight {
		return ErrWrongRootHeight()
	}
	return nil
}

// Copy() returns a reference to a clone of the View
func (x *View) Copy() *View {
	return &View{
		Height:     x.Height,
		Round:      x.Round,
		Phase:      x.Phase,
		RootHeight: x.RootHeight,
		NetworkId:  x.NetworkId,
		ChainId:    x.ChainId,
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
	if x.RootHeight != v.RootHeight {
		return false
	}
	if x.ChainId != v.ChainId {
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
	if x.RootHeight < v.RootHeight {
		return true
	}
	if x.RootHeight > v.RootHeight {
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
	return fmt.Sprintf("(rH:%d, H:%d, R:%d, P:%s)", x.RootHeight, x.Height, x.Round, x.Phase)
}

// jsonView represents the json.Marshaller and json.Unmarshaler implementation of View
type jsonView struct {
	Height     uint64 `json:"height"`
	RootHeight uint64 `json:"committeeHeight"`
	Round      uint64 `json:"round"`
	Phase      string `json:"phase"` // string version of phase
	NetworkID  uint64 `json:"networkID"`
	ChainId    uint64 `json:"chainId"`
}

// MarshalJSON() implements the json.Marshaller interface
func (x View) MarshalJSON() ([]byte, error) {
	return json.Marshal(jsonView{
		Height:     x.Height,
		RootHeight: x.RootHeight,
		Round:      x.Round,
		Phase:      Phase_name[int32(x.Phase)],
		NetworkID:  x.NetworkId,
		ChainId:    x.ChainId,
	})
}

// MarshalJSON() implements the json.Marshaller interface
func (x *View) UnmarshalJSON(b []byte) (err error) {
	j := new(jsonView)
	if err = json.Unmarshal(b, j); err != nil {
		return
	}
	*x = View{
		NetworkId:  j.NetworkID,
		ChainId:    j.ChainId,
		Height:     j.Height,
		RootHeight: j.RootHeight,
		Round:      j.Round,
		Phase:      Phase(Phase_value[j.Phase]),
	}
	return
}

// MarshalJSON() implements the json.Marshaller interface
func (x Phase) MarshalJSON() ([]byte, error) {
	return json.Marshal(Phase_name[int32(x)])
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
	if !bytes.Equal(x.Id, d.Id) {
		return false
	}
	return slices.Equal(x.Heights, d.Heights)
}

// proposersJSON implements the json.Marshaller and json.Unmarshaler interfaces for Proposer
type proposersJSON struct {
	Addresses []HexBytes `json:"addresses,omitempty"`
}

// MarshalJSON() implements the json.Marshaller interface for Proposers
func (x Proposers) MarshalJSON() ([]byte, error) {
	j := proposersJSON{}
	for _, address := range x.Addresses {
		j.Addresses = append(j.Addresses, address)
	}
	return json.Marshal(j)
}

// UnmarshalJSON() implements the json.Unmarshaler interface for Proposers
func (x *Proposers) UnmarshalJSON(bz []byte) (err error) {
	j := new(proposersJSON)
	if err = json.Unmarshal(bz, j); err != nil {
		return
	}
	for _, address := range j.Addresses {
		x.Addresses = append(x.Addresses, address)
	}
	return
}

// SortitionData is the seed data for the IsCandidate and VRF functions
type SortitionData struct {
	LastProposerAddresses [][]byte // the last N proposers addresses prevents any grinding attacks
	Height                uint64   // the height ensures unique proposer selection for each height
	Round                 uint64   // the round ensures unique proposer selection for each round
	TotalValidators       uint64   // the count of validators in the set
	TotalPower            uint64   // the total power of all validators in the set
	VotingPower           uint64   // the amount of voting power the node has
}

// PseudorandomParams are the input params to run the Stake-Weighted-Pseudorandom fallback leader selection algorithm
type PseudorandomParams struct {
	*SortitionData                      // seed data the peer used for sortition
	ValidatorSet   *ConsensusValidators // the set of validators
}

// WeightedPseudorandom() runs the 'no candidates' backup algorithm
// - generates an index for the 'token' that is our Leader from the seed data
func WeightedPseudorandom(p *PseudorandomParams) (publicKey crypto.PublicKeyI) {
	// convert the seed data to a 16 byte hash, so it may fit in a uint64 type
	seed := FormatSortitionInput(p.LastProposerAddresses, p.Height, p.Round)[:16]
	// convert the seedBytes into a uint64 number
	seedUint64 := binary.BigEndian.Uint64(seed)
	// ensure that number falls within our 'Total Power'
	powerIndex := seedUint64 % p.TotalPower

	powerCount := uint64(0)
	// with this deterministically ordered validator set, iterate until exceeding the power index
	// as that Validator has the exact randomly chosen 'token' that is the lottery winner
	for _, v := range p.ValidatorSet.ValidatorSet {
		// add the voting power to the count
		powerCount += v.VotingPower
		// if exceed the powerIndex, that Validator has the exact 'token'
		if powerCount > powerIndex {
			// set the winner and exit
			publicKey, _ = crypto.BytesToBLS12381Public(v.PublicKey)
			return
		}
	}
	// failsafe: should not happen - use the last validator from the set as the winner
	publicKey, _ = crypto.BytesToBLS12381Public(p.ValidatorSet.ValidatorSet[len(p.ValidatorSet.ValidatorSet)-1].PublicKey)
	return
}

// FormatSortitionInput() returns the 'seed data' for the VRF function
// `seed = lastNProposerPublicKeys + height + round`
func FormatSortitionInput(lastNProposerPublicKeys [][]byte, height, round uint64) []byte {
	var input string
	for _, key := range lastNProposerPublicKeys {
		input += BytesToString(key) + "/"
	}
	return crypto.Hash([]byte(input + fmt.Sprintf("%d/%d", height, round)))
}
