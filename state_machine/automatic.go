package state_machine

import (
	"github.com/ginchuco/ginchu/crypto"
	"github.com/ginchuco/ginchu/state_machine/types"
	lib "github.com/ginchuco/ginchu/types"
)

func (s *StateMachine) BeginBlock(beginBlock *lib.BeginBlockParams) lib.ErrorI {
	if err := s.CheckProtocolVersion(); err != nil {
		return err
	}
	if err := s.RewardProposer(crypto.NewAddressFromBytes(beginBlock.ProposerAddress)); err != nil {
		return err
	}
	if err := s.HandleByzantine(beginBlock); err != nil {
		return err
	}
	return nil
}

func (s *StateMachine) EndBlock() (endBlock *lib.EndBlockParams, err lib.ErrorI) {
	endBlock = new(lib.EndBlockParams)
	if err = s.DeletePaused(s.Height()); err != nil {
		return
	}
	if err = s.DeleteUnstaking(s.Height()); err != nil {
		return
	}
	endBlock.ValidatorSet, err = s.GetConsensusValidators()
	return
}

func (s *StateMachine) GetConsensusValidators() (*lib.ValidatorSet, lib.ErrorI) {
	set := new(lib.ValidatorSet)
	params, err := s.GetParamsVal()
	if err != nil {
		return nil, err
	}
	it, err := s.RevIterator(types.ConsensusPrefix())
	if err != nil {
		return nil, err
	}
	defer it.Close()
	for i := uint64(0); it.Valid() && i < params.ValidatorMaxCount.Value; it.Next() {
		addr, err := types.AddressFromKey(it.Key())
		if err != nil {
			return nil, err
		}
		val, err := s.GetValidator(addr)
		if err != nil {
			return nil, err
		}
		if val.MaxPausedHeight != 0 {
			return nil, types.ErrValidatorPaused()
		}
		if val.UnstakingHeight != 0 {
			return nil, types.ErrValidatorUnstaking()
		}
		set.ValidatorSet = append(set.ValidatorSet, &lib.ValidatorKeyAndPower{
			PublicKey:   val.PublicKey,
			VotingPower: val.StakedAmount,
		})
	}
	return set, nil
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
	amount, err := lib.StringToBigInt(params.ValidatorBlockReward.Value)
	if err != nil {
		return err
	}
	return s.MintToAccount(crypto.NewAddressFromBytes(validator.Output), amount)
}

func (s *StateMachine) HandleByzantine(beginBlock *lib.BeginBlockParams) (err lib.ErrorI) {
	params, err := s.GetParamsVal()
	if err != nil {
		return err
	}
	if s.Height()%params.ValidatorNonSignWindow.Value == 0 {
		if err = s.SlashAndResetNonSigners(params); err != nil {
			return err
		}
	}
	if err = s.SlashBadProposers(params, beginBlock.BadProposers); err != nil {
		return err
	}
	if err = s.SlashFaultySigners(params, beginBlock.FaultySigners); err != nil {
		return err
	}
	if err = s.SlashDoubleSigners(params, beginBlock.DoubleSigners); err != nil {
		return err
	}
	return s.IncrementNonSigners(beginBlock.NonSigners)
}
