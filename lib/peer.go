package lib

import (
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
