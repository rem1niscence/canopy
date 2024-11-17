package codec

import (
	"encoding/json"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// BinaryCodec is an interface model that defines the requirements for binary encoding and decoding
// A binary encoder converts data into a compact, non-human-readable binary format, which is highly
// efficient in terms of both storage size and speed for serialization and deserialization
type BinaryCodec interface {
	Marshal(message any) ([]byte, error)
	Unmarshal(data []byte, ptr any) error
	ToAny(proto.Message) (*anypb.Any, error)
	FromAny(*anypb.Any) (proto.Message, error)
}

// JSONCodec is an interface model that defines requirements for json encoding and decoding
// converts data into a human-readable text format (JSON) which can be more friendly but less performant
type JSONCodec interface {
	json.Marshaler
	json.Unmarshaler
}

// ensure the protobuf codec implements the BinaryCodec interface
var _ BinaryCodec = &Protobuf{}

// Protobuf is an encoding implementation for protobuf
type Protobuf struct{}

// Marshal() converts a message to bytes
func (p *Protobuf) Marshal(message any) ([]byte, error) {
	return proto.Marshal(message.(proto.Message))
}

// Unmarshal() converts bytes to a protobuf structure
func (p *Protobuf) Unmarshal(data []byte, ptr any) error {
	return proto.Unmarshal(data, ptr.(proto.Message))
}

// A Protobuf Any is a special type that allows a protocol buffer message to encapsulate another message generically

// ToAny() packs a protobuf message to a generic any
func (p *Protobuf) ToAny(message proto.Message) (*anypb.Any, error) {
	return anypb.New(message)
}

// FromAny() converts a proto any to the protobuf message
func (p *Protobuf) FromAny(any *anypb.Any) (proto.Message, error) {
	return anypb.UnmarshalNew(any, proto.UnmarshalOptions{})
}
