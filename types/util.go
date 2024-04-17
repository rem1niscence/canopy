package types

import (
	"container/list"
	"encoding/binary"
	"encoding/hex"
	"github.com/ginchuco/ginchu/types/codec"
	"github.com/ginchuco/ginchu/types/crypto"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"math/big"
	"strconv"
	"time"
)

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

var (
	cdc = codec.Protobuf{}
)

func Marshal(message any) ([]byte, ErrorI) {
	bz, err := proto.Marshal(message.(proto.Message))
	if err != nil {
		return nil, ErrMarshal(err)
	}
	return bz, nil
}

func Unmarshal(data []byte, ptr any) ErrorI {
	if err := proto.Unmarshal(data, ptr.(proto.Message)); err != nil {
		return ErrUnmarshal(err)
	}
	return nil
}

func ToAny(message proto.Message) (*anypb.Any, ErrorI) {
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

func StringToBytes(s string) ([]byte, ErrorI) {
	b, err := hex.DecodeString(s)
	if err != nil {
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

func BigAdd(a, b *big.Int) *big.Int          { return new(big.Int).Add(a, b) }
func BigSub(a, b *big.Int) *big.Int          { return new(big.Int).Sub(a, b) }
func BigMul(a, b *big.Int) *big.Int          { return new(big.Int).Mul(a, b) }
func BigDiv(a, b *big.Int) *big.Int          { return new(big.Int).Div(a, b) }
func BigIsZero(a *big.Int) bool              { return len(a.Bits()) == 0 }
func BigEqual(a, b *big.Int) bool            { return a.Cmp(b) == 0 }
func BigLTE(a, b *big.Int) bool              { return a.Cmp(b) <= 0 }
func BigGTE(a *big.Int, b *big.Int) bool     { return a.Cmp(b) >= 0 }
func BigLess(a *big.Int, b *big.Int) bool    { return a.Cmp(b) == -1 }
func BigGreater(a *big.Int, b *big.Int) bool { return a.Cmp(b) == 1 }

func StringReducePercentage(amount string, percent int8) (string, ErrorI) {
	// convert amount to big.Int
	a, err := StringToBigInt(amount)
	if err != nil {
		return "", err
	}
	// convert to a percent float: ex 1 - .05 = .95
	percentFloat := big.NewFloat(float64(1) - float64(percent)/float64(100))
	// amountBig.ToFloat() * percentFloat: ex 100 * .95 = 95 (reduced by 5%)
	result := new(big.Float).Mul(new(big.Float).SetInt(a), percentFloat)
	// truncate to big.Int
	c, _ := result.Int(nil)
	// convert back to string
	return BigIntToString(c), nil
}

func StringBigLTE(s string, b *big.Int) (bool, ErrorI) {
	a, err := StringToBigInt(s)
	if err != nil {
		return false, err
	}
	return BigLTE(a, b), nil
}

func StringsLess(s, s2 string) (bool, ErrorI) {
	cmp, err := StringsCmp(s, s2)
	if err != nil {
		return false, err
	}
	return cmp == -1, nil
}

func StringsGTE(s, s2 string) (bool, ErrorI) {
	cmp, err := StringsCmp(s, s2)
	if err != nil {
		return false, err
	}
	return cmp >= 0, nil
}

func StringsCmp(s, s2 string) (int, ErrorI) {
	a, err := StringToBigInt(s)
	if err != nil {
		return 0, err
	}
	b, err := StringToBigInt(s2)
	if err != nil {
		return 0, err
	}
	return a.Cmp(b), nil
}

func StringBigAdd(s string, s2 string) (string, ErrorI) {
	a, err := StringToBigInt(s)
	if err != nil {
		return "", err
	}
	b, err := StringToBigInt(s2)
	if err != nil {
		return "", err
	}
	c := BigAdd(a, b)
	return BigIntToString(c), nil
}

func StringSub(s string, s2 string) (string, ErrorI) {
	a, err := StringToBigInt(s)
	if err != nil {
		return "", err
	}
	b, err := StringToBigInt(s2)
	if err != nil {
		return "", err
	}
	c := BigSub(a, b)
	return BigIntToString(c), nil
}

func StringAdd(s string, s2 string) (string, ErrorI) {
	a, err := StringToBigInt(s)
	if err != nil {
		return "", err
	}
	b, err := StringToBigInt(s2)
	if err != nil {
		return "", err
	}
	c := BigAdd(a, b)
	return BigIntToString(c), nil
}

func StringDiv(s, s2 string) (string, ErrorI) {
	a, err := StringToBigInt(s)
	if err != nil {
		return "", err
	}
	b, err := StringToBigInt(s2)
	if err != nil {
		return "", err
	}
	c := BigDiv(a, b)
	return BigIntToString(c), nil
}

func StringPercentDiv(s, s2 string) (int, ErrorI) {
	a, err := StringToBigFloat(s)
	if err != nil {
		return 0, err
	}
	b, err := StringToBigFloat(s2)
	if err != nil {
		return 0, err
	}
	c := new(big.Float).Quo(a, b)
	f, _ := c.Float64()
	return int(f * 100), nil
}

func StringToBigInt(s string) (*big.Int, ErrorI) {
	b := big.Int{}
	i, ok := b.SetString(s, 10)
	if !ok {
		return nil, errStringToBigInt()
	}
	return i, nil
}

func StringToBigFloat(s string) (*big.Float, ErrorI) {
	b := big.Float{}
	i, ok := b.SetString(s)
	if !ok {
		return nil, errStringToBigFloat()
	}
	return i, nil
}

func StringToUint64(s string) uint64 {
	i, _ := strconv.ParseUint(s, 10, 64)
	return i
}

func BigIntToString(b *big.Int) string {
	return b.Text(10)
}
