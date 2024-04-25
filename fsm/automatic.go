package fsm

import (
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
)

func (s *StateMachine) BeginBlock() lib.ErrorI {
	if err := s.CheckProtocolVersion(); err != nil {
		return err
	}
	nonSignerPercent, err := s.HandleByzantine(s.BeginBlockParams)
	if err != nil {
		return err
	}
	if err = s.RewardProposer(crypto.NewAddressFromBytes(s.BeginBlockParams.BlockHeader.ProposerAddress), nonSignerPercent); err != nil {
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

func (s *StateMachine) GetConsensusValidators() (*lib.ConsensusValidators, lib.ErrorI) {
	set := new(lib.ConsensusValidators)
	params, err := s.GetParamsVal()
	if err != nil {
		return nil, err
	}
	it, err := s.RevIterator(types.ConsensusPrefix())
	if err != nil {
		return nil, err
	}
	defer it.Close()
	for i := uint64(0); it.Valid() && i < params.ValidatorMaxCount; it.Next() {
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
		set.ValidatorSet = append(set.ValidatorSet, &lib.ConsensusValidator{
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
	if s.Height() >= version.Height && uint64(s.ProtocolVersion) < version.Version {
		return types.ErrInvalidProtocolVersion()
	}
	return nil
}

func (s *StateMachine) UpdateProducerKeys(publicKey []byte) lib.ErrorI {
	keys, err := s.GetProducerKeys()
	if err != nil {
		return err
	}
	if keys == nil || len(keys.ProducerKeys) == 0 {
		keys = new(lib.ProducerKeys)
		keys.ProducerKeys = make([][]byte, 5)
	}
	index := s.Height() % 5
	keys.ProducerKeys[index] = publicKey
	return s.SetProducerKeys(keys)
}

func (s *StateMachine) GetProducerKeys() (*lib.ProducerKeys, lib.ErrorI) {
	bz, err := s.Get(types.ProducersPrefix())
	if err != nil {
		return nil, err
	}
	ptr := new(lib.ProducerKeys)
	if err = lib.Unmarshal(bz, ptr); err != nil {
		return nil, err
	}
	return ptr, nil
}

func (s *StateMachine) SetProducerKeys(keys *lib.ProducerKeys) lib.ErrorI {
	bz, err := lib.Marshal(keys)
	if err != nil {
		return err
	}
	return s.Set(types.ProducersPrefix(), bz)
}

func (s *StateMachine) RewardProposer(address crypto.AddressI, nonSignerPercent int) lib.ErrorI {
	validator, err := s.GetValidator(address)
	if err != nil {
		return err
	}
	valParams, err := s.GetParamsVal()
	if err != nil {
		return err
	}
	govParams, err := s.GetParamsGov()
	if err != nil {
		return err
	}
	if err = s.UpdateProducerKeys(validator.PublicKey); err != nil {
		return err
	}
	fee, err := s.GetPool(types.PoolID_FeeCollector)
	if err != nil {
		return err
	}
	totalReward := fee.Amount + valParams.ValidatorBlockReward
	afterDAOCut := lib.Uint64ReducePercentage(totalReward, int8(govParams.DaoRewardPercentage))
	daoCut := totalReward - afterDAOCut
	proposerCut := lib.Uint64ReducePercentage(afterDAOCut, int8(nonSignerPercent))
	if err = s.MintToPool(types.PoolID_DAO, daoCut); err != nil {
		return err
	}
	return s.MintToAccount(crypto.NewAddressFromBytes(validator.Output), proposerCut)
}

func (s *StateMachine) HandleByzantine(beginBlock *lib.BeginBlockParams) (nonSignerPercent int, err lib.ErrorI) {
	block := beginBlock.BlockHeader
	params, err := s.GetParamsVal()
	if err != nil {
		return 0, err
	}
	if s.Height()%params.ValidatorNonSignWindow == 0 {
		if err = s.SlashAndResetNonSigners(params); err != nil {
			return 0, err
		}
	}
	if err = s.SlashBadProposers(params, block.BadProposers); err != nil {
		return 0, err
	}
	for _, evidence := range beginBlock.BlockHeader.Evidence {
		if err = s.SlashDoubleSigners(params, evidence.DoubleSigners); err != nil {
			return 0, err
		}
	}
	nonSigners, nonSignerPercent, err := block.LastQuorumCertificate.GetNonSigners(beginBlock.ValidatorSet)
	if err != nil {
		return 0, err
	}
	if err = s.IncrementNonSigners(nonSigners); err != nil {
		return 0, err
	}
	return
}
