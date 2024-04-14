package types

import (
	"encoding/binary"
	"encoding/hex"
	"github.com/ginchuco/ginchu/types/codec"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

var (
	cdc = codec.Protobuf{}
)

func Marshal(message any) ([]byte, ErrorI) {
	bz, err := proto.Marshal(message.(proto.Message))
	if err != nil {
		return nil, ErrMarshal(err)
	}
	return bz, nil
}

func MustMarshal(message any) []byte {
	bz, err := proto.Marshal(message.(proto.Message))
	panic(err)
	return bz
}

func Unmarshal(data []byte, ptr any) ErrorI {
	if err := proto.Unmarshal(data, ptr.(proto.Message)); err != nil {
		return ErrUnmarshal(err)
	}
	return nil
}

func ToAny(message proto.Message) (*anypb.Any, ErrorI) {
	a, err := anypb.New(message)
	if err != nil {
		return nil, ErrToAny(err)
	}
	return a, nil
}

func FromAny(any *anypb.Any) (proto.Message, ErrorI) {
	msg, err := anypb.UnmarshalNew(any, proto.UnmarshalOptions{})
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

func BytesToString(b []byte) string {
	return hex.EncodeToString(b)
}

func StringToBytes(s string) ([]byte, ErrorI) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, ErrStringToBytes(err)
	}
	return b, nil
}
