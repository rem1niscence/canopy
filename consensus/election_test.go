package consensus

import (
	"github.com/ginchuco/ginchu/lib/crypto"
	"github.com/stretchr/testify/require"
	"math"
	"math/rand"
	"testing"
)

func TestSortition(t *testing.T) {
	privateKey, _ := crypto.NewBLSPrivateKey()
	lastNProposers := [][]byte{[]byte("a"), []byte("b"), []byte("c")}
	power, totalPower := 1000000, 2000000
	expectedAvg := float64(power) / float64(totalPower)
	totalIterations := 1000
	errorThreshold := .07
	avg := uint64(0)
	for i := 0; i < totalIterations; i++ {
		avg += vrfAndCDF(SortitionParams{
			SortitionData: &SortitionData{
				LastProducersPublicKeys: lastNProposers,
				Height:                  uint64(rand.Intn(math.MaxUint32)),
				VotingPower:             uint64(power),
				TotalPower:              uint64(totalPower),
			},
			PrivateKey: privateKey,
		})
	}
	e := math.Abs(float64(avg)/float64(totalIterations) - expectedAvg)
	require.True(t, e < errorThreshold)
}

func vrfAndCDF(p SortitionParams) uint64 {
	vrf := VRF(p.LastProducersPublicKeys, p.Height, p.Round, p.PrivateKey)
	return CDF(p.VotingPower, p.TotalPower, 1, crypto.Hash(vrf.Signature))
}
