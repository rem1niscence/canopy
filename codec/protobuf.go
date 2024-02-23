package codec

import "google.golang.org/protobuf/proto"

var _ BinaryCodec = &Protobuf{}

type Protobuf struct{}

func (p *Protobuf) Marshal(message any) ([]byte, error) {
	return proto.Marshal(message.(proto.Message))
}

func (p *Protobuf) Unmarshal(data []byte, ptr any) error {
	return proto.Unmarshal(data, ptr.(proto.Message))
}
