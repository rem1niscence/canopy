package codec

import (
	"encoding/json"
)

type Marshaler interface {
	BinaryCodec
	JSONCodec
}

type BinaryCodec interface {
	Marshal(message any) ([]byte, error)
	Unmarshal(data []byte, ptr any) error
}

type JSONCodec interface {
	json.Marshaler
	json.Unmarshaler
}
