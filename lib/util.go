package lib

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/canopy-network/canopy/lib/crypto"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"log"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

// RegisteredPageables is a global slice of registered pageables for generic unmarshalling
var RegisteredPageables = make(map[string]Pageable)

// Page is a pagination wrapper over a slice of data
type Page struct {
	PageParams          // the input parameters for the page
	Results    Pageable `json:"results"`    // the actual returned array of items
	Type       string   `json:"type"`       // the type of the page
	Count      int      `json:"count"`      // count of items included in the page
	TotalPages int      `json:"totalPages"` // number of pages that exist based on these page parameters
	TotalCount int      `json:"totalCount"` // count of items that exist
}

// PageParams are the input parameters to calculate the proper page
type PageParams struct {
	PageNumber int `json:"pageNumber"`
	PerPage    int `json:"perPage"`
}

// Pageable() is a simple interface that represents Page structures
type Pageable interface{ New() Pageable }

// NewPage() returns a new instance of the Page object from the params and pageType
// Load() or LoadArray() is the likely next function call
func NewPage(p PageParams, pageType string) *Page { return &Page{PageParams: p, Type: pageType} }

// Load() fills a page from an IteratorI
func (p *Page) Load(storePrefix []byte, reverse bool, results Pageable, db RStoreI, callback func(k, v []byte) ErrorI) (err ErrorI) {
	var it IteratorI
	// set the page results so that even if it's a zero page, it will have a castable type
	p.Results = results
	// prefix keys with numbers in big endian ensure that reverse iteration
	// is highest to lowest and vise versa
	switch reverse {
	case true:
		it, err = db.RevIterator(storePrefix)
	case false:
		it, err = db.Iterator(storePrefix)
	}
	if err != nil {
		return err
	}
	defer it.Close()
	// skip to index makes the starting point appropriate based on the page params
	// initialize variable to indicate if the loop is counting only or actually populating
	pageStartIndex, countOnly := p.skipToIndex(), false
	// execute the loop
	for ; it.Valid(); it.Next() {
		// pre-increment total count to ensure each iteration of the loop is counted including if !it.Valid() or `countOnly`
		p.TotalCount++
		// while count is below the start page index (LTE because we pre-increment)
		if p.TotalCount <= pageStartIndex || countOnly {
			continue
		}
		// if reached end of the desired page (+1 because we pre-increment)
		if p.TotalCount == pageStartIndex+p.PerPage+1 {
			countOnly = true // switch to only counts
			continue
		}
		// execute the callback; passing key and value
		if e := callback(it.Key(), it.Value()); e != nil {
			return e
		}
		// set the results and increment the count
		p.Results = results
		p.Count++
	}
	// calculate total pages
	p.TotalPages = int(math.Ceil(float64(p.TotalCount) / float64(p.PerPage)))
	return
}

// LoadArray() fills a page from a slice
func (p *Page) LoadArray(slice any, results Pageable, callback func(i any) ErrorI) (err ErrorI) {
	arr := reflect.ValueOf(slice)
	if arr.Kind() != reflect.Slice {
		return ErrInvalidArgument()
	}
	// skip to index makes the starting point appropriate based on the page params
	pageStartIndex, size := p.skipToIndex(), arr.Len()
	// initialize variable to indicate if the loop is counting only or actually populating
	countOnly := false
	for p.TotalCount < size {
		// pre-increment total count to ensure each iteration of the loop is counted including if p.TotalCount > size or `countOnly`
		p.TotalCount++
		// while count is below the start page index (LTE because we pre-increment)
		if p.TotalCount <= pageStartIndex || countOnly {
			continue
		}
		elem := arr.Index(p.TotalCount - 1).Interface()
		if e := callback(elem); e != nil {
			return e
		}
		// if reached end of the desired page (+1 because we pre-increment)
		if p.TotalCount-1 == pageStartIndex+p.PerPage {
			countOnly = true // switch to only counts
			continue
		}
		// set the results and increment the count
		p.Results = results
		p.Count++
	}
	// calculate total pages
	p.TotalPages = int(math.Ceil(float64(p.TotalCount) / float64(p.PerPage)))
	return
}

// skipToIndex() sanity checks params and then determines the first index of the page
func (p *PageParams) skipToIndex() int {
	defaultPerPage, maxPerPage := 10, 5000
	if p.PerPage == 0 {
		p.PerPage = defaultPerPage
	}
	if p.PerPage > maxPerPage {
		p.PerPage = maxPerPage
	}
	// start page count at 1 not 0
	if p.PageNumber == 0 {
		p.PageNumber = 1
	}
	if p.PageNumber == 1 {
		return 0
	}
	lastPage := p.PageNumber - 1
	return lastPage * p.PerPage
}

