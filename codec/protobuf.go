package codec

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

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
