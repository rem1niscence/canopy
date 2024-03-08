package types

import (
	"github.com/ginchuco/ginchu/codec"
	lib "github.com/ginchuco/ginchu/types"
	"github.com/gogo/protobuf/proto"
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
