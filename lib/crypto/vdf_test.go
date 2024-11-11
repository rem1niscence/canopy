package crypto

import (
	"github.com/stretchr/testify/require"
	"sync/atomic"
	"testing"
	"time"
)

func TestVDF(t *testing.T) {
	// define a seed
	seed := []byte("seed")
	// create a new vdf service
	vdfService := &VDFService{running: &atomic.Bool{}, Iterations: 10}
	// generates a VDF proof using the current params state of the VDF Service object
	vdfService.Run(seed)
	// finish the vdf
	out, iterations := vdfService.Finish()
	// verify the vdf
	require.True(t, vdfService.VerifyVDF(seed, out, iterations))
}

func TestVDFPrematureExit(t *testing.T) {
	testTimeout := 2 * time.Second
	// define a seed
	seed, iterations := []byte("seed"), 10
	// create a new vdf service
	vdfService := &VDFService{TargetTime: time.Second, stopChan: make(chan struct{}), running: &atomic.Bool{}, Iterations: iterations}
	// generates a VDF proof using the current params state of the VDF Service object
	go vdfService.Run(seed)
	// exit the vdf immediately
	out, gotIterations := vdfService.Finish()
	require.Nil(t, out)
	// ensure the last iterations equal expected
	require.Zero(t, gotIterations)
	// wait until iterations are adjusted or timeout
loop:
	for {
		select {
		case <-time.After(testTimeout):
			t.Fatal("test timeout")
		default:
			if vdfService.Iterations != 10 {
				break loop
			}
		}
	}
	// ensure iterations were adjusted smaller
	require.Less(t, vdfService.Iterations, iterations)
}
