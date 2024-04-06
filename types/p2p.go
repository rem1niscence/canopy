package types

import "google.golang.org/protobuf/proto"

type P2P interface {
	SendToAll(msg proto.Message) ErrorI
	SendToOne(recipientPublicKey []byte, msg proto.Message) ErrorI
}
