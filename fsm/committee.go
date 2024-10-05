package fsm

import (
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"math"
)

// COMMITTEES BELOW

func (s *StateMachine) GetProtocolSubsidizedCommittees() (paidIDs []uint64, err lib.ErrorI) {
	supply, err := s.GetSupply()
	if err != nil {
		return nil, err
	}
	valParams, err := s.GetParamsVal()
	if err != nil {
		return nil, err
	}
	committees := supply.CommitteesWithDelegations
	totalStakeFromAllCommittees, numCommittees := uint64(0), len(committees)
	for i := 0; i < numCommittees; i++ {
		totalStakeFromAllCommittees += committees[i].Amount
	}
	// since re-staking is enabled in the Canopy protocol, the protocol is able to simply pick which chains are paid via a 'committed stake'
	// percentage. Since the effective cost of 're-staking' is essentially zero (minus the risk associated with slashing on the chain)
	// this may be thought of like a popularity contest or a vote.
	for _, committee := range committees {
		committedStakePercent := lib.Uint64PercentageDiv(committee.Amount, totalStakeFromAllCommittees)
		if committedStakePercent >= valParams.ValidatorMinimumPercentForPaidCommittee {
			paidIDs = append(paidIDs, committee.Id)
		}
	}
	return
}

// FundCommitteeRewardPools() mints newly created tokens to protocol subsidized committees
func (s *StateMachine) FundCommitteeRewardPools() lib.ErrorI {
	// get governance params that are needed to complete this operation
	govParams, err := s.GetParamsGov()
	if err != nil {
		return err
	}
	// get validator params that are needed to complete this operation
	params, err := s.GetParamsVal()
	if err != nil {
		return err
	}
	// get the committees that `qualify` for subsidization
	subsidizedCommitteeIds, err := s.GetProtocolSubsidizedCommittees()
	if err != nil {
		return err
	}
	// the total mint amount is defined by a governance parameter
	totalMintAmount := params.ValidatorCommitteeReward
	// calculate the amount left for the committees after the parameterized DAO cut
	mintAmountAfterDAOCut := lib.Uint64ReducePercentage(totalMintAmount, float64(govParams.DaoRewardPercentage))
	// calculate the DAO cut
	daoCut := totalMintAmount - mintAmountAfterDAOCut
	// mint to the DAO account
	if err = s.MintToPool(lib.DAOPoolID, daoCut); err != nil {
		return err
	}
	// calculate the amount given to each qualifying committee
	mintAmountPerCommittee := mintAmountAfterDAOCut / uint64(len(subsidizedCommitteeIds))
	// issue that amount to each subsidized committee
	for _, committeeId := range subsidizedCommitteeIds {
		if err = s.MintToPool(committeeId, mintAmountPerCommittee); err != nil {
			return err
		}
	}
	return nil
}

// DistributeCommitteeReward() distributes the committee mint based on the PaymentPercents from the result of the QuorumCertificate
func (s *StateMachine) DistributeCommitteeReward() lib.ErrorI {
	committeesData, err := s.GetCommitteesData()
	if err != nil {
		return err
	}
	paramsVal, err := s.GetParamsVal()
	if err != nil {
		return err
	}
	for i, data := range committeesData.List {
		rewardPool, e := s.GetPool(data.CommitteeId)
		if e != nil {
			return e
		}
		for _, ep := range data.PaymentPercents {
			address := crypto.NewAddress(ep.Address)
			fullReward := uint64(float64(rewardPool.Amount) * float64(ep.Percent) / float64(data.NumberOfSamples*100))
			earlyWithdrawalReward := lib.Uint64ReducePercentage(fullReward, float64(paramsVal.ValidatorEarlyWithdrawalPenalty))
			if val, _ := s.GetValidator(address); val != nil {
				if val.Compound {
					if err = s.HandleMessageEditStake(&types.MessageEditStake{
						Address:       val.Address,
						Amount:        val.StakedAmount + fullReward,
						Committees:    val.Committees,
						NetAddress:    val.NetAddress,
						OutputAddress: val.Output,
						Delegate:      val.Delegate,
						Compound:      val.Compound,
					}); err != nil {
						return err
					}
				} else {
					if err = s.MintToAccount(crypto.NewAddress(val.Output), earlyWithdrawalReward); err != nil {
						return err
					}
				}
			} else {
				if err = s.MintToAccount(address, earlyWithdrawalReward); err != nil {
					return err
				}
			}
		}
		rewardPool.Amount = 0
		if err = s.SetPool(rewardPool); err != nil {
			return err
		}
		// clear structure
		committeesData.List[i] = &types.CommitteeData{
			CommitteeId:     data.CommitteeId,
			CommitteeHeight: data.CommitteeHeight,
			ChainHeight:     data.ChainHeight,
		}
	}
	return s.SetCommitteesData(committeesData)
}

