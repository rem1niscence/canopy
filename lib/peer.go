package lib

import (
	"container/list"
	"encoding/json"
	"github.com/ginchuco/ginchu/lib/crypto"
	"google.golang.org/protobuf/proto"
	"net"
	"slices"
	"strings"
	"time"
)

// Channels are logical communication paths or streams that operate over a single 'multiplexed' network connection
type Channels map[Topic]chan *MessageAndMetadata

// MessageAndMetadata is a wrapper over a P2P message with information about the sender and the hash of the message
// for easy de-duplication at the module level
type MessageAndMetadata struct {
	Message proto.Message
	Hash    []byte
	Sender  *PeerInfo
}

// WithHash() fills the hash field with the cryptographic hash of the message (used for de-duplication)
func (x *MessageAndMetadata) WithHash() *MessageAndMetadata {
	x.Hash = nil
	bz, _ := MarshalJSON(x)
	x.Hash = crypto.Hash(bz)
	return x
}

// FromString() creates a new PeerAddress object from string (without meta)
// Peer String example: <some-public-key>@<some-net-address>
func (x *PeerAddress) FromString(s string) ErrorI {
	arr := strings.Split(s, "@")
	if len(arr) != 2 {
		return ErrInvalidNetAddrString(s)
	}
	pubKey, err := crypto.NewPublicKeyFromString(arr[0])
	if err != nil {
		return ErrInvalidNetAddressPubKey(arr[0])
	}
	host, port, er := net.SplitHostPort(arr[1])
	if er != nil {
		return ErrInvalidNetAddressHostAndPort(s)
	}
	x.NetAddress = net.JoinHostPort(host, port)
	x.PublicKey = pubKey.Bytes()
	return nil
}

// peerAddressJSON is the json.Marshaller and json.Unmarshaler representation fo the PeerAddress object
type peerAddressJSON struct {
	PublicKey  HexBytes `json:"public_key,omitempty"`
	NetAddress string   `json:"net_address,omitempty"`
}

// MarshalJSON satisfies the json.Marshaller interface for PeerAddress
func (x *PeerAddress) MarshalJSON() ([]byte, error) {
	return json.Marshal(peerAddressJSON{
		PublicKey:  x.PublicKey,
		NetAddress: x.NetAddress,
	})
}

// UnmarshalJSON satisfies the json.Unmarshlaer interface for PeerAddress
func (x *PeerAddress) UnmarshalJSON(bz []byte) error {
	j := new(peerAddressJSON)
	if err := json.Unmarshal(bz, j); err != nil {
		return err
	}
	x.PublicKey, x.NetAddress = j.PublicKey, j.NetAddress
	return nil
}

// Sign() adds a digital signature to the PeerMeta for remote public key verification
func (x *PeerMeta) Sign(key crypto.PrivateKeyI) *PeerMeta {
	x.Signature = key.Sign(x.SignBytes())
	return x
}

// HasChain() is true if the PeerMeta contains the chainID
func (x *PeerMeta) HasChain(id uint64) bool {
	for _, c := range x.Chains {
		if c == id {
			return true
		}
	}
	return false
}

// SignBytes() returns the canonical byte representation used to digitally sign the bytes
func (x *PeerMeta) SignBytes() []byte {
	sig := x.Signature
	x.Signature = nil
	bz, _ := Marshal(x)
	x.Signature = sig
	return bz
}

// Copy() returns a reference to a clone of the PeerMeta
func (x *PeerMeta) Copy() *PeerMeta {
	if x == nil {
		return nil
	}
	return &PeerMeta{
		NetworkId: x.NetworkId,
		Chains:    slices.Clone(x.Chains),
		Signature: slices.Clone(x.Signature),
	}
}

// HasChain() returns if the PeerAddress's PeerMeta has this chain
func (x *PeerAddress) HasChain(id uint64) bool { return x.PeerMeta.HasChain(id) }

