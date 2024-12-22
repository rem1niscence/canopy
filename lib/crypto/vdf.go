package crypto

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"math/big"
	"sort"
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
	Results    VDF           // the results from the previous VDF run
	stopChan   chan struct{} // channel to signal an exit for the vdf
	running    *atomic.Bool  // if the vdf service is currently running
}

// NewVDFService() creates a new instance of the VDF service
func NewVDFService(targetTime time.Duration) (vdf *VDFService) {
	vdf = &VDFService{TargetTime: targetTime, stopChan: make(chan struct{}), running: &atomic.Bool{}}
	vdf.estimateIterationsPerSecond()
	return
}

// Run() *blocking call*:  generates a VDF proof using the current params state of the VDF Service object
// The design is to save the results
func (vdf *VDFService) Run(seed []byte) {
	if vdf == nil {
		return
	}
	// - Run() and not running locks and starts a run
	// - Run() and already running returns
	if !vdf.running.CompareAndSwap(false, true) {
		return
	}
	// clear the results object
	vdf.Results = VDF{}
	// at the end of this function, reset the sync variable
	defer vdf.running.Store(false)
	// track the start time to measure the 'processing time'
	startTime := time.Now()
	// run the VDF generation - if Stop() called, this will exit prematurely with y and proof being nil
	y, proof := GenerateVDF(seed, vdf.Iterations, vdf.stopChan)
	// adjusting variables so must lock for thread safety as the Stop() function may be accessing the `Output`
	// if prematurely stopped
	if y == nil || proof == nil {
		// don't know how long was left in the VDF so decrease iterations by a fixed amount
		// example: 10% fixed decrease on 500 iterations = 450 next iterations
		vdf.Iterations = int(float64(vdf.Iterations) * (1 - IterationsFixedDecreasePercent/100))
		// exit
		return
	}
	// save the result
	vdf.Results = VDF{
		Proof:      proof,
		Output:     y,
		Iterations: uint64(vdf.Iterations),
	}
	// adjust the iterations based on completion time
	vdf.adjustIterations(time.Since(startTime))
}

