package fsm

import (
	"github.com/canopy-network/canopy/fsm/types"
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
	"math"
)

// COMMITTEES BELOW

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
	subsidizedCommitteeIds, err := s.GetPaidCommittees(params)
	if err != nil {
		return err
	}
	// calculate the number of halvenings
	halvenings := s.height / uint64(types.BlocksPerHalvening)
	// each halving, the reward is divided by 2
	totalMintAmount := uint64(float64(types.InitialTokensPerBlock) / (math.Pow(2, float64(halvenings))))
	// define a convenience variable for the number of subsidized committees
	subsidizedCount := uint64(len(subsidizedCommitteeIds))
	// if there are no subsidized committees or no mint amount
	if subsidizedCount == 0 || totalMintAmount == 0 {
		return nil
	}
	// calculate the amount left for the committees after the parameterized DAO cut
	mintAmountAfterDAOCut := lib.Uint64ReducePercentage(totalMintAmount, float64(govParams.DaoRewardPercentage))
	// calculate the DAO cut
	daoCut := totalMintAmount - mintAmountAfterDAOCut
	// mint to the DAO account
	if err = s.MintToPool(lib.DAOPoolID, daoCut); err != nil {
		return err
	}
	// calculate the amount given to each qualifying committee
	mintAmountPerCommittee := mintAmountAfterDAOCut / subsidizedCount
	// issue that amount to each subsidized committee
	for _, committeeId := range subsidizedCommitteeIds {
		if err = s.MintToPool(committeeId, mintAmountPerCommittee); err != nil {
			return err
		}
	}
	return nil
}

// GetPaidCommittees() returns a list of committeeIDs that receive a portion of the 'block reward'
// Think of these committees as 'automatically subsidized' by the protocol
func (s *StateMachine) GetPaidCommittees(valParams *types.ValidatorParams) (paidIDs []uint64, err lib.ErrorI) {
	// retrieve the supply
	supply, err := s.GetSupply()
	if err != nil {
		return nil, err
	}
	// since re-staking is enabled in the Canopy protocol, the protocol is able to simply pick which chains are paid via a 'committed stake'
	// percentage. Since the effective cost of 're-staking' is essentially zero (minus the risk associated with slashing on the chain)
	// this may be thought of like a popularity contest or a vote.
	for _, committee := range supply.CommitteeStaked {
		// calculate the percent of stake the committee controls
		committedStakePercent := lib.Uint64PercentageDiv(committee.Amount, supply.Staked)
		// if the committee percentage is over the threshold
		if committedStakePercent >= valParams.ValidatorStakePercentForSubsidizedCommittee {
			// add it to the paid list
			paidIDs = append(paidIDs, committee.Id)
		}
	}
	return
}

// DistributeCommitteeRewards() distributes the committee mint based on the PaymentPercents from the result of the QuorumCertificate
func (s *StateMachine) DistributeCommitteeRewards() lib.ErrorI {
	// retrieve the necessary parameters
	paramsVal, err := s.GetParamsVal()
	if err != nil {
		return err
	}
	// retrieve the master list of committee data
	committeesData, err := s.GetCommitteesData()
	if err != nil {
		return err
	}
	// for each committee data in the list
	for i, data := range committeesData.List {
		// check to see if any payment percents were issued
		if len(data.PaymentPercents) == 0 {
			// if none issued, move on to the next
			continue
		}
		// retrieve the reward pool
		rewardPool, e := s.GetPool(data.CommitteeId)
		if e != nil {
			return e
		}
		// for each payment percent issued
		for _, stub := range data.PaymentPercents {
			if err = s.DistributeCommitteeReward(stub, rewardPool.Amount, data.NumberOfSamples, paramsVal); err != nil {
				return err
			}
		}
		// zero out the reward pool
		rewardPool.Amount = 0
		if err = s.SetPool(rewardPool); err != nil {
			return err
		}
		// clear the committee data, but leave the ID, (external) chain height, and committee height
		committeesData.List[i] = &types.CommitteeData{
			CommitteeId:             data.CommitteeId,
			LastCanopyHeightUpdated: data.LastCanopyHeightUpdated,
			LastChainHeightUpdated:  data.LastChainHeightUpdated,
		}
	}
	// set the committees data
	return s.SetCommitteesData(committeesData)
}

