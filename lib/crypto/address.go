package crypto

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
)

// Address represents a short version of a public key that pairs to a users secret private key
// Addresses are the most used identity in the blockchain state due to their hash collision resistant property
type Address []byte

// Address must conform to the AddressI interface
var _ AddressI = &Address{}

const (
	// the number of bytes in an address
	AddressSize = 20
)

// MarshalJSON() is the address implementation of json.Marshaller interface
func (a *Address) MarshalJSON() ([]byte, error) { return json.Marshal(a.String()) }

// UnmarshalJSON() is the address implementation of json.Marshaller interface
func (a *Address) UnmarshalJSON(b []byte) (err error) {
	var hexString string
	// decode the bytes to a hex string
	if err = json.Unmarshal(b, &hexString); err != nil {
		return
	}
	// decode the string to bytes
	bz, err := hex.DecodeString(hexString)
	if err != nil {
		return
	}
	// assign the bytes to the address object
	*a = bz
	return
}

func (a *Address) Bytes() []byte          { return (*a)[:] }
func (a *Address) String() string         { return hex.EncodeToString(a.Bytes()) }
func (a *Address) Equals(e AddressI) bool { return bytes.Equal(a.Bytes(), e.Bytes()) }
func (a *Address) Marshal() ([]byte, error) {
	return cdc.Marshal(ProtoAddress{Address: a.Bytes()})
}

// NewAddress() creates a new address object from bytes by assigning bytes to the underlying address object
func NewAddress(b []byte) AddressI {
	a := Address(b)
	return &a
}