// Finish() signals the service to complete and returns the output
// - already running signals a stop in the running thread and returns
// - not running returns
func (vdf *VDFService) Finish() (results *VDF) {
	if vdf == nil {
		return
	}
	// if service has not yet completed, signal to stop
	if vdf.running.Load() {
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
func (vdf *VDFService) VerifyVDF(seed []byte, results *VDF) bool {
	return VerifyVDF(seed, results.Output, results.Proof, int(results.Iterations))
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
		_, _ = GenerateVDF(nil, StartingIterationsPerSecond, nil)
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
}

/*
	Credit: The base for this implementation is github.com/harmony-one/vdf (last commit 2019)

	Canopy upgrades this code for (1) memory efficiency, (2) code convention + de-duplication, and (3) documentation
*/

// GenerateVDF() executes a verifiable delay function given a seed and a number of iterations
func GenerateVDF(seed []byte, iterations int, stop <-chan struct{}) (out []byte, proofBytes []byte) {
	// initialize the VDF with a seed
	_, classGroup := initVDF(seed)
	// calculate the vdf and return the out and proof
	y, proof := calculateVDF(classGroup, iterations, stop)
	if y == nil || proof == nil {
		return
	} else {
		return y.Encode(), proof.Encode()
	}
}

// VerifyVDF() verifies VDF bytes given a seed and iterations
func VerifyVDF(seed, out, proof []byte, iterations int) bool {
	discriminant, classGroup := initVDF(seed)
	y, p := new(ClassGroup), new(ClassGroup)
	if err := y.Decode(out, discriminant); err != nil {
		return false
	}
	if err := p.Decode(proof, discriminant); err != nil {
		return false
	}
	return verifyProof(classGroup, y, p, iterations)
}

// initVDF() initializes a class group and a discriminant from a seed
func initVDF(seed []byte) (discriminant *big.Int, classGroup *ClassGroup) {
	// create a discriminant (a large, publicly known negative integer)
	discriminant = NewDiscriminant(seed)
	// generate a class group to initialize the VDF
	classGroup = newClassGroup(bigTwo, bigOne, discriminant)
	return
}

// evaluate() performs an optimized evaluation of h ^ (2^T // B) on a ClassGroup
func evaluate(identity *ClassGroup, B *big.Int, T, k, l int, C map[int]*ClassGroup) (result *ClassGroup) {
	// Divide k into two parts: k1 and k0
	k1, k0 := k/2, k-k/2
	// precompute 2^k, 2^k0 and 2^k1
	twoPowK, twoPowK0, twoPowK1 := int64(math.Pow(2, float64(k))), int64(math.Pow(2, float64(k0))), int64(math.Pow(2, float64(k1)))
	// start with the Identity ClassGroup
	x := CloneClassGroup(identity)
	// iterate over l steps
	for j := l - 1; j >= 0; j-- {
		// compute x raised to the power of 2^k
		if x = x.Pow(twoPowK); x == nil {
			return
		}
		// initialize ys with the Identity group, size of 2^k
		ys := make([]*ClassGroup, twoPowK)
		for b := int64(0); b < twoPowK; b++ {
			ys[b] = identity
		}
		// populate ys based on blocks
		for i := 0; i < int(math.Ceil(float64(T)/float64(k*l))); i++ {
			if T-k*(i*l+j+1) < 0 {
				continue
			}
			// get a specific block
			b := getBlock(i*l+j, k, T, B).Int64() // TODO carefully check big.Int to int64 value conversion...might cause serious issues later
			if ys[b] = ys[b].Multiply(C[i*k*l]); ys[b] == nil {
				return
			}
		}

		// first loop: Iterate over b1 in the range [0, 2^k1)
		// this combines blocks based on their higher-order bits
		for b1 := int64(0); b1 < twoPowK1; b1++ {
			z := identity
			for b0 := int64(0); b0 < twoPowK0; b0++ {
				if z = z.Multiply(ys[b1*twoPowK0+b0]); z == nil {
					return
				}
			}
			if z = z.Pow(b1 * twoPowK0); z == nil {
				return
			}
			if x = x.Multiply(z); x == nil {
				return
			}
		}

		// second loop: Iterate over b0 in the range [0, 2^k0)
		// this processes blocks by their lower-order bits first
		for b0 := int64(0); b0 < twoPowK0; b0++ {
			z := identity
			for b1 := int64(0); b1 < twoPowK1; b1++ {
				if z = z.Multiply(ys[b1*twoPowK0+b0]); z == nil {
					return
				}
			}
			if z = z.Pow(b0); z == nil {
				return
			}
			if x = x.Multiply(z); x == nil {
				return
			}
		}
	}

	// return the final computed ClassGroup x
	return x
}

// calculateVDF() executes the VDF and returns the output (y) and the proof
func calculateVDF(x *ClassGroup, iterations int, stop <-chan struct{}) (y, proof *ClassGroup) {
	// approximate time and memory using the number of iterations
	// k is a parameter that controls the time complexity for each proof step
	// L is a parameter that relates to the memory usage for the computation
	L, k := approximateParameters(iterations)
	// k * l determines the "chunk size" for how many iterations are involved in each phase of the VDF calculation
	iterationsPerChunk := k * L
	// calculate the number of loops to be executed
	chunks := int(math.Ceil(float64(iterations) / float64(iterationsPerChunk)))
	// is a list of "checkpoints" where the VDF computation needs to store intermediate results of squaring
	checkpoints := make([]int, chunks+2)
	// populate the checkpoints with the proper indices
	for i := 0; i < chunks+1; i++ {
		checkpoints[i] = i * k * L
	}
	// add iterations to ensure that the final value (after all iterations are completed) is computed and stored
	checkpoints[chunks+1] = iterations
	// execute the main squaring function - this is where the majority of the operation is spent
	powers := iterateSquarings(x, checkpoints, stop)
	// if nil exit
	if powers == nil {
		return nil, nil
	}
	// y is the final output of the calculated 'squarings'
	y = powers[iterations]
	// generate a proof using input x and final output y
	proof = generateProof(x, y, iterations, k, L, powers)
	// return the final output and proof
	return y, proof
}

// generateProof() generates a proof given input x and final output y
// Equation y = x ^ (2 ^T) and pi
func generateProof(x, y *ClassGroup, T, k, l int, powers map[int]*ClassGroup) *ClassGroup {
	// serialize x and y
	xBytes, yBytes := x.Encode(), y.Encode()
	// generate a random proof from xBytes and yBytes
	B := hashPrime(xBytes, yBytes)
	// execute the optimized evaluation
	return evaluate(x.Identity(), B, T, k, l, powers)
}

// approximateParameters() approximates L and k, based on the number of iterations T and the amount of memory available
// This function matches the paper which uses these parameters to balance the tradeoff between time and memory usage
// - L represents the memory constraint
// - K represents the time constraint
// - T is the number of iterations
func approximateParameters(iterations int) (int, int) {
	// memory limit is set to 10M based on paper
	const memoryLimit = 10000000
	// calculate convenience variables
	log2, L := math.Log(2), 1
	logMemory := math.Log(memoryLimit) / log2
	logTime := math.Log(float64(iterations)) / log2
	// if the number of iterations is greater than the memory limit, adjust L
	if logTime-logMemory > 0 {
		L = int(math.Ceil(math.Pow(2, logMemory-20)))
	}
	// Total time for proof: T/k + L * 2^(k+1)
	// To optimize, set left equal to right, and solve for k
	// intermediate = T * log(2) / (2 * L)
	// k ≈ log(intermediate) - log(log(intermediate)) + 0.25
	// This is a simplified version of the product log approximation (W)
	intermediate := float64(iterations) * log2 / float64(2*L)
	k := int(math.Max(math.Round(math.Log(intermediate)-math.Log(math.Log(intermediate))+0.25), 1))

	return L, k
}

// iterateSquarings() incrementally calculates powers of the class group x by repeatedly squaring it
// At each checkpoint (milestone) specified by powersToCalculate, it stores the current power
func iterateSquarings(x *ClassGroup, checkpoints []int, stop <-chan struct{}) map[int]*ClassGroup {
	// setup variables
	powersSaved, previous := make(map[int]*ClassGroup), 0
	// set up a pointer for the current power
	currX := CloneClassGroup(x)
	// ensure the checkpoints are sorted in ascending order
	sort.Ints(checkpoints)
	// for each milestone
	for _, current := range checkpoints {
		// calculate the number of square operations between the last milestone and the next
		iterations := current - previous
		// execute the square ops
		for i := 0; i < iterations; i++ {
			currX = currX.Pow(2)
			if currX == nil {
				return nil
			}
		}
		// increment previous
		previous = current
		// save the power at the milestone
		powersSaved[current] = currX
		// check to see if stop was triggered
		select {
		case <-stop:
			return nil
		default:
		}
	}
	// return
	return powersSaved
}

// verifyProof() checks the validity of a proof for x and y in the ClassGroup
func verifyProof(x, y, proof *ClassGroup, T int) (result bool) {
	var z, pToB, xToR *ClassGroup
	// calculate B = hashPrime(xBytes, yBytes), a prime derived from the serialized inputs
	B := hashPrime(x.Encode(), y.Encode())
	// r = 2^T mod B
	TBig := bip.New().SetInt64(int64(T))
	r := bip.New().Exp(bigTwo, TBig, B)
	defer bip.Recycle(r, TBig)
	// proof^B
	if pToB = proof.BigPow(B); pToB == nil {
		return
	}
	// x^r
	if xToR = x.BigPow(r); xToR == nil {
		return
	}
	// z = proof^B * x^r
	if z = pToB.Multiply(xToR); z == nil {
		return
	}
	// check if z equals y and return the result
	return z.Equal(y)
}

// hashPrime() creates a random prime based on input x, y
func hashPrime(x, y []byte) *big.Int {
	// pre-allocate a buffer with iBuf (8 bytes) + x + y
	buffer := make([]byte, 8+len(x)+len(y))
	copy(buffer[8:], x)
	copy(buffer[8+len(x):], y)
	z := bip.New()
	// reference the portion of the buffer for iBuf
	iBuf := buffer[:8]
	for i := 0; ; i++ {
		// write the integer `i` to the pre-allocated `iBuf`
		binary.BigEndian.PutUint64(iBuf, uint64(i))
		// set the bytes of z as the hash of the buffer
		z.SetBytes(Hash(buffer)[:16])
		// check primality
		if z.ProbablyPrime(1) {
			return z
		}
	}
}

// getBlock() calculates the ith block of the form 2^T // B,
// where the sum of all get_block(i) * 2^ki equals t^T // B
func getBlock(i, k, T int, B *big.Int) *big.Int {
	// create temporary variables for intermediate calculations
	baseValue := bip.New() // 2^k
	expValue := bip.New()  // 2^(T - k*(i+1))
	result := bip.New()
	defer bip.Recycle(baseValue, expValue, result)
	// calculate 2^k
	baseValue.SetInt64(int64(math.Pow(2, float64(k))))
	// calculate 2^(T - k*(i + 1))
	expValue.Exp(bigTwo, result.SetInt64(int64(T-k*(i+1))), B)
	// multiply the results and divide by B
	return floorDivision(result.Mul(baseValue, expValue), B)
}

// vdfJSON is a helper struct to implement the json.Marshaller and json.Unmarshaler interface for VDF
type vdfJSON struct {
	Proof      string `json:"proof,omitempty"`
	Output     string `json:"output,omitempty"`
	Iterations uint64 `json:"iterations,omitempty"`
}

// MarshalJSON() implements the json.Marshaller interface for VDF
func (x *VDF) MarshalJSON() ([]byte, error) {
	return json.Marshal(&vdfJSON{
		Proof:      hex.EncodeToString(x.Proof),
		Output:     hex.EncodeToString(x.Output),
		Iterations: x.Iterations,
	})
}

// MarshalJSON() implements the json.Marshaller interface for VDF
func (x *VDF) UnmarshalJSON(b []byte) (err error) {
	j := new(vdfJSON)
	if err = json.Unmarshal(b, j); err != nil {
		return
	}
	// hex decode the proof
	proof, err := hex.DecodeString(j.Proof)
	if err != nil {
		return
	}
	// hex decode the output
	output, err := hex.DecodeString(j.Output)
	if err != nil {
		return
	}
	*x = VDF{
		Proof:      proof,
		Output:     output,
		Iterations: j.Iterations,
	}
	return
}