// UnmarshalJSON() overrides the unmarshalling logic of the
// Page for generic structure assignment (registered pageables) and custom formatting
func (p *Page) UnmarshalJSON(b []byte) error {
	var j jsonPage
	if err := json.Unmarshal(b, &j); err != nil {
		return err
	}
	var pageable Pageable
	m, ok := RegisteredPageables[j.Type]
	if !ok {
		return ErrUnknownPageable(j.Type)
	}
	pageable = m.New()
	if err := json.Unmarshal(j.Results, pageable); err != nil {
		return err
	}
	*p = Page{
		PageParams: j.PageParams,
		Results:    pageable,
		Type:       j.Type,
		Count:      j.Count,
		TotalPages: j.TotalPages,
		TotalCount: j.TotalCount,
	}
	return nil
}

// jsonPage is the internal structure for custom json for the Page structure
type jsonPage struct {
	PageParams
	Results    json.RawMessage `json:"results"`
	Type       string          `json:"type"`
	Count      int             `json:"count"`
	TotalPages int             `json:"totalPages"`
	TotalCount int             `json:"totalCount"`
}

// Marshal() serializes a proto.Message into a byte slice
func Marshal(message any) ([]byte, ErrorI) {
	bz, err := proto.Marshal(message.(proto.Message))
	if err != nil {
		return nil, ErrMarshal(err)
	}
	return bz, nil
}

// Unmarshal() deserializes a byte slice into a proto.Message
func Unmarshal(data []byte, ptr any) ErrorI {
	if data == nil || ptr == nil {
		return nil
	}
	if err := proto.Unmarshal(data, ptr.(proto.Message)); err != nil {
		return ErrUnmarshal(err)
	}
	return nil
}

// MarshalJSON() serializes a message into a JSON byte slice
func MarshalJSON(message any) ([]byte, ErrorI) {
	bz, err := json.Marshal(message)
	if err != nil {
		return nil, ErrJSONMarshal(err)
	}
	return bz, nil
}

// MarshalJSONIndent() serializes a message into an indented JSON byte slice
func MarshalJSONIndent(message any) ([]byte, ErrorI) {
	bz, err := json.MarshalIndent(message, "", "  ")
	if err != nil {
		return nil, ErrJSONMarshal(err)
	}
	return bz, nil
}

// MarshalJSONIndentString() serializes a message into an indented JSON string
func MarshalJSONIndentString(message any) (string, ErrorI) {
	bz, err := MarshalJSONIndent(message)
	return string(bz), err
}

// UnmarshalJSON() deserializes a JSON byte slice into the specified object
func UnmarshalJSON(bz []byte, ptr any) ErrorI {
	if err := json.Unmarshal(bz, ptr); err != nil {
		return ErrJSONUnmarshal(err)
	}
	return nil
}

// NewJSONFromFile() reads a json object from file
func NewJSONFromFile(o any, dataDirPath, filePath string) ErrorI {
	bz, err := os.ReadFile(filepath.Join(dataDirPath, filePath))
	if err != nil {
		return ErrReadFile(err)
	}
	return UnmarshalJSON(bz, &o)
}

// SaveJSONToFile() saves a json object to a file
func SaveJSONToFile(j any, dataDirPath, filePath string) (err ErrorI) {
	bz, err := MarshalJSONIndent(j)
	if err != nil {
		return
	}
	if e := os.WriteFile(filepath.Join(dataDirPath, filePath), bz, os.ModePerm); e != nil {
		return ErrWriteFile(e)
	}
	return
}

// NewAny() converts a proto.Message into an anypb.Any type
func NewAny(message proto.Message) (*anypb.Any, ErrorI) {
	a, err := anypb.New(message)
	if err != nil {
		return nil, ErrToAny(err)
	}
	return a, nil
}

// FromAny() converts an anypb.Any type back into a proto.Message
func FromAny(any *anypb.Any) (proto.Message, ErrorI) {
	msg, err := anypb.UnmarshalNew(any, proto.UnmarshalOptions{})
	if err != nil {
		return nil, ErrFromAny(err)
	}
	return msg, nil
}

// BytesToString() converts a byte slice to a hexadecimal string
func BytesToString(b []byte) string {
	return hex.EncodeToString(b)
}

// StringToBytes() converts a hexadecimal string back into a byte slice
func StringToBytes(s string) ([]byte, ErrorI) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, ErrStringToBytes(err)
	}
	return b, nil
}

// BytesToTruncatedString() converts a byte slice to a truncated hexadecimal string
func BytesToTruncatedString(b []byte) string {
	if len(b) > 10 {
		return hex.EncodeToString(b[:10])
	}
	return hex.EncodeToString(b)
}

