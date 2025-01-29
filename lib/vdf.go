package lib

import (
	"github.com/canopy-network/canopy/lib/crypto"
	"sync/atomic"
	"time"
)

const (
	// StartingIterationsPerSecond is a 'best-guess' estimate based on 2.3 GHz 8-Core Intel Core i9
	StartingIterationsPerSecond = 550
	// IterationsFixedDecreasePercent is a 'best-guess' iteration reduction when a Stop() is called before finishing
	IterationsFixedDecreasePercent = float64(10)
)

// VDFService is a structure that wraps Verifiable Delay Functionality
// Verifiable Delay Function (VDF) is a cryptographic algorithm that requires a specific,
// non-parallelizable amount of time to compute, while its result can be quickly and easily verified
// Here's how it works:
//   - VDFService.Run() runs the VDF for a pre-defined number of iterations
//   - There's two paths: Success and Interrupt, either path results in an adjustment in the number of
//     iterations based on ProcessingTime (how long it took) vs TargetTime (the desired completion time)
//   - - The success path is a non-interrupted VDF run. This run results in a populated VDFResults object
//   - - The interrupt path is a premature exit VDF run that has an empty VDFResults object
//
// The VDF is designed to handle a single call to Run() always followed by a single call to Finish()
type VDFService struct {
	TargetTime time.Duration // the desired completion time of a VDF run, overages are expected, so add 'breathing room'
	Iterations int           // number of iterations the VDF will currently Run()
	Results    crypto.VDF    // the results from the previous VDF run
	stopChan   chan struct{} // channel to signal an exit for the vdf
	running    *atomic.Bool  // if the vdf service is currently running
	log        LoggerI
}

// NewVDFService() creates a new instance of the VDF service
func NewVDFService(targetTime time.Duration, log LoggerI) (vdf *VDFService) {
	vdf = &VDFService{TargetTime: targetTime, stopChan: make(chan struct{}, 100), running: &atomic.Bool{}, log: log}
	vdf.estimateIterationsPerSecond()
	return
}

// Run() *blocking call*:  generates a VDF proof using the current params state of the VDF Service object
// The design is to save the results
func (vdf *VDFService) Run(seed []byte) {
	vdf.log.Debugf("Starting the VDF service with %d iterations", vdf.Iterations)
	if vdf == nil {
		return
	}
	// - Run() and not running locks and starts a run
	// - Run() and already running returns
	if !vdf.running.CompareAndSwap(false, true) {
		vdf.log.Debug("VDF service is already running")
		return
	}
	// clear the results object
	vdf.Results = crypto.VDF{}
	// at the end of this function, reset the sync variable
	defer vdf.running.Store(false)
	// track the start time to measure the 'processing time'
	startTime := time.Now()
	// run the VDF generation - if Stop() called, this will exit prematurely with y and proof being nil
	y, proof := crypto.GenerateVDF(seed, vdf.Iterations, vdf.stopChan)
	// adjusting variables so must lock for thread safety as the Stop() function may be accessing the `Output`
	// if prematurely stopped
	if y == nil || proof == nil {
		// don't know how long was left in the VDF so decrease iterations by a fixed amount
		// example: 10% fixed decrease on 500 iterations = 450 next iterations
		vdf.Iterations = int(float64(vdf.Iterations) * (1 - IterationsFixedDecreasePercent/100))
		// exit
		return
	}
	// get the duration of the VDF run
	duration := time.Since(startTime)
	vdf.log.Debugf("VDF service completed with %d iterations in %s", vdf.Iterations, duration.String())
	// save the result
	vdf.Results = crypto.VDF{
		Proof:      proof,
		Output:     y,
		Iterations: uint64(vdf.Iterations),
	}
	// adjust the iterations based on completion time
	vdf.adjustIterations(duration)
}

// Finish() signals the service to complete and returns the output
// - already running signals a stop in the running thread and returns
// - not running returns
func (vdf *VDFService) Finish() (results *crypto.VDF) {
	if vdf == nil {
		return
	}
	vdf.log.Debugf("End signaled for VDF service")
	// if service has not yet completed, signal to stop
	if vdf.running.Load() {
		vdf.log.Warn("Prematurely stopping VDF service")
		vdf.stopChan <- struct{}{} // NOTE: multiple sequential calls to stop is not supported
		return
	}
	// if output is empty, it's a premature exit
	if vdf.Results.Output == nil {
		return
	}
	// return last (run) iterations
	return &vdf.Results
}

// VerifyVDF() verifies the VDF using the seed, the proof, and the number of iterations
func (vdf *VDFService) VerifyVDF(seed []byte, results *crypto.VDF) bool {
	return crypto.VerifyVDF(seed, results.Output, results.Proof, int(results.Iterations))
}

// estimateIterationsPerSecond() runs a quick VDF test to determine what the iterations per second is on this processor
// NOTE: longer target times have been observed to complete more iterations quicker,
// so theoretically this a safe starting place
func (vdf *VDFService) estimateIterationsPerSecond() {
	var (
		totalTime  time.Duration
		iterations = 1 // more iterations  = longer startup time
	)
	for i := 0; i < iterations; i++ {
		startTime := time.Now()
		_, _ = crypto.GenerateVDF(nil, StartingIterationsPerSecond, nil)
		totalTime += time.Since(startTime)
	}
	// calculate average seconds per iteration
	averageSeconds := totalTime.Seconds() / float64(iterations)
	// set the iterations number based on the deviation from 1 second
	vdf.Iterations = int(float64(StartingIterationsPerSecond) / averageSeconds)
}

// adjustIterations() changes the number of iterations to be completed based on the
// previous result and the target time
func (vdf *VDFService) adjustIterations(actualTime time.Duration) {
	// coefficient = target_time / actual_time
	adjustmentCoefficient := vdf.TargetTime.Seconds() / actualTime.Seconds()
	// new_iterations = old_iterations * coefficient
	vdf.Iterations = int(float64(vdf.Iterations) * adjustmentCoefficient)
	vdf.log.Debugf("Adjusted iterations for next run to %d", vdf.Iterations)
}