// Copy() returns a clone of the PeerAddress
func (x *PeerAddress) Copy() *PeerAddress {
	pkCopy := make([]byte, len(x.PublicKey))
	copy(pkCopy, x.PublicKey)
	return &PeerAddress{
		PublicKey:  pkCopy,
		NetAddress: x.NetAddress,
		PeerMeta:   x.PeerMeta.Copy(),
	}
}

// HasChain() returns if the PeerInfo has a chain under the PeerAddresses' PeerMeta
func (x *PeerInfo) HasChain(id uint64) bool { return x.Address.HasChain(id) }

// Copy() returns a reference to a clone of the PeerInfo
func (x *PeerInfo) Copy() *PeerInfo {
	return &PeerInfo{
		Address:       x.Address.Copy(),
		IsOutbound:    x.IsOutbound,
		IsMustConnect: x.IsMustConnect,
		IsTrusted:     x.IsTrusted,
		Reputation:    x.Reputation,
	}
}

// MarshalJSON satisfies the json.Marshaller interface for PeerInfo
func (x *PeerInfo) MarshalJSON() ([]byte, error) {
	return json.Marshal(peerInfoJSON{
		Address:     x.Address,
		IsOutbound:  x.IsOutbound,
		IsValidator: x.IsMustConnect,
		IsTrusted:   x.IsTrusted,
		Reputation:  x.Reputation,
	})
}

// peerInfoJSON is the json marshaller and unmarshaler representation of PeerInfo
type peerInfoJSON struct {
	Address     *PeerAddress `json:"Address"`
	IsOutbound  bool         `json:"is_outbound"`
	IsValidator bool         `json:"is_validator"`
	IsTrusted   bool         `json:"is_trusted"`
	Reputation  int32        `json:"reputation"`
}

// MESSAGE LIMITERS BELOW

// SimpleLimiter ensures the number of requests don't exceed
// a total limit and a limit per requester during a timeframe
type SimpleLimiter struct {
	requests        map[string]int
	totalRequests   int
	maxPerRequester int
	maxRequests     int
	reset           *time.Ticker
}

// NewLimiter() returns a new instance of SimpleLimiter with
// - max requests per requester
// - max total requests
// - how often to reset the limiter
func NewLimiter(maxPerRequester, maxRequests, resetWindowS int) *SimpleLimiter {
	return &SimpleLimiter{
		requests:        map[string]int{},
		maxPerRequester: maxPerRequester,
		maxRequests:     maxRequests,
		reset:           time.NewTicker(time.Duration(resetWindowS) * time.Second),
	}
}

// NewRequest() processes a new request and checks if the requester or total requests should be blocked
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

// Reset() clears the requests and resets the total request count
func (l *SimpleLimiter) Reset() {
	l.requests = map[string]int{}
	l.totalRequests = 0
}

// C() returns the channel that signals when the limiter may be reset
func (l *SimpleLimiter) C() <-chan time.Time {
	return l.reset.C
}

// MaxMessageCacheSize returns the max number of messages in the cache queue
const MaxMessageCacheSize = 10000

type MessageCache struct {
	queue *list.List
	m     map[string]struct{}
}

// NewMessageCache() initializes and returns a new MessageCache instance
func NewMessageCache() *MessageCache {
	return &MessageCache{
		queue: list.New(),
		m:     map[string]struct{}{},
	}
}

// Add inserts a new message into the cache if it doesn't already exist
// It removes the oldest message if the cache is full
func (c *MessageCache) Add(msg *MessageAndMetadata) (ok bool) {
	k := BytesToString(msg.Hash)
	if _, found := c.m[k]; found {
		return false
	}
	if c.queue.Len() >= MaxMessageCacheSize {
		e := c.queue.Front()
		message := e.Value.(*MessageAndMetadata)
		delete(c.m, BytesToString(message.Hash))
		c.queue.Remove(e)
	}
	c.m[k] = struct{}{}
	c.queue.PushFront(msg)
	return true
}
