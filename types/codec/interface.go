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