// DistributeCommitteeReward() issues a single committee reward unit based on an individual 'Payment Stub'
func (s *StateMachine) DistributeCommitteeReward(stub *lib.PaymentPercents, rewardPoolAmount, numberOfSamples uint64, valParams *types.ValidatorParams) lib.ErrorI {
	address := crypto.NewAddress(stub.Address)
	// full_reward = truncate ( percentage / number_of_samples * available_reward )
	fullReward := uint64(float64(stub.Percent) / float64(numberOfSamples*100) * float64(rewardPoolAmount))
	// if not compounding, use the early withdrawal reward
	earlyWithdrawalReward := lib.Uint64ReducePercentage(fullReward, float64(valParams.ValidatorEarlyWithdrawalPenalty))
	// check if is validator
	validator, _ := s.GetValidator(address)
	// if non validator, send EarlyWithdrawalReward to the address
	if validator == nil {
		return s.AccountAdd(address, earlyWithdrawalReward)
	}
	// if validator and compounding, send full reward to the stake of the validator
	if validator.Compound && validator.UnstakingHeight == 0 {
		return s.UpdateValidatorStake(validator, validator.Committees, fullReward)
	}
	// if validator is not compounding, send earlyWithdrawalReward reward to the output address of the validator
	return s.AccountAdd(crypto.NewAddress(validator.Output), earlyWithdrawalReward)
}

// GetCommitteeMembers() retrieves the ValidatorSet that is responsible for the 'committeeId'
func (s *StateMachine) GetCommitteeMembers(committeeID uint64, all ...bool) (vs lib.ValidatorSet, err lib.ErrorI) {
	// get the validator params
	p, err := s.GetParamsVal()
	if err != nil {
		return
	}
	// set the maximum size limit of the committee
	maxSize := p.ValidatorMaxCommitteeSize
	if all != nil && all[0] {
		maxSize = math.MaxUint64
	}
	// iterate through the prefix for the committee, from the highest stake amount to lowest
	it, err := s.RevIterator(types.CommitteePrefix(committeeID))
	if err != nil {
		return
	}
	defer it.Close()
	// create a variable to hold the committee members
	members := make([]*lib.ConsensusValidator, 0)
	// loop through the iterator
	for i := uint64(0); it.Valid() && i < maxSize; func() { it.Next(); i++ }() {
		// get the address from the iterator key
		address, e := types.AddressFromKey(it.Key())
		if e != nil {
			return vs, e
		}
		// get the validator from the address
		val, e := s.GetValidator(address)
		if e != nil {
			return vs, e
		}
		// ensure the validator is not included in the committee if it's paused or unstaking
		if val.MaxPausedHeight != 0 || val.UnstakingHeight != 0 {
			continue
		}
		// add the member to the list
		members = append(members, &lib.ConsensusValidator{
			PublicKey:   val.PublicKey,
			VotingPower: val.StakedAmount,
			NetAddress:  val.NetAddress,
		})
	}
	// convert list to a validator set (includes shared public key)
	return lib.NewValidatorSet(&lib.ConsensusValidators{ValidatorSet: members})
}

// GetCanopyCommitteeMembers() returns the committee members specifically for the Canopy ID
func (s *StateMachine) GetCanopyCommitteeMembers(all ...bool) (*lib.ConsensusValidators, lib.ErrorI) {
	// get the members for the CanopyCommitteeId
	canopyCommittee, err := s.GetCommitteeMembers(lib.CanopyCommitteeId, all...)
	if err != nil {
		return nil, err
	}
	// only return the validator list, not the full 'Set' which includes a shared public key and other meta information
	return canopyCommittee.ValidatorSet, nil
}

// GetCommitteePaginated() returns a 'page' of committee members ordered from highest stake to lowest
func (s *StateMachine) GetCommitteePaginated(p lib.PageParams, committeeId uint64) (page *lib.Page, err lib.ErrorI) {
	page, res := lib.NewPage(p, types.ValidatorsPageName), make(types.ValidatorPage, 0)
	err = page.Load(types.CommitteePrefix(committeeId), true, &res, s.store, func(k, _ []byte) (err lib.ErrorI) {
		// get the address from the key
		address, err := types.AddressFromKey(k)
		if err != nil {
			return err
		}
		// get the validator from the address
		validator, err := s.GetValidator(address)
		if err != nil {
			return err
		}
		// if validator is not paused and not unstaking
		if validator.UnstakingHeight == 0 || validator.MaxPausedHeight == 0 {
			// append the validator to the page
			res = append(res, validator)
		}
		return
	})
	return
}

