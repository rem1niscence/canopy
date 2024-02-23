package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
)

func NewPrivateKey() (PrivateKeyI, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return NewPrivateKeyED25519(priv), nil
}

func NewPrivateKeyFromBytes(bz []byte) PrivateKeyI {
	return NewPrivateKeyED25519(bz)
}

func NewPrivateKeyFromString(hexString string) (PrivateKeyI, error) {
	bz, err := hex.DecodeString(hexString)
	if err != nil {
		return nil, err
	}
	return NewPrivateKeyED25519(bz), nil
}

func NewPublicKey() (PublicKeyI, error) {
	pk, err := NewPrivateKey()
	if err != nil {
		return nil, err
	}
	return pk.PublicKey(), nil
}

func NewPublicKeyFromBytes(bz []byte) PublicKeyI {
	return NewPublicKeyED25519(bz)
}

func NewPublicKeyFromString(hexString string) (PublicKeyI, error) {
	bz, err := hex.DecodeString(hexString)
	if err != nil {
		return nil, err
	}
	return NewPublicKeyED25519(bz), nil
}

func NewAddress() (AddressI, error) {
	pk, err := NewPublicKey()
	if err != nil {
		return nil, err
	}
	return pk.Address(), nil
}

func NewAddressFromBytes(bz []byte) AddressI {
	a := Address(bz)
	return &a
}

func NewAddressFromString(hexString string) (AddressI, error) {
	bz, err := hex.DecodeString(hexString)
	if err != nil {
		return nil, err
	}
	return NewAddressFromBytes(bz), nil
}
