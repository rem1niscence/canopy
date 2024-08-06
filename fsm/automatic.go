package fsm

import (
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"math"
)

func (s *StateMachine) BeginBlock() lib.ErrorI {
	if s.IsGenesis() {
		return nil
	}
	if err := s.CheckProtocolVersion(); err != nil {
		return err
	}
	params, err := s.GetParamsVal()
	if err != nil {
		return err
	}
	nonSignerPercent, err := s.HandleByzantine(s.BeginBlockParams, params)
	if err != nil {
		return err
	}
	if err = s.RewardProposer(crypto.NewAddressFromBytes(s.BeginBlockParams.BlockHeader.ProposerAddress), nonSignerPercent); err != nil {
		return err
	}
	if err = s.RewardCommittees(params); err != nil {
		return err
	}
	return nil
}

func (s *StateMachine) IsGenesis() bool {
	return s.Height() <= 1 // height 1 will have a nil last qc
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

func (s *StateMachine) GetConsensusValidators(all ...bool) (*lib.ConsensusValidators, lib.ErrorI) {
	set := new(lib.ConsensusValidators)
	params, err := s.GetParamsVal()
	if err != nil {
		return nil, err
	}
	valMaxCount := params.ValidatorMaxCount
	it, err := s.RevIterator(types.ConsensusPrefix())
	if err != nil {
		return nil, err
	}
	defer it.Close()
	if all != nil && all[0] {
		valMaxCount = math.MaxUint64
	}
	for i := uint64(0); it.Valid() && i < valMaxCount; it.Next() {
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
			NetAddress:  val.NetAddress,
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

func (s *StateMachine) UpdateProposerKeys(publicKey []byte) lib.ErrorI {
	keys, err := s.GetProducerKeys()
	if err != nil {
		return err
	}
	if keys == nil || len(keys.ProposerKeys) == 0 {
		keys = new(lib.ProposerKeys)
		keys.ProposerKeys = make([][]byte, 5)
	}
	index := s.Height() % 5
	keys.ProposerKeys[index] = publicKey
	return s.SetProducerKeys(keys)
}

func (s *StateMachine) GetProducerKeys() (*lib.ProposerKeys, lib.ErrorI) {
	bz, err := s.Get(types.ProducersPrefix())
	if err != nil {
		return nil, err
	}
	ptr := new(lib.ProposerKeys)
	if err = lib.Unmarshal(bz, ptr); err != nil {
		return nil, err
	}
	return ptr, nil
}

func (s *StateMachine) SetProducerKeys(keys *lib.ProposerKeys) lib.ErrorI {
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
	if err = s.UpdateProposerKeys(validator.PublicKey); err != nil {
		return err
	}
	fee, err := s.GetPool(types.FEE_Pool_ID)
	if err != nil {
		return err
	}
	totalReward := fee.Amount + valParams.ValidatorBlockReward
	afterDAOCut := lib.Uint64ReducePercentage(totalReward, float64(govParams.DaoRewardPercentage))
	daoCut := totalReward - afterDAOCut
	proposerCut := lib.Uint64ReducePercentage(afterDAOCut, float64(nonSignerPercent))
	if err = s.MintToPool(types.DAO_Pool_ID, daoCut); err != nil {
		return err
	}
	return s.MintToAccount(crypto.NewAddressFromBytes(validator.Output), proposerCut)
}

func (s *StateMachine) RewardCommittees(params *types.ValidatorParams) lib.ErrorI {
	ids, err := s.GetRewardedCommittees()
	if err != nil {
		return err
	}
	// mintPerCommittee = reward / num_committees
	mintPerCommittee := lib.RoundFloatToUint64(float64(params.ValidatorCommitteeReward) / float64(len(ids)))
	for _, id := range ids {
		if err = s.MintToPool(id, mintPerCommittee); err != nil {
			return err
		}
	}
	return nil
}

func (s *StateMachine) DistributeCommitteeReward() lib.ErrorI {
	eq, err := s.GetEquityByCommittee()
	if err != nil {
		return err
	}
	for _, equity := range eq.EquityByCommittee {
		rewardPool, e := s.GetPool(equity.CommitteeId)
		if e != nil {
			return e
		}
		for _, ep := range equity.EquityPoints {
			rewardAmount := float64(rewardPool.Amount) * float64(ep.Points) / float64(equity.NumberOfSamples*10000)
			if err = s.MintToAccount(crypto.NewAddress(ep.Address), uint64(rewardAmount)); err != nil {
				return err
			}
		}
		rewardPool.Amount = 0
		if err = s.SetPool(rewardPool); err != nil {
			return err
		}
	}
	return s.ClearEquityByCommittee()
}

func (s *StateMachine) HandleByzantine(beginBlock *lib.BeginBlockParams, params *types.ValidatorParams) (nonSignerPercent int, err lib.ErrorI) {
	block := beginBlock.BlockHeader
	if s.Height()%params.ValidatorNonSignWindow == 0 {
		if err = s.SlashAndResetNonSigners(params); err != nil {
			return 0, err
		}
	}
	if err = s.SlashBadProposers(params, block.BadProposers); err != nil {
		return 0, err
	}
	if err = s.HandleDoubleSigners(params, block.DoubleSigners); err != nil {
		return 0, err
	}
	if s.height <= 2 {
		return // height 2 would use height 1 as begin_block which uses genesis as lastQC
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
