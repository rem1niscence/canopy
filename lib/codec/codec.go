package codec

import (
	"encoding/json"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

type Marshaler interface {
	BinaryCodec
	JSONCodec
}

type BinaryCodec interface {
	Marshal(message any) ([]byte, error)
	Unmarshal(data []byte, ptr any) error
	ToAny(proto.Message) (*anypb.Any, error)
	FromAny(*anypb.Any) (proto.Message, error)
}

type JSONCodec interface {
	json.Marshaler
	json.Unmarshaler
}

var _ BinaryCodec = &Protobuf{}

type Protobuf struct{}

func (p *Protobuf) Marshal(message any) ([]byte, error) {
	return proto.Marshal(message.(proto.Message))
}

func (p *Protobuf) Unmarshal(data []byte, ptr any) error {
	return proto.Unmarshal(data, ptr.(proto.Message))
}

func (p *Protobuf) ToAny(message proto.Message) (*anypb.Any, error) {
	return anypb.New(message)
}

func (p *Protobuf) FromAny(any *anypb.Any) (proto.Message, error) {
	return anypb.UnmarshalNew(any, proto.UnmarshalOptions{})
}
