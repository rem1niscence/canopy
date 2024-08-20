package fsm

import (
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
)

// COMMITTEES BELOW

func (s *StateMachine) GetRewardedCommittees() (paidIDs []uint64, totalStakeFromPaidCommittes uint64, err lib.ErrorI) {
	supply, err := s.GetSupply()
	if err != nil {
		return nil, 0, err
	}
	committees := supply.CommitteesWithDelegations
	totalStakeFromAllCommittees, numCommittees := uint64(0), len(committees)
	for i := 0; i < numCommittees; i++ {
		totalStakeFromAllCommittees += committees[i].Amount
	}
	// We don't know how many of the chains are paid
	// We can't pre-pick a percentage ^ because we don't know how many paid chains there are
	// Given a distribution of stake between committees find a cutoff where committees stop receiving mint
	totalPercent, totalStakeFromPaidCommittees := float64(0), uint64(0)
	for i, committee := range committees {
		paidIDs = append(paidIDs, committee.Id)
		totalStakeFromPaidCommittees += committee.Amount
		// basic hyperbolic ceiling function: Σ (n / (n + 1)), for n from 1 to N
		// that requires each paid committee to be similar in stake percentage to its paid peers
		// this approach is superior over an average as it fairly allows the most significant
		// committees to dictate the cutoff point
		threshold := float64(i+1) / float64(i+2) * 100
		totalPercent += float64(committee.Amount) / float64(totalStakeFromAllCommittees) * 100
		if totalPercent > threshold {
			// check for alternate
			// admit alternate if he has half or more the stake of the last admitted
			altIdx := i + 1
			if altIdx >= numCommittees-1 {
				return // there is no such alternate
			}
			alternate := committees[altIdx]
			if committee.Amount/alternate.Amount <= 2 {
				paidIDs = append(paidIDs, alternate.Id)
				totalStakeFromPaidCommittees += committee.Amount
			}
			return
		}
	}
	return
}

