package crypto

import (
	ed25519 "crypto/ed25519"
	"encoding/hex"
	"encoding/json"
)

const (
	Ed25519PrivKeySize   = ed25519.PrivateKeySize
	Ed25519PubKeySize    = ed25519.PublicKeySize
	Ed25519SignatureSize = ed25519.SignatureSize
)

// Private Key Below

type ED25519PrivateKey struct{ ed25519.PrivateKey }

func NewPrivateKeyED25519(privateKey ed25519.PrivateKey) *ED25519PrivateKey {
	return &ED25519PrivateKey{PrivateKey: privateKey}
}

var _ PrivateKeyI = &ED25519PrivateKey{}

func (p *ED25519PrivateKey) String() string         { return hex.EncodeToString(p.Bytes()) }
func (p *ED25519PrivateKey) Bytes() []byte          { return p.PrivateKey }
func (p *ED25519PrivateKey) Sign(msg []byte) []byte { return ed25519.Sign(p.PrivateKey, msg) }

func (p *ED25519PrivateKey) PublicKey() PublicKeyI {
	return &PublicKeyED25519{p.PrivateKey.Public().(ed25519.PublicKey)}
}

func (p *ED25519PrivateKey) Equals(key PrivateKeyI) bool {
	return p.PrivateKey.Equal(ed25519.PrivateKey(key.Bytes()))
}
func (p *ED25519PrivateKey) MarshalJSON() ([]byte, error) { return json.Marshal(p.String()) }
func (p *ED25519PrivateKey) UnmarshalJSON(b []byte) (err error) {
	var hexString string
	if err = json.Unmarshal(b, &hexString); err != nil {
		return
	}
	bz, err := hex.DecodeString(hexString)
	if err != nil {
		return
	}
	*p = *NewPrivateKeyED25519(bz)
	return
}

// Public Key Below

type PublicKeyED25519 struct{ ed25519.PublicKey }

func NewPublicKeyED25519(publicKey ed25519.PublicKey) *PublicKeyED25519 {
	return &PublicKeyED25519{PublicKey: publicKey}
}

var _ PublicKeyI = &PublicKeyED25519{}

func (p *PublicKeyED25519) Address() AddressI {
	pubHash := Hash(p.Bytes())
	address := Address(pubHash[:AddressSize])
	return &address
}
func (p *PublicKeyED25519) MarshalJSON() ([]byte, error) { return json.Marshal(p.String()) }
func (p *PublicKeyED25519) UnmarshalJSON(b []byte) (err error) {
	var hexString string
	if err = json.Unmarshal(b, &hexString); err != nil {
		return
	}
	bz, err := hex.DecodeString(hexString)
	if err != nil {
		return
	}
	*p = *NewPublicKeyED25519(bz)
	return
}
func (p *PublicKeyED25519) Bytes() []byte  { return p.PublicKey }
func (p *PublicKeyED25519) String() string { return hex.EncodeToString(p.Bytes()) }

func (p *PublicKeyED25519) VerifyBytes(msg []byte, sig []byte) bool {
	return ed25519.Verify(p.PublicKey, msg, sig)
}

func (p *PublicKeyED25519) Equals(i PublicKeyI) bool {
	return p.PublicKey.Equal(ed25519.PublicKey(i.Bytes()))
}
