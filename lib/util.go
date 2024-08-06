package lib

import (
	"container/list"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/ginchuco/ginchu/lib/crypto"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"math/big"
	"net/url"
	"strings"
	"time"
)

type Page struct {
	PageParams
	Results    Pageable `json:"results"`
	Type       string   `json:"type"`
	Count      int      `json:"count"`
	TotalPages int      `json:"totalPages"`
	TotalCount int      `json:"totalCount"`
}

func NewPage(p PageParams) *Page {
	return &Page{PageParams: p}
}

type PageParams struct {
	PageNumber int `json:"pageNumber"`
	PerPage    int `json:"perPage"`
}

type ValidatorFilters struct {
	Unstaking FilterOption `json:"unstaking"`
	Paused    FilterOption `json:"paused"`
	Delegate  FilterOption `json:"delegate"`
}

func (v ValidatorFilters) On() bool {
	return v.Unstaking != 0 || v.Paused != 0
}

func (p *PageParams) SkipToIndex() int {
	if p.PerPage == 0 {
		p.PerPage = 10
	}
	if p.PerPage > 5000 {
		p.PerPage = 5000
	}
	if p.PageNumber == 0 {
		p.PageNumber = 1
	}
	if p.PageNumber == 1 {
		return 0
	}
	lastPage := p.PageNumber - 1
	return lastPage * p.PerPage
}

type jsonPage struct {
	PageParams
	Results    json.RawMessage `json:"results"`
	Type       string          `json:"type"`
	Count      int             `json:"count"`
	TotalPages int             `json:"totalPages"`
	TotalCount int             `json:"totalCount"`
}

type Pageable interface {
	New() Pageable
	Len() int
}

var RegisteredPageables = make(map[string]Pageable)

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

type SimpleLimiter struct {
	requests        map[string]int
	totalRequests   int
	maxPerRequester int
	maxRequests     int
	reset           *time.Ticker
}

func NewLimiter(maxPerRequester, maxRequests, resetWindowS int) *SimpleLimiter {
	return &SimpleLimiter{
		requests:        map[string]int{},
		maxPerRequester: maxPerRequester,
		maxRequests:     maxRequests,
		reset:           time.NewTicker(time.Duration(resetWindowS) * time.Second),
	}
}

func (l *SimpleLimiter) NewRequest(requester string) (requesterBlock, totalBlock bool) {
	if l.totalRequests >= l.maxRequests {
		return false, true
	}
	if count := l.requests[requester]; count >= l.maxPerRequester {
		return true, false
	}
	l.requests[requester]++
	l.totalRequests++
	return
}

func (l *SimpleLimiter) Reset() {
	l.requests = map[string]int{}
	l.totalRequests = 0
}

func (l *SimpleLimiter) C() <-chan time.Time {
	return l.reset.C
}

const (
	MaxMessageCacheSize = 10000
)

type MessageCache struct {
	queue *list.List
	m     map[string]struct{}
}

func NewMessageCache() MessageCache {
	return MessageCache{
		queue: list.New(),
		m:     map[string]struct{}{},
	}
}

func (c MessageCache) Add(msg *MessageWrapper) bool {
	k := BytesToString(msg.Hash)
	if _, found := c.m[k]; found {
		return false
	}
	if c.queue.Len() >= MaxMessageCacheSize {
		e := c.queue.Front()
		message := e.Value.(*MessageWrapper)
		delete(c.m, BytesToString(message.Hash))
		c.queue.Remove(e)
	}
	c.m[k] = struct{}{}
	c.queue.PushFront(msg)
	return true
}

func Marshal(message any) ([]byte, ErrorI) {
	bz, err := proto.Marshal(message.(proto.Message))
	if err != nil {
		return nil, ErrMarshal(err)
	}
	return bz, nil
}

func Unmarshal(data []byte, ptr any) ErrorI {
	if data == nil || ptr == nil {
		return nil
	}
	if err := proto.Unmarshal(data, ptr.(proto.Message)); err != nil {
		return ErrUnmarshal(err)
	}
	return nil
}

func MarshalJSON(message any) ([]byte, ErrorI) {
	bz, err := json.Marshal(message)
	if err != nil {
		return nil, ErrJSONMarshal(err)
	}
	return bz, nil
}

func MarshalJSONIndent(message any) ([]byte, ErrorI) {
	bz, err := json.MarshalIndent(message, "", "  ")
	if err != nil {
		return nil, ErrJSONMarshal(err)
	}
	return bz, nil
}

func MarshalJSONIndentString(message any) (string, ErrorI) {
	bz, err := MarshalJSONIndent(message)
	return string(bz), err
}

