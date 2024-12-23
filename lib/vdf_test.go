package lib

import (
	"github.com/canopy-network/canopy/lib/crypto"
	"github.com/stretchr/testify/require"
	"sync/atomic"
	"testing"
	"time"
)

func TestVDF(t *testing.T) {
	// define a seed
	seed := []byte("seed")
	// create a new vdf service
	vdfService := &VDFService{running: &atomic.Bool{}, Iterations: 1000}
	// generates a VDF proof using the current params state of the VDF Service object
	vdfService.Run(seed)
	// finish the vdf
	results := vdfService.Finish()
	// verify the vdf
	require.True(t, vdfService.VerifyVDF(seed, results))
}

func TestVDFPrematureExit(t *testing.T) {
	testTimeout := 2 * time.Second
	// define a seed
	seed, iterations := []byte("seed"), 1000
	// create a new vdf service
	vdfService := &VDFService{TargetTime: time.Second, stopChan: make(chan struct{}), running: &atomic.Bool{}, Iterations: iterations}
	// generates a VDF proof using the current params state of the VDF Service object
	go vdfService.Run(seed)
out:
	for {
		select {
		case <-time.After(testTimeout):
			t.Fatal("test timeout")
		default:
			if vdfService.running.Load() == true {
				break out
			}
		}
	}
	// exit the vdf immediately
	require.Nil(t, vdfService.Finish())
loop:
	for {
		select {
		case <-time.After(testTimeout):
			t.Fatal("test timeout")
		default:
			if vdfService.Iterations != iterations {
				break loop
			}
		}
	}
	// ensure empty results
	require.EqualExportedValues(t, crypto.VDF{}, vdfService.Results)
	// ensure iterations were adjusted smaller
	require.Less(t, vdfService.Iterations, iterations)
}
