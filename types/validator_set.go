package types

import (
	"github.com/drand/kyber"
	"github.com/ginchuco/ginchu/crypto"
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