func UnmarshalJSON(bz []byte, ptr any) ErrorI {
	if err := json.Unmarshal(bz, ptr); err != nil {
		return ErrJSONUnmarshal(err)
	}
	return nil
}

func NewAny(message proto.Message) (*anypb.Any, ErrorI) {
	a, err := anypb.New(message)
	if err != nil {
		return nil, ErrToAny(err)
	}
	return a, nil
}

func FromAny(any *anypb.Any) (proto.Message, ErrorI) {
	msg, err := anypb.UnmarshalNew(any, proto.UnmarshalOptions{})
	if err != nil {
		return nil, ErrFromAny(err)
	}
	return msg, nil
}

func ProtoEnumToBytes(i uint32) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(i))
	return b
}

func BytesToString(b []byte) string {
	return hex.EncodeToString(b)
}

func BzToTruncStr(b []byte) string {
	if len(b) > 10 {
		return hex.EncodeToString(b[:10])
	}
	return hex.EncodeToString(b)
}

func StringToBytes(s string) ([]byte, ErrorI) {
	b, err := hex.DecodeString(s)
	if err != nil {
		fmt.Println(s)
		return nil, ErrStringToBytes(err)
	}
	return b, nil
}

func PublicKeyFromBytes(pubKey []byte) (crypto.PublicKeyI, ErrorI) {
	publicKey, err := crypto.NewBLSPublicKeyFromBytes(pubKey)
	if err != nil {
		return nil, ErrPubKeyFromBytes(err)
	}
	return publicKey, nil
}

func MerkleTree(items [][]byte) (root []byte, tree [][]byte, err ErrorI) {
	root, tree, er := crypto.MerkleTree(items)
	if er != nil {
		return nil, nil, ErrMerkleTree(er)
	}
	return
}

func CopyBytes(bz []byte) (dst []byte) {
	dst = make([]byte, len(bz))
	copy(dst, bz)
	return
}
func BigGreater(a *big.Int, b *big.Int) bool { return a.Cmp(b) == 1 }

func Uint64PercentageDiv(dividend, divisor uint64) (res uint64) {
	a := Uint64ToBigFloat(dividend)
	b := Uint64ToBigFloat(divisor)
	b.Quo(a, b)
	b.Mul(b, big.NewFloat(100))
	b.Add(b, big.NewFloat(.5)) // round ties away
	res, _ = b.Uint64()
	return
}

func Uint64Percentage(amount uint64, percentage uint64) (res uint64) {
	a := Uint64ToBigFloat(amount)
	b := big.NewFloat(float64(percentage))
	b.Quo(b, big.NewFloat(100))
	a.Mul(a, b)
	a.Add(a, big.NewFloat(.5)) // round ties away
	res, _ = a.Uint64()
	return
}

func Uint64ReducePercentage(amount uint64, percentage float64) (res uint64) {
	a := Uint64ToBigFloat(amount)
	b := big.NewFloat(100 - percentage)
	b.Quo(b, big.NewFloat(100))
	a.Mul(a, b)
	a.Add(a, big.NewFloat(.5)) // round ties away
	res, _ = a.Uint64()
	return
}

func Uint64ToBigFloat(u uint64) *big.Float {
	return new(big.Float).SetUint64(u)
}

func RoundFloatToUint64(f float64) uint64 {
	if f < 0 {
		return uint64(f)
	}
	return uint64(f + .5) // Round ties away
}

type HexBytes []byte

func NewHexBytesFromString(s string) (HexBytes, ErrorI) {
	bz, err := hex.DecodeString(s)
	if err != nil {
		return nil, ErrJSONUnmarshal(err)
	}
	return bz, nil
}

func (x HexBytes) String() string {
	return BytesToString(x)
}

func (x HexBytes) MarshalJSON() ([]byte, error) {
	return json.Marshal(BytesToString(x))
}
func (x *HexBytes) UnmarshalJSON(b []byte) (err error) {
	var s string
	if err = json.Unmarshal(b, &s); err != nil {
		return err
	}
	*x, err = StringToBytes(s)
	return
}

var (
	MaxHash, MaxAddress = []byte(strings.Repeat("F", crypto.HashSize)), []byte(strings.Repeat("F", crypto.AddressSize))
)

func ReplaceURLPort(rawURL, replacementPort string) (returned string, err error) {
	u, err := url.Parse(rawURL)
	port := u.Port()
	if port == "" {
		return rawURL + ":" + replacementPort, nil
	}
	returned = strings.ReplaceAll(rawURL, u.Port(), replacementPort)
	return
}

type FilterOption int

// nolint:all
const (
	Both FilterOption = 0
	Yes               = 1
	No                = 2
)
