package consensus

import (
	"fmt"
	lib "github.com/ginchuco/ginchu/types"
	"github.com/ginchuco/ginchu/types/crypto"
	"github.com/stretchr/testify/require"
	"math"
	"math/rand"
	"testing"
)

func TestSortition(t *testing.T) {
	privateKey, _ := crypto.NewBLSPrivateKey()
	lastNLeaders := [][]byte{[]byte("a"), []byte("b"), []byte("c")}
	power, totalPower := 1, 2
	powerString, totalPowerString := fmt.Sprintf("%d", power), fmt.Sprintf("%d", totalPower)
	expectedAvg := float64(power) / float64(totalPower)
	totalIterations := 1000
	errorThreshold := .07
	avg := uint64(0)
	for i := 0; i < totalIterations; i++ {
		avg += vrfAndCDF(SortitionParams{
			SortitionData: SortitionData{
				LastProducersPublicKeys: lastNLeaders,
				Height:                  uint64(rand.Intn(math.MaxUint32)),
				VotingPower:             powerString,
				TotalPower:              totalPowerString,
			},
			PrivateKey: privateKey,
		})
	}
	e := math.Abs(float64(avg)/float64(totalIterations) - expectedAvg)
	require.True(t, e < errorThreshold)
}

func vrfAndCDF(p SortitionParams) uint64 {
	vrf := VRF(p.LastProducersPublicKeys, p.Height, p.Round, p.PrivateKey)
	return CDF(lib.StringToUint64(p.VotingPower), lib.StringToUint64(p.TotalPower), 1, crypto.Hash(vrf.Signature))
}