// UpdateCommittees() updates the committee information in state for a specific validator
func (s *StateMachine) UpdateCommittees(address crypto.AddressI, oldValidator *types.Validator, newStakedAmount uint64, newCommittees []uint64) lib.ErrorI {
	// delete the committee information for the validator
	if err := s.DeleteCommittees(address, oldValidator.StakedAmount, oldValidator.Committees); err != nil {
		return err
	}
	// re-set the committees with the new information
	return s.SetCommittees(address, newStakedAmount, newCommittees)
}

// SetCommittees() sets the membership and staked supply for all an addresses' committees
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

// DeleteCommittees() deletes the membership and staked supply for all an addresses' committees
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

// SetCommitteeMember() sets the address as a 'member' of the committee in the state
func (s *StateMachine) SetCommitteeMember(address crypto.AddressI, committeeID, stakeForCommittee uint64) lib.ErrorI {
	return s.Set(types.KeyForCommittee(committeeID, address, stakeForCommittee), nil)
}

// DeleteCommitteeMember() removes the address from being a 'member' of the committee in the state
func (s *StateMachine) DeleteCommitteeMember(address crypto.AddressI, committeeID, stakeForCommittee uint64) lib.ErrorI {
	return s.Delete(types.KeyForCommittee(committeeID, address, stakeForCommittee))
}

// DELEGATIONS BELOW

func (s *StateMachine) GetDelegatesPaginated(p lib.PageParams, committeeId uint64) (page *lib.Page, err lib.ErrorI) {
	page, res := lib.NewPage(p, types.ValidatorsPageName), make(types.ValidatorPage, 0)
	err = page.Load(types.DelegatePrefix(committeeId), true, &res, s.store, func(k, _ []byte) (err lib.ErrorI) {
		// get the address from the key
		address, err := types.AddressFromKey(k)
		if err != nil {
			return err
		}
		// get the validator from the address
		validator, err := s.GetValidator(address)
		if err != nil {
			return err
		}
		// if validator is not paused and not unstaking
		if validator.UnstakingHeight == 0 || validator.MaxPausedHeight == 0 {
			// append the validator to the page
			res = append(res, validator)
		}
		return
	})
	return
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
// 'Committee Data' is information saved in state that is aggregated from CertificateResult messages
// This information secures and dictates the distribution of a Committee's reward pool

// UpsertCommitteeData() updates or inserts a committee data to the committees data list
func (s *StateMachine) UpsertCommitteeData(new *types.CommitteeData) lib.ErrorI {
	// retrieve the committees' data list, the target and index in the list based on the committeeId
	committeesData, targetData, idx, err := s.getCommitteeDataAndList(new.CommitteeId)
	// check the new committee data is not 'out-dated'
	if new.LastCanopyHeightUpdated < targetData.LastCanopyHeightUpdated || new.LastChainHeightUpdated <= targetData.LastChainHeightUpdated {
		return types.ErrInvalidCertificateResults()
	}
	// combine the new data with the target
	if err = targetData.Combine(new); err != nil {
		return err
	}
	// add the target back into the list
	committeesData.List[idx] = targetData
	// set the list back into state
	return s.SetCommitteesData(committeesData)
}

// GetCommitteeData() is a convenience function to retrieve the committee data from the master list
func (s *StateMachine) GetCommitteeData(targetCommitteeID uint64) (*types.CommitteeData, lib.ErrorI) {
	// pass through call
	_, targetData, _, err := s.getCommitteeDataAndList(targetCommitteeID)
	if err != nil {
		return nil, err
	}
	// return the target committee data and error only
	return targetData, nil
}

// getCommitteeDataAndList() returns the master list of committee data and the specified target data and its index from the target committee id
func (s *StateMachine) getCommitteeDataAndList(targetCommitteeID uint64) (list *types.CommitteesData, d *types.CommitteeData, idx int, err lib.ErrorI) {
	// first, get the master list of 'committee data'
	list, err = s.GetCommitteesData()
	if err != nil {
		return
	}
	// linear search for the committee data
	for i, data := range list.List {
		// if target found, return
		if data.CommitteeId == targetCommitteeID {
			return list, data, i, nil
		}
	}
	// if target is not found in the list...
	// the target index is the new list end
	idx = len(list.List)
	// set the committee data in the returned variable
	d = &types.CommitteeData{
		CommitteeId:             targetCommitteeID,
		LastCanopyHeightUpdated: 0,
		LastChainHeightUpdated:  0,
		PaymentPercents:         make([]*lib.PaymentPercents, 0),
		NumberOfSamples:         0,
	}
	// insert a new committee fund at the end of the list
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
