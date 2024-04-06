package leader_election

import (
	"fmt"
	"github.com/ginchuco/ginchu/crypto"
	lib "github.com/ginchuco/ginchu/types"
	"github.com/stretchr/testify/require"
	"math"
	"math/big"
	"math/rand"
	"strings"
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
				LastNLeaderPubKeys: lastNLeaders,
				Height:             uint64(rand.Intn(math.MaxUint32)),
				VotingPower:        powerString,
				TotalPower:         totalPowerString,
			},
			PrivateKey: privateKey,
		})
	}
	e := math.Abs(float64(avg)/float64(totalIterations) - expectedAvg)
	require.True(t, e < errorThreshold)
}

func vrfAndCDF(p SortitionParams) uint64 {
	vrf := VRF(p.LastNLeaderPubKeys, p.Height, p.Round, p.PrivateKey)
	return CDF(lib.StringToUint64(p.VotingPower), lib.StringToUint64(p.TotalPower), 1, crypto.Hash(vrf.Signature))
}

func TestT(t *testing.T) {
	numValidators := uint64(119)
	maxHash := strings.Repeat("F", crypto.HashSize*2)[:16]
	blockHash, _, _ := big.ParseFloat(maxHash, 16, vrfFloatPrec, big.ToNearestEven)
	mod, _ := blockHash.Uint64()
	mod = 121
	fmt.Println(mod % numValidators)
}
