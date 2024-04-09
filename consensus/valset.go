package consensus

import (
	"github.com/drand/kyber"
	"github.com/ginchuco/ginchu/crypto"
	lib "github.com/ginchuco/ginchu/types"
)

type ValidatorSet struct {
	validatorSet          *lib.ValidatorSet
	key                   crypto.MultiPublicKeyI
	powerMap              map[string]Validator // public_key -> Validator
	totalPower            string
	minimumPowerForQuorum string // 2f+1
	numValidators         uint64
}

type Validator struct {
	PublicKey   crypto.PublicKeyI
	VotingPower string
	Index       int
}

func NewValidatorSet(validators *lib.ValidatorSet) (vs ValidatorSet, err lib.ErrorI) {
	totalPower, count := "0", uint64(0)
	points, powerMap := make([]kyber.Point, 0), make(map[string]Validator)
	for i, v := range validators.ValidatorSet {
		point, er := crypto.NewBLSPointFromBytes(v.PublicKey)
		if err != nil {
			return ValidatorSet{}, lib.ErrPubKeyFromBytes(er)
		}
		points = append(points, point)
		powerMap[lib.BytesToString(v.PublicKey)] = Validator{
			PublicKey:   crypto.NewBLS12381PublicKey(point),
			VotingPower: v.VotingPower,
			Index:       i,
		}
		totalPower, err = lib.StringAdd(totalPower, v.VotingPower)
		if err != nil {
			return
		}
		count++
	}
	minimumPowerForQuorum, err := lib.StringReducePercentage(totalPower, 33)
	if err != nil {
		return
	}
	mpk, er := crypto.NewMultiBLSFromPoints(points, nil)
	if er != nil {
		return ValidatorSet{}, lib.ErrNewMultiPubKey(er)
	}
	return ValidatorSet{
		validatorSet:          validators,
		key:                   mpk,
		powerMap:              powerMap,
		totalPower:            totalPower,
		minimumPowerForQuorum: minimumPowerForQuorum,
		numValidators:         count,
	}, nil
}

func (vs *ValidatorSet) GetValidator(publicKey []byte) (*Validator, lib.ErrorI) {
	val, found := vs.powerMap[lib.BytesToString(publicKey)]
	if !found {
		return nil, ErrValidatorNotInSet(publicKey)
	}
	return &val, nil
}

func (vs *ValidatorSet) GetValidatorAtIndex(i int) (*Validator, lib.ErrorI) {
	if uint64(i) >= vs.numValidators {
		return nil, ErrInvalidValidatorIndex()
	}
	val := vs.validatorSet.ValidatorSet[i]
	publicKey, err := publicKeyFromBytes(val.PublicKey)
	if err != nil {
		return nil, err
	}
	return &Validator{
		PublicKey:   publicKey,
		VotingPower: val.VotingPower,
		Index:       i,
	}, nil
}
