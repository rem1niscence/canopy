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

type PrivateKeyED25519 struct{ ed25519.PrivateKey }

func NewPrivateKeyED25519(privateKey ed25519.PrivateKey) *PrivateKeyED25519 {
	return &PrivateKeyED25519{PrivateKey: privateKey}
}

var _ PrivateKeyI = &PrivateKeyED25519{}

func (p *PrivateKeyED25519) String() string         { return hex.EncodeToString(p.Bytes()) }
func (p *PrivateKeyED25519) Bytes() []byte          { return p.PrivateKey }
func (p *PrivateKeyED25519) Sign(msg []byte) []byte { return ed25519.Sign(p.PrivateKey, msg) }

func (p *PrivateKeyED25519) PublicKey() PublicKeyI {
	return &PublicKeyED25519{p.PrivateKey.Public().(ed25519.PublicKey)}
}

func (p *PrivateKeyED25519) Equals(key PrivateKeyI) bool {
	return p.PrivateKey.Equal(ed25519.PrivateKey(key.Bytes()))
}
func (p *PrivateKeyED25519) MarshalJSON() ([]byte, error) { return json.Marshal(p.String()) }
func (p *PrivateKeyED25519) UnmarshalJSON(b []byte) (err error) {
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
