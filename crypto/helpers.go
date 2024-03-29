package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"github.com/drand/kyber"
	"github.com/drand/kyber/sign"
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

func NewBLSPrivateKeyFromString(hexString string) (PrivateKeyI, error) {
	bz, err := hex.DecodeString(hexString)
	if err != nil {
		return nil, err
	}
	return NewBLSPrivateKeyFromBytes(bz)
}

func NewBLSPrivateKeyFromBytes(bz []byte) (PrivateKeyI, error) {
	keyCopy := newBLSSuite().G2().Scalar()
	if err := keyCopy.UnmarshalBinary(bz); err != nil {
		return nil, err
	}
	return &BLS12381PrivateKey{
		Scalar: keyCopy,
		scheme: newBLSScheme(),
	}, nil
}

func NewBLSPublicKeyFromString(hexString string) (PublicKeyI, error) {
	bz, err := hex.DecodeString(hexString)
	if err != nil {
		return nil, err
	}
	return NewBLSPublicKeyFromBytes(bz)
}

func NewBLSPublicKeyFromBytes(bz []byte) (PublicKeyI, error) {
	point, err := NewBLSPointFromBytes(bz)
	if err != nil {
		return nil, err
	}
	return &BLS12381PublicKey{
		Point:  point,
		scheme: newBLSScheme(),
	}, nil
}

func NewBLSPointFromBytes(bz []byte) (kyber.Point, error) {
	point := newBLSSuite().G1().Point()
	if err := point.UnmarshalBinary(bz); err != nil {
		return nil, err
	}
	return point, nil
}

func NewMultiBLSFromPoints(publicKeys []kyber.Point, bitmap []byte) (MultiPublicKey, error) {
	mask, err := sign.NewMask(newBLSSuite(), publicKeys, nil)
	if err != nil {
		return nil, err
	}
	if bitmap != nil {
		if err = mask.SetMask(bitmap); err != nil {
			return nil, err
		}
	}
	return NewBLSMultiPublicKey(mask), nil
}

func NewMultiBLS(publicKeys [][]byte, bitmap []byte) (MultiPublicKey, error) {
	var points []kyber.Point
	for _, bz := range publicKeys {
		point, err := NewBLSPointFromBytes(bz)
		if err != nil {
			return nil, err
		}
		points = append(points, point)
	}
	return NewMultiBLSFromPoints(points, bitmap)
}
