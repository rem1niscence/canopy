package crypto

import (
	"encoding/hex"
	"github.com/drand/kyber"
	bls12381 "github.com/drand/kyber-bls12381"
	"github.com/drand/kyber/pairing"
	"github.com/drand/kyber/sign"
	"github.com/drand/kyber/sign/bdn"
)

const (
	BLS12381PrivKeySize   = 32
	BLS12381PubKeySize    = 48
	BLS12381SignatureSize = 96
)

var _ PrivateKeyI = &BLS12381PrivateKey{}

type BLS12381PrivateKey struct {
	kyber.Scalar
	scheme *bdn.Scheme
}

func NewBLS12381PrivateKey(privateKey kyber.Scalar, publicKey kyber.Point) *BLS12381PrivateKey {
	return &BLS12381PrivateKey{Scalar: privateKey, scheme: newBLSScheme()}
}

func (b *BLS12381PrivateKey) Bytes() []byte {
	bz, _ := b.MarshalBinary()
	return bz
}

func (b *BLS12381PrivateKey) Sign(msg []byte) []byte {
	bz, _ := b.scheme.Sign(b.Scalar, msg)
	return bz
}

func (b *BLS12381PrivateKey) PublicKey() PublicKeyI {
	suite := newBLSSuite()
	public := suite.G1().Point().Mul(b.Scalar, suite.G1().Point().Base())
	return NewBLS12381PublicKey(public)
}

func (b *BLS12381PrivateKey) Equals(i PrivateKeyI) bool {
	private, ok := i.(*BLS12381PrivateKey)
	if !ok {
		return false
	}
	return b.Equal(private.Scalar)
}

func (b *BLS12381PrivateKey) String() string {
	return hex.EncodeToString(b.Bytes())
}

type BLS12381PublicKey struct {
	kyber.Point
	scheme *bdn.Scheme
}

func NewBLS12381PublicKey(publicKey kyber.Point) *BLS12381PublicKey {
	return &BLS12381PublicKey{Point: publicKey, scheme: newBLSScheme()}
}

func (b *BLS12381PublicKey) Address() AddressI {
	pubHash := Hash(b.Bytes())
	address := Address(pubHash[:AddressSize])
	return &address
}

func (b *BLS12381PublicKey) Bytes() []byte {
	bz, _ := b.MarshalBinary()
	return bz
}

func (b *BLS12381PublicKey) VerifyBytes(msg []byte, sig []byte) bool {
	return b.scheme.Verify(b.Point, msg, sig) == nil
}

func (b *BLS12381PublicKey) Equals(i PublicKeyI) bool {
	pub2, ok := i.(*BLS12381PublicKey)
	if !ok {
		return false
	}
	return b.Equal(pub2.Point)
}

func (b *BLS12381PublicKey) String() string {
	return hex.EncodeToString(b.Bytes())
}

type BLS12381MultiPublicKey struct {
	mask   *sign.Mask
	scheme *bdn.Scheme
}

func NewBLSMultiPublicKey(mask *sign.Mask) *BLS12381MultiPublicKey {
	return &BLS12381MultiPublicKey{mask: mask, scheme: newBLSScheme()}
}

func (b *BLS12381MultiPublicKey) VerifyBytes(msg, sig []byte) bool {
	publicKey, _ := b.scheme.AggregatePublicKeys(b.mask)
	return b.scheme.Verify(publicKey, msg, sig) == nil
}

func (b *BLS12381MultiPublicKey) AggregateSignatures(ordered [][]byte) ([]byte, error) {
	signature, err := b.scheme.AggregateSignatures(ordered, b.mask)
	if err != nil {
		return nil, err
	}
	return signature.MarshalBinary()
}

func (b *BLS12381MultiPublicKey) AddSigner(index int) error {
	return b.mask.SetBit(index, true)
}

func (b *BLS12381MultiPublicKey) Reset() {
	b.mask, _ = sign.NewMask(newBLSSuite(), b.mask.Publics(), nil)
}

func (b *BLS12381MultiPublicKey) Copy() MultiPublicKey {
	p := b.mask.Publics()
	pCopy := make([]kyber.Point, len(p))
	copy(pCopy, p)
	m := b.mask.Mask()
	mCopy := make([]byte, len(m))
	copy(mCopy, m)
	k, _ := NewMultiBLSFromPoints(pCopy, mCopy)
	return k
}

func (b *BLS12381MultiPublicKey) PublicKeys() (keys []PublicKeyI) {
	for _, key := range b.mask.Publics() {
		keys = append(keys, NewBLS12381PublicKey(key))
	}
	return
}

func (b *BLS12381MultiPublicKey) Bitmap() []byte            { return b.mask.Mask() }
func (b *BLS12381MultiPublicKey) SetBitmap(bm []byte) error { return b.mask.SetMask(bm) }
func newBLSScheme() *bdn.Scheme                             { return bdn.NewSchemeOnG2(newBLSSuite()) }
func newBLSSuite() pairing.Suite                            { return bls12381.NewBLS12381Suite() }
