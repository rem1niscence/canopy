package lib

import (
	"bytes"
	"github.com/drand/kyber"
	"github.com/ginchuco/ginchu/lib/crypto"
)

type ValidatorSetWrapper struct {
	ValidatorSet  *ValidatorSet
	Key           crypto.MultiPublicKeyI
	PowerMap      map[string]ValidatorWrapper // public_key -> Validator
	TotalPower    string
	MinimumMaj23  string // 2f+1
	NumValidators uint64
}

type ValidatorWrapper struct {
	PublicKey   crypto.PublicKeyI
	VotingPower string
	Index       int
}

func NewValidatorSet(validators *ValidatorSet) (vs ValidatorSetWrapper, err ErrorI) {
	totalPower, count := "0", uint64(0)
	points, powerMap := make([]kyber.Point, 0), make(map[string]ValidatorWrapper)
	for i, v := range validators.ValidatorSet {
		point, er := crypto.NewBLSPointFromBytes(v.PublicKey)
		if err != nil {
			return ValidatorSetWrapper{}, ErrPubKeyFromBytes(er)
		}
		points = append(points, point)
		powerMap[BytesToString(v.PublicKey)] = ValidatorWrapper{
			PublicKey:   crypto.NewBLS12381PublicKey(point),
			VotingPower: v.VotingPower,
			Index:       i,
		}
		totalPower, err = StringAdd(totalPower, v.VotingPower)
		if err != nil {
			return
		}
		count++
	}
	minimumPowerForQuorum, err := StringReducePercentage(totalPower, 33)
	if err != nil {
		return
	}
	mpk, er := crypto.NewMultiBLSFromPoints(points, nil)
	if er != nil {
		return ValidatorSetWrapper{}, ErrNewMultiPubKey(er)
	}
	return ValidatorSetWrapper{
		ValidatorSet:  validators,
		Key:           mpk,
		PowerMap:      powerMap,
		TotalPower:    totalPower,
		MinimumMaj23:  minimumPowerForQuorum,
		NumValidators: count,
	}, nil
}

func (vs *ValidatorSetWrapper) GetValidator(publicKey []byte) (*ValidatorWrapper, ErrorI) {
	val, found := vs.PowerMap[BytesToString(publicKey)]
	if !found {
		return nil, ErrValidatorNotInSet(publicKey)
	}
	return &val, nil
}

func (vs *ValidatorSetWrapper) GetValidatorAtIndex(i int) (*ValidatorWrapper, ErrorI) {
	if uint64(i) >= vs.NumValidators {
		return nil, ErrInvalidValidatorIndex()
	}
	val := vs.ValidatorSet.ValidatorSet[i]
	publicKey, err := PublicKeyFromBytes(val.PublicKey)
	if err != nil {
		return nil, err
	}
	return &ValidatorWrapper{
		PublicKey:   publicKey,
		VotingPower: val.VotingPower,
		Index:       i,
	}, nil
}

func (x *ValidatorSet) Root() ([]byte, ErrorI) {
	if x == nil || len(x.ValidatorSet) == 0 {
		return nil, nil
	}
	var bytes [][]byte
	for _, val := range x.ValidatorSet {
		bz, err := Marshal(val)
		if err != nil {
			return nil, err
		}
		bytes = append(bytes, bz)
	}
	root, _, err := MerkleTree(bytes)
	return root, err
}

func (x *AggregateSignature) Equals(a2 *AggregateSignature) bool {
	if x == nil || a2 == nil {
		return false
	}
	if !bytes.Equal(x.Signature, a2.Signature) {
		return false
	}
	if !bytes.Equal(x.Bitmap, a2.Bitmap) {
		return false
	}
	return true
}

func (x *AggregateSignature) CheckBasic(sb SignByte, vs ValidatorSetWrapper) ErrorI {
	if x == nil {
		return ErrEmptyAggregateSignature()
	}
	if len(x.Signature) != crypto.BLS12381SignatureSize {
		return ErrInvalidAggrSignatureLength()
	}
	if len(x.Bitmap) == 0 {
		return ErrEmptySignerBitmap()
	}
	key := vs.Key.Copy()
	if er := key.SetBitmap(x.Bitmap); er != nil {
		return ErrInvalidSignerBitmap(er)
	}
	msg, err := sb.SignBytes()
	if err != nil {
		return err
	}
	if !key.VerifyBytes(msg, x.Signature) {
		return ErrInvalidAggrSignature()
	}
	return nil
}

func (x *AggregateSignature) Check(sb SignByte, vs ValidatorSetWrapper) (isPartialQC bool, err ErrorI) {
	if err = x.CheckBasic(sb, vs); err != nil {
		return false, err
	}
	// check 2/3 maj
	key := vs.Key.Copy()
	if er := key.SetBitmap(x.Bitmap); er != nil {
		return false, ErrInvalidSignerBitmap(er)
	}
	totalSignedPower := "0"
	for i, val := range vs.ValidatorSet.ValidatorSet {
		signed, er := key.SignerEnabledAt(i)
		if er != nil {
			return false, ErrInvalidSignerBitmap(er)
		}
		if signed {
			totalSignedPower, err = StringAdd(totalSignedPower, val.VotingPower)
			if err != nil {
				return false, err
			}
		}
	}
	hasMaj23, err := StringsGTE(totalSignedPower, vs.MinimumMaj23)
	if err != nil {
		return false, err
	}
	if !hasMaj23 {
		return true, nil
	}
	return false, nil
}

func (x *AggregateSignature) GetDoubleSigners(y *AggregateSignature, vs ValidatorSetWrapper) (doubleSigners [][]byte, err ErrorI) {
	key, key2 := vs.Key.Copy(), vs.Key.Copy()
	if er := key.SetBitmap(x.Bitmap); er != nil {
		return nil, ErrInvalidSignerBitmap(er)
	}
	if er := key2.SetBitmap(y.Bitmap); er != nil {
		return nil, ErrInvalidSignerBitmap(er)
	}
	for i, val := range vs.ValidatorSet.ValidatorSet {
		signed, er := key.SignerEnabledAt(i)
		if er != nil {
			return nil, ErrInvalidSignerBitmap(er)
		}
		if signed {
			signed, er = key2.SignerEnabledAt(i)
			if er != nil {
				return nil, ErrInvalidSignerBitmap(er)
			}
			if signed {
				doubleSigners = append(doubleSigners, val.PublicKey)
			}
		}
	}
	return
}

func (x *AggregateSignature) GetNonSigners(valSet *ValidatorSet) (nonSigners [][]byte, nonSignerPercent int, err ErrorI) {
	vs, err := NewValidatorSet(valSet)
	if err != nil {
		return nil, 0, err
	}
	key := vs.Key.Copy()
	if er := key.SetBitmap(x.Bitmap); er != nil {
		return nil, 0, ErrInvalidSignerBitmap(er)
	}
	nonSignerPower := "0"
	for i, val := range vs.ValidatorSet.ValidatorSet {
		signed, er := key.SignerEnabledAt(i)
		if er != nil {
			return nil, 0, ErrInvalidSignerBitmap(er)
		}
		if !signed {
			nonSigners = append(nonSigners, val.PublicKey)
			nonSignerPower, err = StringAdd(nonSignerPower, val.VotingPower)
			if err != nil {
				return nil, 0, err
			}
		}
	}
	nonSignerPercent, err = StringPercentDiv(nonSignerPower, vs.TotalPower)
	return
}
