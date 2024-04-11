package types

import (
	"google.golang.org/protobuf/proto"
)

type P2P interface {
	SendToValidators(msg proto.Message) ErrorI
	SendToOne(pubKey []byte, msg proto.Message) ErrorI
	SendToPeer(msg proto.Message) (PeerInfo, ErrorI)
	ReceiveChannel(topic Topic) chan MessageWrapper
	GetPeerInfo(pubKey []byte) PeerInfo
	GetPeersForHeight(height uint64) []PeerInfo
	GetMaxPeerHeight() uint64
	ChangeReputation(pubKey []byte, delta int32)
}

type MessageWrapper struct {
	Message proto.Message
	Sender  PeerInfo
}
