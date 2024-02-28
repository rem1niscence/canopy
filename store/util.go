package store

import "github.com/ginchuco/ginchu/codec"

var cdc = codec.Protobuf{}

func PrefixEndBytes(prefix []byte) []byte {
	if len(prefix) == 0 {
		return []byte{byte(255)}
	}
	end := make([]byte, len(prefix))
	copy(end, prefix)
	for {
		if end[len(end)-1] != byte(255) {
			end[len(end)-1]++
			break
		} else {
			end = end[:len(end)-1]
			if len(end) == 0 {
				end = nil
				break
			}
		}
	}
	return end
}
