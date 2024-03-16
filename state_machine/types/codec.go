package types

import (
	"encoding/binary"
	"github.com/ginchuco/ginchu/codec"
	lib "github.com/ginchuco/ginchu/types"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

var cdc = codec.Protobuf{}

func Marshal(protoMsg proto.Message) ([]byte, lib.ErrorI) {
	bz, err := cdc.Marshal(protoMsg)
	if err != nil {
		return nil, ErrMarshal(err)
	}
	return bz, nil
}

func Unmarshal(bz []byte, ptr any) lib.ErrorI {
	if err := cdc.Unmarshal(bz, ptr); err != nil {
		return ErrUnmarshal(err)
	}
	return nil
}

func FromAny(a *anypb.Any) (proto.Message, lib.ErrorI) {
	msg, err := cdc.FromAny(a)
	if err != nil {
		return nil, ErrFromAny(err)
	}
	return msg, nil
}

func ProtoEnumToBytes(i uint32) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(i))
	return b
}
