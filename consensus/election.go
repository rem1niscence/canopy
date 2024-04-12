package consensus

import (
	"encoding/binary"
	"fmt"
	lib "github.com/ginchuco/ginchu/types"
	"github.com/ginchuco/ginchu/types/crypto"
	"gonum.org/v1/gonum/stat/distuv"
	"math/big"
	"strings"
)

/*
	Use CDF + practical VRF which is Hash(BLS.Signature(Last 5 Proposer PubKey + Height + Round))
	Fallback to Round Robin if none found

	- protects against grinding attack
	- protects against proposer ddos
	- weights based on stake

*/

const (
	vrfFloatPrec                    = uint(8 * (crypto.HashSize + 1))
	delimiter                       = "/"
	maxCandidates                   = 10
	minCandidates                   = 3
	percentOfValidatorsAsCandidates = 10
)

var maxVrfOutFloat *big.Float

func init() {
	maxVrfOutFloat, _, _ = big.ParseFloat(strings.Repeat("F", crypto.HashSize*2), 16, vrfFloatPrec, big.ToNearestEven)
}

type SortitionParams struct {
	SortitionData
	PrivateKey crypto.PrivateKeyI
}

type SortitionVerifyParams struct {
	SortitionData
	Signature []byte
	PublicKey crypto.PublicKeyI
}

type SortitionData struct {
	LastProducersPublicKeys [][]byte
	Height                  uint64
	Round                   uint64
	TotalValidators         uint64
	VotingPower             string
	TotalPower              string
}

type RoundRobinParams struct {
	SortitionData
	ValidatorSet *lib.ValidatorSet
}

func Sortition(p *SortitionParams) (out []byte, vrf *lib.Signature, isCandidate bool) {
	vrf = VRF(p.LastProducersPublicKeys, p.Height, p.Round, p.PrivateKey)
	out, isCandidate = sortition(p.VotingPower, p.TotalPower, p.TotalValidators, vrf.Signature)
	return
}

func VerifyCandidate(p *SortitionVerifyParams) (out []byte, isCandidate bool) {
	if p == nil {
		return nil, false
	}
	msg := formatInput(p.LastProducersPublicKeys, p.Height, p.Round)
	if !p.PublicKey.VerifyBytes(msg, p.Signature) {
		return nil, false
	}
	return sortition(p.VotingPower, p.TotalPower, p.TotalValidators, p.Signature)
}

func sortition(votingPower, totalPower string, totalValidators uint64, signature []byte) (out []byte, isCandidate bool) {
	out = crypto.Hash(signature)
	isCandidate = CDF(lib.StringToUint64(votingPower), lib.StringToUint64(totalPower), expectedCandidates(totalValidators), out) >= 1
	return
}

type VRFCandidate struct {
	PublicKey crypto.PublicKeyI
	Out       []byte
}

func SelectLeaderFromCandidates(candidates []VRFCandidate, data SortitionData, v *lib.ValidatorSet) (leaderPublicKey crypto.PublicKeyI) {
	if candidates == nil {
		return WeightedRoundRobin(&RoundRobinParams{
			SortitionData: data,
			ValidatorSet:  v,
		})
	}
	largest := new(big.Int)
	for _, c := range candidates {
		candidate := new(big.Int).SetBytes(c.Out)
		if lib.BigGreater(candidate, largest) {
			leaderPublicKey = c.PublicKey
			largest = candidate
		}
	}
	return
}

func VRF(lastNLeaders [][]byte, height, round uint64, privateKey crypto.PrivateKeyI) *lib.Signature {
	vrfIn := formatInput(lastNLeaders, height, round)
	return &lib.Signature{
		PublicKey: privateKey.PublicKey().Bytes(),
		Signature: privateKey.Sign(vrfIn),
	}
}

func CDF(votingPower, totalVotingPower, expectedCandidates uint64, vrfOut []byte) uint64 {
	binomial := distuv.Binomial{
		N: float64(votingPower),
		P: float64(expectedCandidates) / float64(totalVotingPower),
	}
	vrfOutFloat := toFloatBetween0And1(vrfOut)
	for i := uint64(0); i < votingPower; i++ {
		if vrfOutFloat <= binomial.CDF(float64(i)) {
			return i
		}
	}
	return votingPower
}

func WeightedRoundRobin(p *RoundRobinParams) (publicKey crypto.PublicKeyI) {
	seed := crypto.Hash(formatInput(p.LastProducersPublicKeys, p.Height, p.Round))[:16]
	seedUint64 := binary.BigEndian.Uint64(seed)
	totalPower := lib.StringToUint64(p.TotalPower)
	powerIndex := seedUint64 % totalPower

	powerCount := uint64(0)
	for _, v := range p.ValidatorSet.ValidatorSet {
		powerCount += lib.StringToUint64(v.VotingPower)
		if powerCount >= powerIndex {
			publicKey, _ = crypto.NewBLSPublicKeyFromBytes(v.PublicKey)
			return
		}
	}
	return nil
}

func expectedCandidates(totalValidators uint64) uint64 {
	candidates := uint64(float64(totalValidators) * (percentOfValidatorsAsCandidates / 100))
	if candidates < minCandidates {
		return minCandidates
	}
	if candidates > maxCandidates {
		return maxCandidates
	}
	return candidates
}

func toFloatBetween0And1(vrfOut []byte) float64 {
	f := new(big.Float).SetPrec(vrfFloatPrec)
	f.SetInt(new(big.Int).SetBytes(vrfOut[:]))
	prob, _ := new(big.Float).Quo(f, maxVrfOutFloat).Float64()
	return prob
}

func formatInput(lastNLeaderPublicKeys [][]byte, height, round uint64) []byte {
	var input string
	for _, key := range lastNLeaderPublicKeys {
		input += lib.BytesToString(key) + delimiter
	}
	return []byte(input + fmt.Sprintf("%d/%d", height, round))
}
