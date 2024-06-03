package crypto

import (
	"encoding/hex"
	"encoding/json"
	"errors"
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

func NewBLS12381PrivateKey(privateKey kyber.Scalar) *BLS12381PrivateKey {
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

func (b *BLS12381PrivateKey) MarshalJSON() ([]byte, error) { return json.Marshal(b.String()) }
func (b *BLS12381PrivateKey) UnmarshalJSON(bz []byte) (err error) {
	var hexString string
	if err = json.Unmarshal(bz, &hexString); err != nil {
		return
	}
	bz, err = hex.DecodeString(hexString)
	if err != nil {
		return
	}
	pk, err := NewBLSPrivateKeyFromBytes(bz)
	if err != nil {
		return err
	}
	bls, ok := pk.(*BLS12381PrivateKey)
	if !ok {
		return errors.New("invalid bls key")
	}
	*b = *bls
	return
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
func (b *BLS12381PublicKey) MarshalJSON() ([]byte, error) { return json.Marshal(b.String()) }
func (b *BLS12381PublicKey) UnmarshalJSON(bz []byte) (err error) {
	var hexString string
	if err = json.Unmarshal(bz, &hexString); err != nil {
		return
	}
	bz, err = hex.DecodeString(hexString)
	if err != nil {
		return
	}
	pk, err := NewBLSPublicKeyFromBytes(bz)
	if err != nil {
		return err
	}
	bls, ok := pk.(*BLS12381PublicKey)
	if !ok {
		return errors.New("invalid bls key")
	}
	*b = *bls
	return
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
	signatures [][]byte
	mask       *sign.Mask
	scheme     *bdn.Scheme
}

func NewBLSMultiPublicKey(mask *sign.Mask) *BLS12381MultiPublicKey {
	return &BLS12381MultiPublicKey{mask: mask, scheme: newBLSScheme(), signatures: make([][]byte, len(mask.Publics()))}
}

func (b *BLS12381MultiPublicKey) VerifyBytes(msg, sig []byte) bool {
	publicKey, _ := b.scheme.AggregatePublicKeys(b.mask)
	return b.scheme.Verify(publicKey, msg, sig) == nil
}

func (b *BLS12381MultiPublicKey) AggregateSignatures() ([]byte, error) {
	var ordered [][]byte
	for _, signature := range b.signatures {
		if len(signature) != 0 {
			ordered = append(ordered, signature)
		}
	}
	signature, err := b.scheme.AggregateSignatures(ordered, b.mask)
	if err != nil {
		return nil, err
	}
	return signature.MarshalBinary()
}

func (b *BLS12381MultiPublicKey) AddSigner(signature []byte, index int) error {
	b.signatures[index] = signature
	return b.mask.SetBit(index, true)
}

func (b *BLS12381MultiPublicKey) Reset() {
	b.mask, _ = sign.NewMask(newBLSSuite(), b.mask.Publics(), nil)
	b.signatures = make([][]byte, len(b.mask.Publics()))
}

func (b *BLS12381MultiPublicKey) Copy() MultiPublicKeyI {
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

func (b *BLS12381MultiPublicKey) Bitmap() []byte { return b.mask.Mask() }
func (b *BLS12381MultiPublicKey) SignerEnabledAt(i int) (bool, error) {
	if i > len(b.PublicKeys()) || i < 0 {
		return false, errors.New("invalid bitmap index")
	}
	mask := b.Bitmap()
	byteIndex := i / 8
	mm := byte(1) << (i & 7)
	return mask[byteIndex]&mm != 0, nil
}

func (b *BLS12381MultiPublicKey) SetBitmap(bm []byte) error { return b.mask.SetMask(bm) }
func newBLSScheme() *bdn.Scheme                             { return bdn.NewSchemeOnG2(newBLSSuite()) }
func newBLSSuite() pairing.Suite                            { return bls12381.NewBLS12381Suite() }
