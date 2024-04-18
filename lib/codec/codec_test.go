package codec

import (
	"testing"
)

var cdc = Protobuf{}

func TestProtobuf(t *testing.T) {
	v := &Test{I: 1}
	bz, err := cdc.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err = cdc.Unmarshal(bz, v); err != nil {
		t.Fatal(err)
	}
}
