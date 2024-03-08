package types

import "google.golang.org/protobuf/proto"

type MessageI interface {
	proto.Message

	SetSigner(signer []byte)
	Check() ErrorI
	Bytes() ([]byte, ErrorI)
	Name() string
	Recipient() string // for transaction indexing by recipient
}
