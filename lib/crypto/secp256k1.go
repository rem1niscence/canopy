package crypto

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	ethCrypto "github.com/ethereum/go-ethereum/crypto"
	_ "github.com/ethereum/go-ethereum/crypto/secp256k1"
	"golang.org/x/crypto/ripemd160"
)

const (
	Secp256K1PrivKeySize   = 32
	Secp256K1PubKeySize    = 33
	Secp256K1SignatureSize = 64
)

var _ PrivateKeyI = &Secp256K1PrivateKey{}

func NewPrivateKeySecp256K1(b []byte) (*Secp256K1PrivateKey, error) {
	pk, err := ethCrypto.ToECDSA(b)
	if err != nil {
		return nil, err
	}
	return &Secp256K1PrivateKey{PrivateKey: pk}, nil
}

// Private Key Below

type Secp256K1PrivateKey struct {
	*ecdsa.PrivateKey
}

func (s *Secp256K1PrivateKey) Sign(msg []byte) []byte {
	sig, _ := ethCrypto.Sign(Hash(msg), s.PrivateKey)
	// a 1-byte value used to indicate the Ethereum 'recovery byte' is omitted
	return sig[:len(sig)-1]
}

func (s *Secp256K1PrivateKey) PublicKey() PublicKeyI {
	return &Secp256K1PublicKey{PublicKey: &s.PrivateKey.PublicKey}
}

func (s *Secp256K1PrivateKey) Bytes() []byte             { return ethCrypto.FromECDSA(s.PrivateKey) }
func (s *Secp256K1PrivateKey) String() string            { return hex.EncodeToString(s.Bytes()) }
func (s *Secp256K1PrivateKey) Equals(i PrivateKeyI) bool { return bytes.Equal(s.Bytes(), i.Bytes()) }

var _ PublicKeyI = &Secp256K1PublicKey{}

func NewPublicKeySecp256K1(b []byte) (*Secp256K1PublicKey, error) {
	pub, err := ethCrypto.DecompressPubkey(b)
	if err != nil {
		return nil, err
	}
	return &Secp256K1PublicKey{PublicKey: pub}, nil
}

type Secp256K1PublicKey struct {
	compressed []byte
	*ecdsa.PublicKey
}

// Address format varies between chains:
// - Cosmos, Harmony, Binance, Avalanche RIPEMD-160(SHA-256(pubkey)) <Tendermint>
// - BTC, BCH, BSV, (and other forks) <1 byte version> + RIPEMD-160(SHA-256(pubkey)) + <4 byte Checksum>
// `RIPEMD-160(SHA-256(pubkey))` seems to be the most common theme in addressing for Secp256K1 public keys
func (s *Secp256K1PublicKey) Address() AddressI {
	hasher := ripemd160.New()
	hasher.Write(Hash(s.Bytes()))
	address := Address(hasher.Sum(nil))
	return &address
}

func (s *Secp256K1PublicKey) VerifyBytes(msg []byte, sig []byte) bool {
	return ethCrypto.VerifySignature(s.Bytes(), Hash(msg), sig)
}
func (s *Secp256K1PublicKey) Bytes() []byte {
	if s.compressed == nil {
		s.compressed = ethCrypto.CompressPubkey(s.PublicKey)
	}
	return s.compressed
}
func (s *Secp256K1PublicKey) String() string           { return hex.EncodeToString(s.Bytes()) }
func (s *Secp256K1PublicKey) Equals(i PublicKeyI) bool { return bytes.Equal(s.Bytes(), i.Bytes()) }