func (s *StateMachine) GetCommittee(committeeID uint64, all ...bool) (vs lib.ValidatorSet, err lib.ErrorI) {
	p, err := s.GetParamsVal()
	if err != nil {
		return
	}
	maxSize := p.ValidatorMaxCommitteeSize
	if all != nil && all[0] {
		maxSize = math.MaxUint64
	}
	it, err := s.RevIterator(types.CommitteePrefix(committeeID))
	if err != nil {
		return
	}
	defer it.Close()
	members := make([]*lib.ConsensusValidator, 0)
	for i := uint64(0); it.Valid() && i < maxSize; func() { it.Next(); i++ }() {
		address, e := types.AddressFromKey(it.Key())
		if e != nil {
			return vs, e
		}
		val, e := s.GetValidator(address)
		if e != nil {
			return vs, e
		}
		members = append(members, &lib.ConsensusValidator{
			PublicKey:   val.PublicKey,
			VotingPower: val.StakedAmount,
			NetAddress:  val.NetAddress,
		})
	}
	vs, err = lib.NewValidatorSet(&lib.ConsensusValidators{ValidatorSet: members})
	return
}

// GetCanopyCommittee() returns the committee specifically for the Canopy ID
func (s *StateMachine) GetCanopyCommittee(all ...bool) (*lib.ConsensusValidators, lib.ErrorI) {
	canopyCommittee, err := s.GetCommittee(lib.CanopyCommitteeId, all...)
	if err != nil {
		return nil, err
	}
	return canopyCommittee.ValidatorSet, nil
}

func (s *StateMachine) GetCommitteePaginated(p lib.PageParams, committeeId uint64) (page *lib.Page, err lib.ErrorI) {
	return s.getValidatorsPaginated(p, lib.ValidatorFilters{}, types.CommitteePrefix(committeeId))
}

func (s *StateMachine) UpdateCommittees(address crypto.AddressI, oldValidator *types.Validator, newStakedAmount uint64, newCommittees []uint64) lib.ErrorI {
	if err := s.DeleteCommittees(address, oldValidator.StakedAmount, oldValidator.Committees); err != nil {
		return err
	}
	if oldValidator.MaxPausedHeight != 0 {
		return nil // don't set if paused
	}
	return s.SetCommittees(address, newStakedAmount, newCommittees)
}

func (s *StateMachine) SetCommittees(address crypto.AddressI, totalStake uint64, committees []uint64) lib.ErrorI {
	for _, committee := range committees {
		if err := s.SetCommitteeMember(address, committee, totalStake); err != nil {
			return err
		}
		if err := s.AddToCommitteeStakedSupply(committee, totalStake); err != nil {
			return err
		}
	}
	return nil
}

func (s *StateMachine) DeleteCommittees(address crypto.AddressI, totalStake uint64, committees []uint64) lib.ErrorI {
	for _, committee := range committees {
		if err := s.DeleteCommitteeMember(address, committee, totalStake); err != nil {
			return err
		}
		if err := s.SubFromCommitteeStakedSupply(committee, totalStake); err != nil {
			return err
		}
	}
	return nil
}

func (s *StateMachine) SetCommitteeMember(address crypto.AddressI, committeeID, stakeForCommittee uint64) lib.ErrorI {
	return s.Set(types.KeyForCommittee(committeeID, address, stakeForCommittee), nil)
}

func (s *StateMachine) DeleteCommitteeMember(address crypto.AddressI, committeeID, stakeForCommittee uint64) lib.ErrorI {
	return s.Delete(types.KeyForCommittee(committeeID, address, stakeForCommittee))
}

// DELEGATIONS BELOW

func (s *StateMachine) GetDelegatesPaginated(p lib.PageParams, committeeId uint64) (page *lib.Page, err lib.ErrorI) {
	return s.getValidatorsPaginated(p, lib.ValidatorFilters{}, types.DelegatePrefix(committeeId))
}