func (s *StateMachine) GetCommittee(committeeID uint64) (vs lib.ValidatorSet, err lib.ErrorI) {
	p, err := s.GetParamsVal()
	if err != nil {
		return
	}
	it, err := s.Iterator(types.CommitteePrefix(committeeID))
	if err != nil {
		return
	}
	defer it.Close()
	members := make([]*lib.ConsensusValidator, 0)
	for i := uint64(0); it.Valid() && i < p.ValidatorMaxCommitteeSize; func() { it.Next(); i++ }() {
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

func (s *StateMachine) GetCommitteePaginated(p lib.PageParams, committeeId uint64) (page *lib.Page, err lib.ErrorI) {
	return s.getValidatorsPaginated(p, lib.ValidatorFilters{}, types.CommitteePrefix(committeeId))
}

func (s *StateMachine) UpdateCommittees(address crypto.AddressI, oldValidator *types.Validator, newStakedAmount uint64, newCommittees []*types.Committee) lib.ErrorI {
	if err := s.DeleteCommittees(address, oldValidator.StakedAmount, oldValidator.Committees); err != nil {
		return err
	}
	if oldValidator.MaxPausedHeight != 0 {
		return nil // don't set if paused
	}
	return s.SetCommittees(address, newStakedAmount, newCommittees)
}

func (s *StateMachine) SetCommittees(address crypto.AddressI, totalStake uint64, committees []*types.Committee) lib.ErrorI {
	for _, committee := range committees {
		stakeForCommittee := lib.Uint64Percentage(totalStake, committee.StakePercent)
		if err := s.SetCommitteeMember(address, committee.Id, stakeForCommittee); err != nil {
			return err
		}
		if err := s.AddToCommitteeStakedSupply(committee.Id, stakeForCommittee); err != nil {
			return err
		}
	}
	return nil
}

func (s *StateMachine) DeleteCommittees(address crypto.AddressI, totalStake uint64, committees []*types.Committee) lib.ErrorI {
	for _, committee := range committees {
		stakeForCommittee := lib.Uint64Percentage(totalStake, committee.StakePercent)
		if err := s.DeleteCommitteeMember(address, committee.Id, stakeForCommittee); err != nil {
			return err
		}
		if err := s.SubFromCommitteeStakedSupply(committee.Id, stakeForCommittee); err != nil {
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

func (s *StateMachine) UpdateDelegations(address crypto.AddressI, oldValidator *types.Validator, newStakedAmount uint64, newCommittees []*types.Committee) lib.ErrorI {
	if err := s.DeleteDelegations(address, oldValidator.StakedAmount, oldValidator.Committees); err != nil {
		return err
	}
	return s.SetDelegations(address, newStakedAmount, newCommittees)
}

func (s *StateMachine) SetDelegations(address crypto.AddressI, totalStake uint64, committees []*types.Committee) lib.ErrorI {
	for _, committee := range committees {
		stakeForCommittee := lib.Uint64Percentage(totalStake, committee.StakePercent)
		if err := s.SetDelegate(address, committee.Id, stakeForCommittee); err != nil {
			return err
		}
		if err := s.AddToDelegateStakedSupply(committee.Id, stakeForCommittee); err != nil {
			return err
		}
	}
	return nil
}

func (s *StateMachine) DeleteDelegations(address crypto.AddressI, totalStake uint64, committees []*types.Committee) lib.ErrorI {
	for _, committee := range committees {
		stakeForCommittee := lib.Uint64Percentage(totalStake, committee.StakePercent)
		if err := s.DeleteDelegate(address, committee.Id, stakeForCommittee); err != nil {
			return err
		}
		if err := s.SubFromDelegateStakedSupply(committee.Id, stakeForCommittee); err != nil {
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

// PROPOSALS BELOW

func (s *StateMachine) UpsertProposal(upsert *lib.Proposal) lib.ErrorI {
	p, proposal, idx, err := s.getProposal(upsert.Meta.CommitteeId)
	if upsert.Meta.CommitteeHeight <= proposal.Meta.CommitteeHeight && upsert.Meta.ChainHeight <= proposal.Meta.ChainHeight {
		return types.ErrInvalidProposal()
	}
	if err = proposal.Combine(upsert); err != nil {
		return err
	}
	p.Proposals[idx] = proposal
	return s.SetProposals(p)
}

func (s *StateMachine) GetProposal(committeeID uint64) (*lib.Proposal, lib.ErrorI) {
	_, p, _, err := s.getProposal(committeeID)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (s *StateMachine) getProposal(committeeID uint64) (p *lib.Proposals, e *lib.Proposal, idx int, err lib.ErrorI) {
	p, err = s.GetProposals()
	if err != nil {
		return
	}
	for i, proposal := range p.Proposals {
		if proposal.Meta.CommitteeId == committeeID {
			return p, proposal, i, nil
		}
	}
	idx = len(p.Proposals)
	e = &lib.Proposal{
		RewardRecipients: &lib.RewardRecipients{
			PaymentPercents: make([]*lib.PaymentPercents, 0),
			NumberOfSamples: 0,
		},
		Meta: &lib.ProposalMeta{
			CommitteeId:     committeeID,
			CommitteeHeight: 0,
			ChainHeight:     0,
			DoubleSigners:   nil,
			BadProposers:    nil,
		},
	}
	p.Proposals = append(p.Proposals, e)
	return
}

func (s *StateMachine) SetProposals(p *lib.Proposals) lib.ErrorI {
	bz, err := lib.Marshal(p)
	if err != nil {
		return err
	}
	return s.Set(types.ProposalsPrefix(), bz)
}

func (s *StateMachine) GetProposals() (p *lib.Proposals, err lib.ErrorI) {
	bz, err := s.Get(types.ProposalsPrefix())
	if err != nil {
		return nil, err
	}
	p = &lib.Proposals{
		Proposals: make([]*lib.Proposal, 0),
	}
	err = lib.Unmarshal(bz, p)
	return
}