// PublicKeyFromBytes() converts a byte slice into a BLS public key
func PublicKeyFromBytes(pubKey []byte) (crypto.PublicKeyI, ErrorI) {
	publicKey, err := crypto.NewPublicKeyFromBytes(pubKey)
	if err != nil {
		return nil, ErrPubKeyFromBytes(err)
	}
	return publicKey, nil
}

// MerkleTree() generates a Merkle tree and its root from a list of items
func MerkleTree(items [][]byte) (root []byte, tree [][]byte, err ErrorI) {
	root, tree, er := crypto.MerkleTree(items)
	if er != nil {
		return nil, nil, ErrMerkleTree(er)
	}
	return
}

// BigLess() compares two big.Int values and returns true if the first is less
func BigLess(a *big.Int, b *big.Int) bool { return a.Cmp(b) == -1 }

// Uint64PercentageDiv() calculates the percentage from dividend/divisor
func Uint64PercentageDiv(dividend, divisor uint64) (percent uint64) {
	if dividend == 0 || divisor == 0 {
		return 0
	}
	// calculate the percent
	percent = (dividend * 100) / divisor
	// ensure the percent can't exceed 100
	if percent > 100 {
		percent = 100
	}
	return percent
}

// Uint64Percentage() calculates the result of a percentage of an amount
func Uint64Percentage(amount uint64, percentage uint64) (res uint64) {
	if percentage == 0 || amount == 0 {
		return 0
	}
	if percentage >= 100 {
		return amount
	}
	return (amount * percentage) / 100
}

// Uint64ReducePercentage() reduces an amount by a specified percentage
func Uint64ReducePercentage(amount, percentage uint64) (res uint64) {
	if percentage >= 100 || amount == 0 {
		return 0
	}
	if percentage == 0 {
		return amount
	}
	return (amount * (100 - percentage)) / 100
}

// Uint64ToBigFloat() converts a uint64 to a big.Float
func Uint64ToBigFloat(u uint64) *big.Float {
	return new(big.Float).SetUint64(u)
}

// HexBytes represents a byte slice that can be marshaled and unmarshalled as hex strings
type HexBytes []byte

// NewHexBytesFromString() converts a hexadecimal string into HexBytes
func NewHexBytesFromString(s string) (HexBytes, ErrorI) {
	bz, err := hex.DecodeString(s)
	if err != nil {
		return nil, ErrJSONUnmarshal(err)
	}
	return bz, nil
}

// String() returns the HexBytes as a hexadecimal string
func (x HexBytes) String() string {
	return BytesToString(x)
}

// MarshalJSON() serializes the HexBytes to a JSON byte slice
func (x HexBytes) MarshalJSON() ([]byte, error) {
	return json.Marshal(BytesToString(x))
}

// UnmarshalJSON() deserializes a JSON byte slice into HexBytes
func (x *HexBytes) UnmarshalJSON(b []byte) (err error) {
	var s string
	if err = json.Unmarshal(b, &s); err != nil {
		return err
	}
	*x, err = StringToBytes(s)
	return
}

// RemoveIPV4Port() removes the port from an IP address or URL string
func RemoveIPV4Port(address string) (string, ErrorI) {
	split := strings.Split(address, ":")
	switch len(split) {
	case 2:
		return split[0], nil
	case 3:
		return strings.Join(split[:2], ":"), nil
	default:
		return "", ErrInvalidArgument()
	}
}

// ValidNetURLInput() validates the input netURL via regex
// Allow:
// - optional tcp:// prefix
// - valid hostname
// - valid ip4 and ip6 address
//
// Disallow:
// - Ports
// - Sub-paths
func ValidNetURLInput(netURL string) bool {
	// regex for optional tcp://, valid hostname, or IP with no ports
	regex := `^(?:tcp:\/\/)?(?:localhost|(?:[a-zA-Z0-9-]+\.)*[a-zA-Z0-9-]+|(?:\d{1,3}\.){3}\d{1,3}|(?:\[[0-9a-fA-F:]+\]))$`

	matched, err := regexp.MatchString(regex, netURL)
	if err != nil {
		return false
	}
	return matched
}

// AddToPort() adds some number to the port ensuring it doesn't exceed the max port
func AddToPort(portStr string, add uint64) (string, ErrorI) {
	portPart := portStr[1:]
	port, _ := strconv.Atoi(portPart)
	// add the given number to the port
	newPort := port + int(add)
	// ensure the new port doesn't exceed the max port number (65535)
	if newPort > 65535 {
		return "", ErrMaxPort()
	}
	return fmt.Sprintf(":%d", newPort), nil
}