func (s *StateMachine) UpdateDelegations(address crypto.AddressI, oldValidator *types.Validator, newStakedAmount uint64, newCommittees []uint64) lib.ErrorI {
	if err := s.DeleteDelegations(address, oldValidator.StakedAmount, oldValidator.Committees); err != nil {
		return err
	}
	return s.SetDelegations(address, newStakedAmount, newCommittees)
}

func (s *StateMachine) SetDelegations(address crypto.AddressI, totalStake uint64, committees []uint64) lib.ErrorI {
	for _, committee := range committees {
		// actually set the address in the delegate list
		if err := s.SetDelegate(address, committee, totalStake); err != nil {
			return err
		}
		// add to the delegate supply (used for tracking amounts)
		if err := s.AddToDelegateStakedSupply(committee, totalStake); err != nil {
			return err
		}
		// add to the committee supply as well (used for tracking amounts)
		if err := s.AddToCommitteeStakedSupply(committee, totalStake); err != nil {
			return err
		}
	}
	return nil
}

func (s *StateMachine) DeleteDelegations(address crypto.AddressI, totalStake uint64, committees []uint64) lib.ErrorI {
	for _, committee := range committees {
		// remove the address from the delegate list
		if err := s.DeleteDelegate(address, committee, totalStake); err != nil {
			return err
		}
		// remove from the delegate supply (used for tracking amounts)
		if err := s.SubFromDelegateStakedSupply(committee, totalStake); err != nil {
			return err
		}
		// remove from the committee supply as well (used for tracking amounts)
		if err := s.SubFromCommitteeStakedSupply(committee, totalStake); err != nil {
			return err
		}
	}
	return nil
}

func (s *StateMachine) SetDelegate(address crypto.AddressI, committeeID, stakeForCommittee uint64) lib.ErrorI {
	return s.Set(types.KeyForDelegate(committeeID, address, stakeForCommittee), nil)
}

func (s *StateMachine) DeleteDelegate(address crypto.AddressI, committeeID, stakeForCommittee uint64) lib.ErrorI {
	return s.Delete(types.KeyForDelegate(committeeID, address, stakeForCommittee))
}

// COMMITTEE DATA CODE BELOW

func (s *StateMachine) UpsertCommitteeData(upsert *types.CommitteeData) lib.ErrorI {
	committeesData, data, idx, err := s.getCommitteeDataAndList(upsert.CommitteeId)
	if upsert.CommitteeHeight <= data.CommitteeHeight && upsert.ChainHeight <= data.ChainHeight {
		return types.ErrInvalidCertificateResults()
	}
	if err = data.Combine(upsert); err != nil {
		return err
	}
	committeesData.List[idx] = data
	return s.SetCommitteesData(committeesData)
}

func (s *StateMachine) GetCommitteeData(committeeID uint64) (*types.CommitteeData, lib.ErrorI) {
	_, f, _, err := s.getCommitteeDataAndList(committeeID)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (s *StateMachine) getCommitteeDataAndList(committeeID uint64) (list *types.CommitteesData, d *types.CommitteeData, idx int, err lib.ErrorI) {
	list, err = s.GetCommitteesData()
	if err != nil {
		return
	}
	for i, data := range list.List {
		if data.CommitteeId == committeeID {
			return list, data, i, nil
		}
	}
	// insert a new committee fund at the end of the list
	idx = len(list.List)
	d = &types.CommitteeData{
		CommitteeId:     committeeID,
		CommitteeHeight: 0,
		ChainHeight:     0,
		PaymentPercents: make([]*lib.PaymentPercents, 0),
		NumberOfSamples: 0,
	}
	list.List = append(list.List, d)
	return
}

// SetCommitteesData() sets a list of List into the state
func (s *StateMachine) SetCommitteesData(f *types.CommitteesData) lib.ErrorI {
	bz, err := lib.Marshal(f)
	if err != nil {
		return err
	}
	return s.Set(types.CommitteesDataPrefix(), bz)
}

// GetCommitteesData() gets a list of List from the state
func (s *StateMachine) GetCommitteesData() (f *types.CommitteesData, err lib.ErrorI) {
	bz, err := s.Get(types.CommitteesDataPrefix())
	if err != nil {
		return nil, err
	}
	f = &types.CommitteesData{
		List: make([]*types.CommitteeData, 0),
	}
	err = lib.Unmarshal(bz, f)
	return
}
