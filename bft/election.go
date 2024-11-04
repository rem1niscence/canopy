package bft

import (
	"encoding/binary"
	"fmt"
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
	"math/big"
)

/*
	ELECTION SORTITION:

		1) Practical VRF: Hash(BLS.Signature(Last Proposers Addresses + Height + Round)): a cryptographic
		function that produces a random output that can be publicly verified. In BFT, each participant in
		the network uses their private key to create a digital signature against Sortition seed data which
		may be publicly verified using their Public Key.

		2) Linear stake weighted threshold: a mathematical function that creates a target value from the stake;
		if the VRF output is below this threshold, the Validator is selected as a potential leader (Candidate)

		3) Multi Candidate resolution: Replicas choose the lowest VRF out from all valid Candidates as the Leader

		4) FilterOption_Exclude candidate resolution: Stake-Weighted-Pseudorandom selection using a simple modulo over total stake
		landing on a 'token index' over the list of Validators organized by their staked tokens

	Pros of Election Sortition:
	- protects against grinding attack (use of proposers addresses in the seed data)
	- protects against proposer ddos (don't know who the leader is until the process begins)
	- weights based on stake (fairly weighted using threshold)
*/

const (
	vrfFloatPrec                    = uint(8 * (crypto.HashSize + 1)) // the precision of a float is set to the number of bits just larger than the hash size
	maxCandidates                   = 10                              // maximum number of candidates despite the committee size
	minCandidates                   = 1                               // minimum number of candidates despite the committee size
	percentOfValidatorsAsCandidates = 10                              // the target percent of validators who should be candidates based on committee size
)

var maxHashAsFloat *big.Float

func init() {
	// big.Float version of MaxHash
	maxHashAsFloat = new(big.Float).SetInt(new(big.Int).SetBytes(crypto.MaxHash)).SetPrec(vrfFloatPrec)
}

// SortitionParams are the input params to run the Sortition function
type SortitionParams struct {
	*SortitionData                    // the seed data used for sortition
	PrivateKey     crypto.PrivateKeyI // the private key of the Validator
}

// SortitionVerifyParams are the input params to verify the Sortition function
type SortitionVerifyParams struct {
	*SortitionData                   // seed data the peer used for sortition
	Signature      []byte            // the VRF out of the peer
	PublicKey      crypto.PublicKeyI // the public key of the peer
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
	*SortitionData                          // seed data the peer used for sortition
	ValidatorSet   *lib.ConsensusValidators // the set of validators
}

// Sortition() runs the VRF and uses the Hash(output) to determine if IsCandidate
func Sortition(p *SortitionParams) (out []byte, vrf *lib.Signature, isCandidate bool) {
	vrf = VRF(p.LastProposerAddresses, p.Height, p.Round, p.PrivateKey)
	out, isCandidate = sortition(p.VotingPower, p.TotalPower, p.TotalValidators, vrf.Signature)
	return
}

// VerifyCandidate verifies that a remote peer is in fact a Leader Candidate by running the IsCandidate function using the provided VRF out
func VerifyCandidate(p *SortitionVerifyParams) (out []byte, isCandidate bool) {
	if p == nil {
		return nil, false
	}
	// build the seed data
	msg := formatInput(p.LastProposerAddresses, p.Height, p.Round)
	// validate the VRF out
	if !p.PublicKey.VerifyBytes(msg, p.Signature) {
		return nil, false
	}
	// validate the Candidacy by running the IsCandidate function using the Candidate values
	return sortition(p.VotingPower, p.TotalPower, p.TotalValidators, p.Signature)
}

// sortition() determines if IsCandidate using the hash of the VRF and calculates the expected candidates
func sortition(votingPower, totalPower, totalValidators uint64, signature []byte) (out []byte, isCandidate bool) {
	out = crypto.Hash(signature)
	isCandidate = IsCandidate(votingPower, totalPower, expectedCandidates(totalValidators), out)
	return
}

// VRFCandidate is a comparable structure that enables the selection of the Leader between candidates
type VRFCandidate struct {
	PublicKey crypto.PublicKeyI // the public key of the Candidate
	Out       []byte            // the hash of the VRF signature
}