// NewTimer() creates a 0 value initialized instance of a timer
func NewTimer() *time.Timer {
	t := time.NewTimer(0)
	<-t.C
	return t
}

// ResetTimer() stops the existing timer, and resets with the new duration
func ResetTimer(t *time.Timer, d time.Duration) {
	if t == nil {
		*t = *time.NewTimer(d)
	}
	StopTimer(t)
	t.Reset(d)
}

// StopTimer() stops the existing timer
func StopTimer(t *time.Timer) {
	if t == nil {
		return
	}
	if !t.Stop() {
		// drain safely
		for len(t.C) > 0 {
			<-t.C
		}
	}
}

// CatchPanic() catches any panic in the function call or child function calls
func CatchPanic(l LoggerI) {
	if r := recover(); r != nil {
		l.Errorf(string(debug.Stack()))
	}
}

// JoinLenPrefix() appends the items together separated by a single byte to represent the length of the segment
func JoinLenPrefix(toAppend ...[]byte) (res []byte) {
	// for each item to append
	for _, item := range toAppend {
		if item == nil {
			continue
		}
		// store the length of the segment in a single byte
		length := []byte{byte(len(item))}
		// append to the reset of the segment
		res = append(append(res, length...), item...)
	}
	return
}

// DecodeLengthPrefixed() decodes a key that is delimited by the length of the segment in a single byte
func DecodeLengthPrefixed(key []byte) (segments [][]byte) {
	var length int
	for i := 0; i < len(key); i += length {
		if i >= len(key) {
			break
		}
		// read the length prefix
		length = int(key[i])
		i++
		if i+length > len(key) {
			panic("corrupt or incomplete key")
		}
		segments = append(segments, key[i:i+length])
	}
	return
}

// Retry is a simple exponential backoff retry structure in the form of doubling the timeout
type Retry struct {
	waitTimeMS uint64
	maxLoops   uint64
	loopCount  uint64
}

// NewRetry() constructs a new Retry given parameters
func NewRetry(waitTimeMS, maxLoops uint64) *Retry {
	return &Retry{
		waitTimeMS: waitTimeMS,
		maxLoops:   maxLoops,
	}
}

// WaitAndDoRetry() sleeps the appropriate time and returns false if maxed out retry
func (r *Retry) WaitAndDoRetry() bool {
	if r.maxLoops <= r.loopCount {
		return false
	}
	time.Sleep(time.Duration(r.waitTimeMS) * time.Millisecond)
	// double the timeout
	r.waitTimeMS += r.waitTimeMS
	r.loopCount++
	return true
}

// TimeTrack() a utility function to benchmark the time
func TimeTrack(start time.Time) {
	elapsed := time.Since(start)

	// Skip this function, and fetch the PC and file for its parent.
	pc, _, _, _ := runtime.Caller(1)

	// Retrieve a function object this functions parent.
	funcObj := runtime.FuncForPC(pc)

	// Regex to extract just the function name (and not the module path).
	runtimeFunc := regexp.MustCompile(`^.*\.(.*)$`)
	name := runtimeFunc.ReplaceAllString(funcObj.Name(), "$1")

	log.Println(fmt.Sprintf("%s took %s", name, elapsed))
}

// TruncateSlice() safely ensures that a slice doesn't exceed the max size
func TruncateSlice[T any](slice []T, max int) []T {
	if slice == nil {
		return nil
	}
	if len(slice) > max {
		return slice[:max]
	}
	return slice
}

// DeDuplicator is a generic structure that serves as a simple anti-duplication check
type DeDuplicator[T comparable] struct {
	m map[T]struct{}
}

// NewDeDuplicator constructs a new object reference to a DeDuplicator
func NewDeDuplicator[T comparable]() *DeDuplicator[T] {
	return &DeDuplicator[T]{m: make(map[T]struct{})}
}

// Found checks for an existing entry and adds it to the map if it's not present
func (d *DeDuplicator[T]) Found(k T) bool {
	// check if the key already exists
	if _, exists := d.m[k]; exists {
		return true // It's a duplicate
	}
	// add the key to the map
	d.m[k] = struct{}{}
	// not a duplicate
	return false
}

func PrintStackTrace() {
	pc := make([]uintptr, 10) // Get at most 10 stack frames
	n := runtime.Callers(2, pc)
	frames := runtime.CallersFrames(pc[:n])

	fmt.Println("Stack trace:")
	for {
		frame, more := frames.Next()
		fmt.Printf("%s\n\t%s:%d\n", frame.Function, frame.File, frame.Line)
		if !more {
			break
		}
	}
}
