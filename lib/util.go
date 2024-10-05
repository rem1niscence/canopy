package lib

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/ginchuco/ginchu/lib/crypto"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"math"
	"math/big"
	"reflect"
	"runtime/debug"
	"strings"
	"time"
)

// RegisteredPageables is a global slice of registered pageables for generic unmarshalling
var RegisteredPageables = make(map[string]Pageable)

// Page is a pagination wrapper over a slice of data
type Page struct {
	PageParams
	Results    Pageable `json:"results"`
	Type       string   `json:"type"`
	Count      int      `json:"count"`
	TotalPages int      `json:"totalPages"`
	TotalCount int      `json:"totalCount"`
}

// PageParams are the input parameters to calculate the proper page
type PageParams struct {
	PageNumber int `json:"pageNumber"`
	PerPage    int `json:"perPage"`
}

// SkipToIndex() sanity checks params and then determines the first index of the page
func (p *PageParams) SkipToIndex() int {
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

// Pageable() is a simple interface that represents Page structures
type Pageable interface {
	New() Pageable
	Len() int
}

// NewPage() returns a new instance of the Page object from the params and pageType
// Load() or LoadArray() is the likely next function call
func NewPage(p PageParams, pageType string) *Page { return &Page{PageParams: p, Type: pageType} }

// Load() fills a page from an IteratorI
func (p *Page) Load(storePrefix []byte, newestToOldest bool, results Pageable, db RStoreI, callback func(b []byte) ErrorI) (err ErrorI) {
	// retrieve the iterator
	var it IteratorI
	// set the page results so that even if it's a zero page, it will have a castable type
	p.Results = results
	// prefix keys with numbers in big endian ensure that reverse iteration
	// is newest to oldest and vise versa
	switch newestToOldest {
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
	pageStartIndex := p.SkipToIndex()
	for countOnly, i := false, 0; it.Valid(); func() { it.Next(); i++ }() {
		p.TotalCount++
		if i < pageStartIndex || countOnly {
			continue
		}
		// if reached end of the desired page
		if i == pageStartIndex+p.PerPage {
			countOnly = true // switch to only counts
			continue
		}
		if e := callback(it.Value()); e != nil {
			return e
		}

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
	pageStartIndex, size := p.SkipToIndex(), arr.Len()
	for i, countOnly := 0, false; i < size; i++ {
		p.TotalCount++
		if i < pageStartIndex || countOnly {
			continue
		}
		elem := arr.Index(i).Interface()
		if e := callback(elem); e != nil {
			return e
		}
		// if reached end of the desired page
		if i == pageStartIndex+p.PerPage {
			countOnly = true // switch to only counts
			continue
		}
		p.Results = results
		p.Count++
	}
	// calculate total pages
	p.TotalPages = int(math.Ceil(float64(p.TotalCount) / float64(p.PerPage)))
	return
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

// BzToTruncStr() converts a byte slice to a truncated hexadecimal string
func BzToTruncStr(b []byte) string {
	if len(b) > 10 {
		return hex.EncodeToString(b[:10])
	}
	return hex.EncodeToString(b)
}

// StringToBytes() converts a hexadecimal string back into a byte slice
func StringToBytes(s string) ([]byte, ErrorI) {
	b, err := hex.DecodeString(s)
	if err != nil {
		fmt.Println(s)
		return nil, ErrStringToBytes(err)
	}
	return b, nil
}

// PublicKeyFromBytes() converts a byte slice into a BLS public key
func PublicKeyFromBytes(pubKey []byte) (crypto.PublicKeyI, ErrorI) {
	publicKey, err := crypto.NewBLSPublicKeyFromBytes(pubKey)
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

// BigGreater() compares two big.Int values and returns true if the first is greater
func BigGreater(a *big.Int, b *big.Int) bool { return a.Cmp(b) == 1 }

// BigLess() compares two big.Int values and returns true if the first is less
func BigLess(a *big.Int, b *big.Int) bool { return a.Cmp(b) == -1 }

// Uint64PercentageDiv() calculates the percentage from dividend/divisor
func Uint64PercentageDiv(dividend, divisor uint64) (res uint64) {
	a := Uint64ToBigFloat(dividend)
	b := Uint64ToBigFloat(divisor)
	b.Quo(a, b)
	b.Mul(b, big.NewFloat(100))
	b.Add(b, big.NewFloat(.5)) // round ties away
	res, _ = b.Uint64()
	return
}

// Uint64Percentage() calculates the result of a percentage of an amount
func Uint64Percentage(amount uint64, percentage uint64) (res uint64) {
	a := Uint64ToBigFloat(amount)
	b := big.NewFloat(float64(percentage))
	b.Quo(b, big.NewFloat(100))
	a.Mul(a, b)
	a.Add(a, big.NewFloat(.5)) // round ties away
	res, _ = a.Uint64()
	return
}

// Uint64ReducePercentage() reduces an amount by a specified percentage
func Uint64ReducePercentage(amount uint64, percentage float64) (res uint64) {
	a := Uint64ToBigFloat(amount)
	b := big.NewFloat(100 - percentage)
	b.Quo(b, big.NewFloat(100))
	a.Mul(a, b)
	a.Add(a, big.NewFloat(.5)) // round ties away
	res, _ = a.Uint64()
	return
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

// NewTimer() creates a 0 value initialized instance of a timer
func NewTimer() *time.Timer {
	t := time.NewTimer(0)
	<-t.C
	return t
}

// ResetTimer() stops the existing timer, and resets with the new duration
func ResetTimer(t *time.Timer, d time.Duration) {
	StopTimer(t)
	t.Reset(d)
}

// StopTimer() stops the existing timer
func StopTimer(t *time.Timer) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
}

// CatchPanic() catches any panic in the function call or child function calls
func CatchPanic(l LoggerI) {
	if r := recover(); r != nil {
		l.Errorf(string(debug.Stack()))
	}
}
