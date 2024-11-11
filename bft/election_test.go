package bft

import (
	"encoding/binary"
	"fmt"
	"github.com/canopy-network/canopy/lib/crypto"
	"github.com/stretchr/testify/require"
	"math"
	"math/rand"
	"testing"
)

func TestSortitionAndVerifyCandidate(t *testing.T) {
	tests := []struct {
		name        string
		detail      string
		totalVals   uint64
		isCandidate bool
	}{
		{
			name:        "isCandidate",
			detail:      "deterministic key set ensures sortition results in a candidate in a set of 3 validators",
			totalVals:   3,
			isCandidate: true,
		},
		{
			name:        "isNotCandidate",
			detail:      "deterministic key set ensures sortition results in not a candidate in a set of 6 validators",
			totalVals:   4,
			isCandidate: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := newTestConsensus(t, Election, int(test.totalVals))
			privateKey := c.valKeys[0]
			sortitionData := newTestSortitionData(t, c)
			out, vrf, isCandidate := Sortition(&SortitionParams{
				SortitionData: sortitionData,
				PrivateKey:    privateKey,
			})
			require.Equal(t, VRF(sortitionData.LastProposerAddresses, sortitionData.Height, sortitionData.Round, privateKey), vrf)
			require.Equal(t, crypto.Hash(vrf.Signature), out)
			require.Equal(t, test.isCandidate, isCandidate)
			outVerify, isCandidateFromVerify := VerifyCandidate(&SortitionVerifyParams{
				SortitionData: sortitionData,
				Signature:     vrf.Signature,
				PublicKey:     c.valKeys[0].PublicKey(),
			})
			require.Equal(t, out, outVerify)
			require.Equal(t, test.isCandidate, isCandidateFromVerify)
		})
	}
}

func TestWhenIsCandidate(t *testing.T) {
	for i := 1; i < 8; i++ {
		c := newTestConsensus(t, Election, i)
		fmt.Println("NUM VALIDATORS ", i)
		for j := 0; j < len(c.valKeys); j++ {
			privateKey := c.valKeys[j]
			sortitionData := newTestSortitionData(t, c)
			_, _, isCandidate := Sortition(&SortitionParams{
				SortitionData: sortitionData,
				PrivateKey:    privateKey,
			})
			if isCandidate {
				fmt.Printf("%d,", j)
			}
		}
		fmt.Println()
	}
}

func TestSortitionValidity(t *testing.T) {
	privateKey, _ := crypto.NewBLS12381PrivateKey()
	lastNProposers := [][]byte{[]byte("a"), []byte("b"), []byte("c")}
	power, totalPower := 1000000, 3000000
	expectedAvg := float64(power) / float64(totalPower)
	totalIterations := 1000
	errorThreshold := .07
	isCandCount := uint64(0)
	for i := 0; i < totalIterations; i++ {
		if isCand := vrfAndCDF(SortitionParams{
			SortitionData: &SortitionData{
				LastProposerAddresses: lastNProposers,
				Height:                uint64(rand.Intn(math.MaxUint32)),
				VotingPower:           uint64(power),
				TotalPower:            uint64(totalPower),
			},
			PrivateKey: privateKey,
		}); isCand {
			isCandCount++
		}
	}
	e := math.Abs(float64(isCandCount)/float64(totalIterations) - expectedAvg)
	require.True(t, e < errorThreshold)
}

func TestSelectProposerFromCandidates(t *testing.T) {
	tests := []struct {
		name                string
		detail              string
		totalVals           uint64
		totalCandidates     uint64
		expectedProposerIdx int
	}{
		{
			name:                "no candidates, weighted pseudorandom",
			detail:              "deterministic key set ensures that the weighted leader id is deterministic",
			totalVals:           3,
			totalCandidates:     0,
			expectedProposerIdx: 0,
		},
		{
			name:                "3 candidates, lowest index (0) is the proposer",
			detail:              "since out is set to index and the lowest out is the proposer, candidate 0 should be the proposer",
			totalVals:           3,
			totalCandidates:     3,
			expectedProposerIdx: 0,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := newTestConsensus(t, Election, int(test.totalVals))
			var vrfCandidates []VRFCandidate
			for i := uint64(0); i < test.totalCandidates; i++ {
				out := make([]byte, 8)
				binary.BigEndian.PutUint64(out, i)
				vrfCandidates = append(vrfCandidates, VRFCandidate{
					PublicKey: c.valKeys[i].PublicKey(),
					Out:       out,
				})
			}
			expectedProposerPubKey := c.valKeys[test.expectedProposerIdx].PublicKey().Bytes()
			require.Equal(t, expectedProposerPubKey, SelectProposerFromCandidates(vrfCandidates, newTestSortitionData(t, c), c.valSet.ValidatorSet))
		})
	}
}

func newTestSortitionData(t *testing.T, c *testConsensus) *SortitionData {
	var lastNProposers [][]byte
	for _, k := range c.valKeys {
		lastNProposers = append(lastNProposers, k.PublicKey().Address().Bytes())
	}
	val, err := c.valSet.GetValidator(c.valKeys[0].PublicKey().Bytes())
	require.NoError(t, err)
	sortitionData := &SortitionData{
		LastProposerAddresses: lastNProposers,
		Height:                1,
		Round:                 0,
		TotalValidators:       uint64(len(c.valKeys)),
		VotingPower:           val.VotingPower,
		TotalPower:            c.valSet.TotalPower,
	}
	return sortitionData
}

func vrfAndCDF(p SortitionParams) bool {
	vrf := VRF(p.LastProposerAddresses, p.Height, p.Round, p.PrivateKey)
	return IsCandidate(p.VotingPower, p.TotalPower, 1, crypto.Hash(vrf.Signature))
}
