package lib

import (
	"encoding/json"
	"github.com/ginchuco/ginchu/lib/crypto"
	"google.golang.org/protobuf/proto"
	"net"
	"slices"
	"strings"
)

type Channels map[Topic]chan *MessageAndMetadata

type MessageAndMetadata struct {
	Message proto.Message
	Hash    []byte
	Sender  *PeerInfo
}

func (x *MessageAndMetadata) WithHash() *MessageAndMetadata {
	x.Hash = nil
	bz, _ := MarshalJSON(x)
	x.Hash = crypto.Hash(bz)
	return x
}

const (
	at = "@"
)

func (x *PeerInfo) AddressString() string {
	return x.Address.AddressString()
}

func (x *PeerAddress) AddressString() string {
	return BytesToString(x.PublicKey) + at + x.NetAddress
}

func (x *PeerAddress) FromString(s string) ErrorI {
	arr := strings.Split(s, at)
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

func (x *PeerMeta) Sign(key crypto.PrivateKeyI) *PeerMeta {
	x.Signature = key.Sign(x.SignBytes())
	return x
}

func (x *PeerMeta) HasChain(id uint64) bool {
	for _, c := range x.Chains {
		if c == id {
			return true
		}
	}
	return false
}

func (x *PeerMeta) SignBytes() []byte {
	sig := x.Signature
	x.Signature = nil
	bz, _ := Marshal(x)
	x.Signature = sig
	return bz
}
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
func (x *PeerAddress) HasChain(id uint64) bool { return x.PeerMeta.HasChain(id) }

func (x *PeerAddress) Copy() *PeerAddress {
	pkCopy := make([]byte, len(x.PublicKey))
	copy(pkCopy, x.PublicKey)
	return &PeerAddress{
		PublicKey:  pkCopy,
		NetAddress: x.NetAddress,
		PeerMeta:   x.PeerMeta.Copy(),
	}
}
func (x *PeerInfo) HasChain(id uint64) bool { return x.Address.HasChain(id) }
func (x *PeerInfo) Copy() *PeerInfo {
	return &PeerInfo{
		Address:       x.Address.Copy(),
		IsOutbound:    x.IsOutbound,
		IsMustConnect: x.IsMustConnect,
		IsTrusted:     x.IsTrusted,
		Reputation:    x.Reputation,
	}
}

type peerInfoJSON struct {
	Address     *PeerAddress `json:"Address"`
	IsOutbound  bool         `json:"is_outbound"`
	IsValidator bool         `json:"is_validator"`
	IsTrusted   bool         `json:"is_trusted"`
	Reputation  int32        `json:"reputation"`
}

func (x *PeerInfo) MarshalJSON() ([]byte, error) {
	return json.Marshal(peerInfoJSON{
		Address:     x.Address,
		IsOutbound:  x.IsOutbound,
		IsValidator: x.IsMustConnect,
		IsTrusted:   x.IsTrusted,
		Reputation:  x.Reputation,
	})
}

type peerAddressJSON struct {
	PublicKey  HexBytes `json:"public_key,omitempty"`
	NetAddress string   `json:"net_address,omitempty"`
}

func (x *PeerAddress) MarshalJSON() ([]byte, error) {
	return json.Marshal(peerAddressJSON{
		PublicKey:  x.PublicKey,
		NetAddress: x.NetAddress,
	})
}

func (x *PeerAddress) UnmarshalJSON(bz []byte) error {
	j := new(peerAddressJSON)
	if err := json.Unmarshal(bz, j); err != nil {
		return err
	}
	x.PublicKey, x.NetAddress = j.PublicKey, j.NetAddress
	return nil
}