// SelectProposerFromCandidates() chooses the `Leader` by comparing the pre-validated VRF Candidates, no candidates falls back to StakeWeightedRandom selection
func SelectProposerFromCandidates(candidates []VRFCandidate, data *SortitionData, v *lib.ConsensusValidators) (proposerPubKey []byte) {
	// if there are no candidates, fallback to StakeWeightedRandom
	if len(candidates) == 0 {
		return weightedPseudorandom(&PseudorandomParams{
			SortitionData: data,
			ValidatorSet:  v,
		}).Bytes()
	}
	// find the smallest VRF out among all candidates
	var smallest *big.Int
	for _, c := range candidates {
		candidate := new(big.Int).SetBytes(c.Out)
		if smallest == nil || lib.BigLess(candidate, smallest) {
			proposerPubKey = c.PublicKey.Bytes()
			smallest = candidate
		}
	}
	return
}

// VRF() 'Practical Verifiable Random Function': a function that given a secret key and a message, generates a unique random-looking number
// along with a certificate that anyone can check to confirm that this number was produced correctly from that specific message, without revealing the private key or
// how the number was made.
// NOTE: Academically speaking, this is not a true VRF because BLS signatures are not perfectly uniformly distributed in the strictest mathematical sense.
// The slight deviation from perfect uniformity does not significantly affect the security of BLS signatures are still considered secure and suitable
// for applications like digital signatures, VRFs, and blockchain consensus mechanisms.
func VRF(lastNProposers [][]byte, height, round uint64, privateKey crypto.PrivateKeyI) *lib.Signature {
	// generate the seed data that all Validators use during this View
	vrfIn := formatInput(lastNProposers, height, round)
	// sign it with the Private Key
	return &lib.Signature{
		PublicKey: privateKey.PublicKey().Bytes(),
		Signature: privateKey.Sign(vrfIn), // BLS signatures provide non-malleability and uniqueness making them a good candidate for a Practical VRF
	}
}

// IsCandidate: determines if the Validator is a Candidate from their voting power and VRF output
// - Creates a candidacy cutoff point for a Validator based on their stake (more stake = higher chance of being a Candidate)
// - Checks if number(vrfOut) is below the cutoff
func IsCandidate(votingPower, totalVotingPower, expectedCandidates uint64, vrfOut []byte) bool {
	vPower, totalVPower, expCand := lib.Uint64ToBigFloat(votingPower*expectedCandidates), lib.Uint64ToBigFloat(totalVotingPower), lib.Uint64ToBigFloat(expectedCandidates)
	// candidateCutoff = voting power * expected candidates / totalVotingPower
	candidateCutoff, _ := new(big.Float).Quo(vPower.Mul(vPower, expCand), totalVPower).Float64() // may be > 1 but that works fine
	// if VRF is under the candidateCutoff
	return toFloatBetween0And1(vrfOut) < candidateCutoff
}

// weightedPseudorandom() runs the 'no candidates' backup algorithm
// - generates an index for the 'token' that is our Leader from the seed data
func weightedPseudorandom(p *PseudorandomParams) (publicKey crypto.PublicKeyI) {
	// convert the seed data to a 16 byte hash, so it may fit in a uint64 type
	seed := formatInput(p.LastProposerAddresses, p.Height, p.Round)[:16]
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
			publicKey, _ = crypto.NewBLSPublicKeyFromBytes(v.PublicKey)
			return
		}
	}
	// failsafe: should not happen - use the last validator from the set as the winner
	publicKey, _ = crypto.NewBLSPublicKeyFromBytes(p.ValidatorSet.ValidatorSet[len(p.ValidatorSet.ValidatorSet)-1].PublicKey)
	return
}

// expectedCandidates() returns the number of expected candidates based on the committee size within the defined limits
func expectedCandidates(totalValidators uint64) uint64 {
	candidates := lib.Uint64Percentage(totalValidators, percentOfValidatorsAsCandidates)
	if candidates < minCandidates {
		return minCandidates
	}
	if candidates > maxCandidates {
		return maxCandidates
	}
	return candidates
}

// toFloatBetween0And1 converts a hash into a floating point number between 0 and 1
func toFloatBetween0And1(vrfOut []byte) float64 {
	f := new(big.Float).SetPrec(vrfFloatPrec)
	f.SetInt(new(big.Int).SetBytes(vrfOut[:]))
	prob, _ := new(big.Float).Quo(f, maxHashAsFloat).Float64()
	return prob
}

// formatInput() returns the 'seed data' for the VRF function
// `seed = lastNProposerPublicKeys + height + round`
func formatInput(lastNProposerPublicKeys [][]byte, height, round uint64) []byte {
	var input string
	for _, key := range lastNProposerPublicKeys {
		input += lib.BytesToString(key) + "/"
	}
	return crypto.Hash([]byte(input + fmt.Sprintf("%d/%d", height, round)))
}
