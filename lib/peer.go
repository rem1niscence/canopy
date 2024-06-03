package lib

import (
	"encoding/json"
	"github.com/ginchuco/ginchu/lib/crypto"
	"google.golang.org/protobuf/proto"
	"net"
	"strings"
)

type Channels map[Topic]chan *MessageWrapper

type MessageWrapper struct {
	Message proto.Message
	Hash    []byte
	Sender  *PeerInfo
}

const (
	delimiter = "@"
)

func (x *PeerInfo) AddressString() string {
	return x.Address.AddressString()
}

func (x *PeerAddress) AddressString() string {
	return BytesToString(x.PublicKey) + delimiter + x.NetAddress
}

func (x *PeerAddress) FromString(s string) ErrorI {
	arr := strings.Split(s, delimiter)
	if len(arr) != 2 {
		return ErrInvalidNetAddrString(s)
	}
	pubKeyBytes, err := StringToBytes(arr[0])
	if err != nil {
		return err
	}
	if len(pubKeyBytes) != crypto.Ed25519PubKeySize {
		return ErrInvalidNetAddressPubKey(s)
	}
	host, port, er := net.SplitHostPort(arr[1])
	if er != nil {
		return ErrInvalidNetAddressHostAndPort(s)
	}
	x.NetAddress = net.JoinHostPort(host, port)
	x.PublicKey = pubKeyBytes
	return nil
}

func (x *PeerAddress) Copy() *PeerAddress {
	pkCopy := make([]byte, len(x.PublicKey))
	copy(pkCopy, x.PublicKey)
	return &PeerAddress{
		PublicKey:  pkCopy,
		NetAddress: x.NetAddress,
	}
}

func (x *PeerInfo) Copy() *PeerInfo {
	return &PeerInfo{
		Address:     x.Address.Copy(),
		IsOutbound:  x.IsOutbound,
		IsValidator: x.IsValidator,
		IsTrusted:   x.IsTrusted,
		Reputation:  x.Reputation,
	}
}

func (x *PeerInfo) FromString(s string) ErrorI {
	x.Address = new(PeerAddress)
	return x.Address.FromString(s)
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
		IsValidator: x.IsValidator,
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
