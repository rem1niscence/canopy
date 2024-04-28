package lib

import (
	"container/list"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"github.com/ginchuco/ginchu/lib/codec"
	"github.com/ginchuco/ginchu/lib/crypto"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"math/big"
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

func BzToTruncStr(b []byte) string {
	if len(b) > 10 {
		return hex.EncodeToString(b[:10])
	}
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
func BigGreater(a *big.Int, b *big.Int) bool { return a.Cmp(b) == 1 }

func Uint64PercentageDiv(dividend, divisor uint64) (res uint64) {
	a := Uint64ToBigFloat(dividend)
	b := Uint64ToBigFloat(divisor)
	b.Quo(a, b)
	b.Mul(b, big.NewFloat(100))
	res, _ = b.Uint64()
	return
}

func Uint64ReducePercentage(amount uint64, percentage int8) (res uint64) {
	a := Uint64ToBigFloat(amount)
	b := Uint64ToBigFloat(100)
	b.Sub(b, big.NewFloat(float64(percentage)))
	b.Quo(b, big.NewFloat(100))
	a.Mul(a, b)
	res, _ = a.Uint64()
	return
}

func Uint64ToBigFloat(u uint64) *big.Float {
	return new(big.Float).SetUint64(u)
}

type HexBytes []byte

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
