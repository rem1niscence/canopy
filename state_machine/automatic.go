package state_machine

import (
	"github.com/ginchuco/ginchu/crypto"
	"github.com/ginchuco/ginchu/state_machine/types"
	lib "github.com/ginchuco/ginchu/types"
)

func (s *StateMachine) StartBlock(proposerAddress crypto.AddressI, badProposers, nonSigners, faultySigners, doubleSigners []crypto.AddressI) lib.ErrorI {
	if err := s.CheckProtocolVersion(); err != nil {
		return err
	}
	if err := s.RewardProposer(proposerAddress); err != nil {
		return err
	}
	return nil
}

func (s *StateMachine) CheckProtocolVersion() lib.ErrorI {
	params, err := s.GetParamsCons()
	if err != nil {
		return err
	}
	version, err := params.ParseProtocolVersion()
	if err != nil {
		return err
	}
	if s.Height() >= version.Height && uint64(s.protocolVersion) < version.Version {
		return types.ErrInvalidProtocolVersion()
	}
	return nil
}

func (s *StateMachine) RewardProposer(address crypto.AddressI) lib.ErrorI {
	validator, err := s.GetValidator(address)
	if err != nil {
		return err
	}
	params, err := s.GetParamsVal()
	if err != nil {
		return err
	}
	amount, err := lib.StringToBigInt(params.ValidatorProposerBlockReward.Value)
	if err != nil {
		return err
	}
	return s.MintToAccount(crypto.NewAddressFromBytes(validator.Output), amount)
}

func (s *StateMachine) EndBlock() (*lib.ValidatorSet, lib.ErrorI) {
	return nil, nil // TODO max paused, validator set etc.
}
