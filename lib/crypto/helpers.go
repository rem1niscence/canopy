package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/drand/kyber"
	"github.com/drand/kyber/sign"
	"github.com/drand/kyber/util/random"
	"os"
)

type KeyGroup struct {
	Address    AddressI
	PublicKey  PublicKeyI
	PrivateKey PrivateKeyI
}

type keyGroupJson struct {
	Address    string `json:"address"`
	PublicKey  string `json:"publicKey"`
	PrivateKey string `json:"privateKey"`
}

func (k *KeyGroup) UnmarshalJSON(b []byte) error {
	j := new(keyGroupJson)
	if err := json.Unmarshal(b, j); err != nil {
		return err
	}
	address, err := NewAddressFromString(j.Address)
	if err != nil {
		return err
	}
	publicKey, err := NewPublicKeyFromString(j.PublicKey)
	if err != nil {
		return err
	}
	privateKey, err := NewPrivateKeyFromString(j.PrivateKey)
	if err != nil {
		return err
	}
	*k = KeyGroup{
		Address:    address,
		PublicKey:  publicKey,
		PrivateKey: privateKey,
	}
	return nil
}

func NewKeyGroup(pk PrivateKeyI) *KeyGroup {
	pub := pk.PublicKey()
	return &KeyGroup{
		Address:    pub.Address(),
		PublicKey:  pub,
		PrivateKey: pk,
	}
}

func NewEd25519PrivateKey() (PrivateKeyI, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return NewPrivateKeyED25519(priv), nil
}

func NewED25519PrivateKeyFromBytes(bz []byte) PrivateKeyI {
	return NewPrivateKeyED25519(bz)
}

func NewED25519PrivateKeyFromString(hexString string) (PrivateKeyI, error) {
	bz, err := hex.DecodeString(hexString)
	if err != nil {
		return nil, err
	}
	return NewPrivateKeyED25519(bz), nil
}

func NewED25519PublicKey() (PublicKeyI, error) {
	pk, err := NewEd25519PrivateKey()
	if err != nil {
		return nil, err
	}
	return pk.PublicKey(), nil
}

func NewED25519PubKeyFromBytes(bz []byte) PublicKeyI {
	return NewPublicKeyED25519(bz)
}

func NewPublicKeyFromString(s string) (PublicKeyI, error) {
	bz, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}
	return NewPublicKeyFromBytes(bz)
}

func NewPublicKeyFromBytes(bz []byte) (PublicKeyI, error) {
	if len(bz) == Ed25519PubKeySize {
		return NewED25519PubKeyFromBytes(bz), nil
	} else {
		return NewBLSPublicKeyFromBytes(bz)
	}
}

func NewED25519PublicKeyFromString(hexString string) (PublicKeyI, error) {
	bz, err := hex.DecodeString(hexString)
	if err != nil {
		return nil, err
	}
	return NewPublicKeyED25519(bz), nil
}

func NewED25519AddressFromString() (AddressI, error) {
	pk, err := NewED25519PublicKey()
	if err != nil {
		return nil, err
	}
	return pk.Address(), nil
}

func NewAddressFromBytes(bz []byte) AddressI {
	if bz == nil {
		return nil
	}
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

func NewBLSPrivateKey() (PrivateKeyI, error) {
	privateKey, _ := newBLSScheme().NewKeyPair(random.New())
	return NewBLS12381PrivateKey(privateKey), nil
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

func NewBLSPublicKey() (PublicKeyI, error) {
	pk, err := NewBLSPrivateKey()
	if err != nil {
		return nil, err
	}
	return pk.PublicKey(), nil
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

func NewMultiBLSFromPoints(publicKeys []kyber.Point, bitmap []byte) (MultiPublicKeyI, error) {
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

func NewMultiBLS(publicKeys [][]byte, bitmap []byte) (MultiPublicKeyI, error) {
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

func NewBLSPrivateKeyFromFile(filepath string) (PrivateKeyI, error) {
	jsonBytes, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	ptr := new(BLS12381PrivateKey)
	if err = json.Unmarshal(jsonBytes, ptr); err != nil {
		return nil, err
	}
	return ptr, nil
}

func PrivateKeyToFile(key PrivateKeyI, filepath string) error {
	bz, err := json.MarshalIndent(key, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath, bz, 0777)
}

func NewED25519PrivateKeyFromFile(filepath string) (PrivateKeyI, error) {
	jsonBytes, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	ptr := new(ED25519PrivateKey)
	if err = json.Unmarshal(jsonBytes, ptr); err != nil {
		return nil, err
	}
	return ptr, nil
}

func NewPrivateKeyFromString(s string) (PrivateKeyI, error) {
	bz, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}
	return NewPrivateKeyFromBytes(bz)
}

func NewPrivateKeyFromBytes(bz []byte) (PrivateKeyI, error) {
	if len(bz) == BLS12381PrivKeySize {
		return NewBLSPrivateKeyFromBytes(bz)
	} else if len(bz) == Ed25519PrivKeySize {
		return NewED25519PrivateKeyFromBytes(bz), nil
	} else {
		return nil, fmt.Errorf("unknown private key size: %d", len(bz))
	}
}
