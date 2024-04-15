package types

import (
	"github.com/ginchuco/ginchu/types/crypto"
	"google.golang.org/protobuf/proto"
	"net"
	"strings"
)

type P2P interface {
	SendToValidators(msg proto.Message) ErrorI
	SendTo(pubKey []byte, topic Topic, msg proto.Message) ErrorI
	SendToPeer(topic Topic, msg proto.Message) (*PeerInfo, ErrorI)
	ReceiveChannel(topic Topic) chan *MessageWrapper
	GetPeerInfo(pubKey []byte) (PeerInfo, ErrorI)
	GetPeerWithHeight(height uint64) *PeerInfo
	GetMaxPeerHeight() uint64
	ChangeReputation(pubKey []byte, delta int32)
}

type Channels map[Topic]chan *MessageWrapper

type MessageWrapper struct {
	Message proto.Message
	Sender  PeerInfo
}

const (
	delimiter = "@"
)

func (x *PeerInfo) AddressString() string {
	return BytesToString(x.PublicKey) + delimiter + x.NetAddress
}

func (x *PeerInfo) FromString(s string) ErrorI {
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
