package fsm

import (
	"bytes"
	"github.com/canopy-network/canopy/fsm/types"
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
	"slices"
)

// COMMITTEES BELOW

// FundCommitteeRewardPools() mints newly created tokens to protocol subsidized committees
func (s *StateMachine) FundCommitteeRewardPools() lib.ErrorI {
	// get governance params that are needed to complete this operation
	govParams, err := s.GetParamsGov()
	if err != nil {
		return err
	}
	// get the committees that `qualify` for subsidization
	subsidizedChainIds, err := s.GetSubsidizedCommittees()
	if err != nil {
		return err
	}
	// ensure self chain is always a 'paid' chain even if there are no validators
	if !slices.Contains(subsidizedChainIds, s.Config.ChainId) {
		// this ensures nested-chains always receive Native Token payment to their pool
		subsidizedChainIds = append(subsidizedChainIds, s.Config.ChainId)
	}
	// calculate the number of halvenings
	halvenings := s.height / uint64(types.BlocksPerHalvening)
	// each halving, the reward is divided by 2
	totalMintAmount := uint64(types.InitialTokensPerBlock >> halvenings)
	// define a convenience variable for the number of subsidized committees
	subsidizedCount := uint64(len(subsidizedChainIds))
	// if there are no subsidized committees or no mint amount
	if subsidizedCount == 0 || totalMintAmount == 0 {
		return nil
	}
	// calculate the amount left for the committees after the parameterized DAO cut
	mintAmountAfterDAOCut := lib.Uint64ReducePercentage(totalMintAmount, govParams.DaoRewardPercentage)
	// calculate the DAO cut
	daoCut := totalMintAmount - mintAmountAfterDAOCut
	// mint to the DAO account
	if err = s.MintToPool(lib.DAOPoolID, daoCut); err != nil {
		return err
	}
	// calculate the amount given to each qualifying committee
	// mintAmountPerCommittee may truncate, but that's expected,
	// less mint will be created and effectively 'burned'
	mintAmountPerCommittee := mintAmountAfterDAOCut / subsidizedCount
	// issue that amount to each subsidized committee
	for _, chainId := range subsidizedChainIds {
		if err = s.MintToPool(chainId, mintAmountPerCommittee); err != nil {
			return err
		}
	}
	return nil
}

// GetSubsidizedCommittees() returns a list of chainIds that receive a portion of the 'block reward'
// Think of these committees as 'automatically subsidized' by the protocol
func (s *StateMachine) GetSubsidizedCommittees() (paidIDs []uint64, err lib.ErrorI) {
	// get validator params that are needed to complete this operation
	valParams, err := s.GetParamsVal()
	if err != nil {
		return nil, err
	}
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
		if committedStakePercent >= valParams.StakePercentForSubsidizedCommittee {
			// get retired status of the committee
			retired, e := s.CommitteeIsRetired(committee.Id)
			if e != nil {
				return nil, e
			}
			// ensure the committee isn't retired
			if retired {
				s.log.Warnf("Not subsidizing retired committee: %d", committee.Id)
				continue
			}
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
		rewardPool, e := s.GetPool(data.ChainId)
		if e != nil {
			return e
		}
		// create a tracker variable for total amount distributed
		var totalDistributed uint64
		// for each payment percent issued
		for _, stub := range data.PaymentPercents {
			distributed, er := s.DistributeCommitteeReward(stub, rewardPool.Amount, data.NumberOfSamples, paramsVal)
			if er != nil {
				return er
			}
			totalDistributed += distributed
		}
		// ensure the non-distributed (burned) is removed from the 'total supply'
		if err = s.SubFromTotalSupply(rewardPool.Amount - totalDistributed); err != nil {
			return err
		}
		// zero out the reward pool
		rewardPool.Amount = 0
		// update the pool in state
		if err = s.SetPool(rewardPool); err != nil {
			return err
		}
		// clear the committee data, but leave the ID, (external) chain height, and committee height
		committeesData.List[i] = &lib.CommitteeData{
			ChainId:                data.ChainId,
			LastRootHeightUpdated:  data.LastRootHeightUpdated,
			LastChainHeightUpdated: data.LastChainHeightUpdated,
		}
	}
	// set the committee data in state
	return s.SetCommitteesData(committeesData)
}

// DistributeCommitteeReward() issues a single committee reward unit based on an individual 'Payment Stub'
func (s *StateMachine) DistributeCommitteeReward(stub *lib.PaymentPercents, rewardPoolAmount, numberOfSamples uint64, valParams *types.ValidatorParams) (distributed uint64, err lib.ErrorI) {
	address := crypto.NewAddress(stub.Address)
	// full_reward = truncate ( percentage / number_of_samples * available_reward )
	fullReward := (stub.Percent * rewardPoolAmount) / (numberOfSamples * 100)
	// if not compounding, use the early withdrawal reward
	earlyWithdrawalReward := lib.Uint64ReducePercentage(fullReward, valParams.EarlyWithdrawalPenalty)
	// check if is validator
	validator, _ := s.GetValidator(address)
	// if non validator, send EarlyWithdrawalReward to the address
	if validator == nil {
		// add directly to the account with an early withdrawal penalty
		return earlyWithdrawalReward, s.AccountAdd(address, earlyWithdrawalReward)
	}
	// if validator and compounding, send full reward to the stake of the validator
	if validator.Compound && validator.UnstakingHeight == 0 {
		return fullReward, s.UpdateValidatorStake(validator, validator.Committees, fullReward)
	}
	// if validator is not compounding, send earlyWithdrawalReward reward to the output address of the validator
	return earlyWithdrawalReward, s.AccountAdd(crypto.NewAddress(validator.Output), earlyWithdrawalReward)
}

// GetCommitteeMembers() retrieves the ValidatorSet that is responsible for the 'chainId'
func (s *StateMachine) GetCommitteeMembers(chainId uint64) (vs lib.ValidatorSet, err lib.ErrorI) {
	// get the validator params
	p, err := s.GetParamsVal()
	if err != nil {
		return
	}
	// iterate through the prefix for the committee, from the highest stake amount to lowest
	it, err := s.RevIterator(types.CommitteePrefix(chainId))
	if err != nil {
		return
	}
	defer it.Close()
	// create a variable to hold the committee members
	members := make([]*lib.ConsensusValidator, 0)
	// for each item of the iterator up to MaxCommitteeSize
	for i := uint64(0); it.Valid() && i <= p.MaxCommitteeSize; func() { it.Next(); i++ }() {
		// extract the address from the iterator key
		address, e := types.AddressFromKey(it.Key())
		if e != nil {
			return vs, e
		}
		// load the validator from the state using the address
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

// GetCommitteePaginated() returns a 'page' of committee members ordered from the highest stake to lowest
func (s *StateMachine) GetCommitteePaginated(p lib.PageParams, chainId uint64) (page *lib.Page, err lib.ErrorI) {
	// define a page and result variables
	page, res := lib.NewPage(p, types.ValidatorsPageName), make(types.ValidatorPage, 0)
	// populate the page using an iterator over the 'committee prefix' ordered by stake (high to low)
	err = page.Load(types.CommitteePrefix(chainId), true, &res, s.store, func(key, value []byte) (err lib.ErrorI) {
		// get the address from the key
		address, err := types.AddressFromKey(key)
		if err != nil {
			return err
		}
		// get the validator from the address
		validator, err := s.GetValidator(address)
		if err != nil {
			return err
		}
		// skip if validator is not paused and not unstaking
		if validator.UnstakingHeight != 0 || validator.MaxPausedHeight != 0 {
			return
		}
		// append the validator to the page
		res = append(res, validator)
		return
	})
	return
}

// UpdateCommittees() updates the committee information in state for a specific validator
func (s *StateMachine) UpdateCommittees(address crypto.AddressI, oldValidator *types.Validator, newStakedAmount uint64, newCommittees []uint64) lib.ErrorI {
	// delete the committee information based on the 'previous state' of the validator
	if err := s.DeleteCommittees(address, oldValidator.StakedAmount, oldValidator.Committees); err != nil {
		return err
	}
	// set the committee information using the updated stake and committees
	return s.SetCommittees(address, newStakedAmount, newCommittees)
}

// SetCommittees() sets the membership and staked supply for all an addresses' committees
func (s *StateMachine) SetCommittees(address crypto.AddressI, totalStake uint64, committees []uint64) (err lib.ErrorI) {
	// for each committee in the list
	for _, committee := range committees {
		// set the address as a member
		if err = s.SetCommitteeMember(address, committee, totalStake); err != nil {
			return
		}
		// add to the committee staked supply
		if err = s.AddToCommitteeStakedSupply(committee, totalStake); err != nil {
			return
		}
	}
	return
}

// DeleteCommittees() deletes the membership and staked supply for each of an address' committees
func (s *StateMachine) DeleteCommittees(address crypto.AddressI, totalStake uint64, committees []uint64) (err lib.ErrorI) {
	// for each committee in the list
	for _, committee := range committees {
		// remove the address from being a member
		if err = s.DeleteCommitteeMember(address, committee, totalStake); err != nil {
			return
		}
		// subtract from the committee staked supply
		if err = s.SubFromCommitteeStakedSupply(committee, totalStake); err != nil {
			return
		}
	}
	return
}

// SetCommitteeMember() sets the address as a 'member' of the committee in the state
func (s *StateMachine) SetCommitteeMember(address crypto.AddressI, chainId, stakeForCommittee uint64) lib.ErrorI {
	return s.Set(types.KeyForCommittee(chainId, address, stakeForCommittee), nil)
}

// DeleteCommitteeMember() removes the address from being a 'member' of the committee in the state
func (s *StateMachine) DeleteCommitteeMember(address crypto.AddressI, chainId, stakeForCommittee uint64) lib.ErrorI {
	return s.Delete(types.KeyForCommittee(chainId, address, stakeForCommittee))
}

// DELEGATIONS BELOW

// GetAllDelegates() returns all delegates for a certain chainId
func (s *StateMachine) GetAllDelegates(chainId uint64) (vs lib.ValidatorSet, err lib.ErrorI) {
	// iterate from highest stake to lowest
	it, err := s.RevIterator(types.DelegatePrefix(chainId))
	if err != nil {
		return vs, err
	}
	defer it.Close()
	// create a variable to hold the committee members
	members := make([]*lib.ConsensusValidator, 0)
	// loop through the iterator
	for ; it.Valid(); it.Next() {
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
	return lib.NewValidatorSet(&lib.ConsensusValidators{ValidatorSet: members})
}

// GetDelegatesPaginated() returns a page of delegates
func (s *StateMachine) GetDelegatesPaginated(p lib.PageParams, chainId uint64) (page *lib.Page, err lib.ErrorI) {
	// create a page of validator objects
	page, res := lib.NewPage(p, types.ValidatorsPageName), make(types.ValidatorPage, 0)
	// populate the page using the 'delegates' prefix sorted by stake (high to low)
	err = page.Load(types.DelegatePrefix(chainId), true, &res, s.store, func(key, _ []byte) (err lib.ErrorI) {
		// get the address from the key
		address, err := types.AddressFromKey(key)
		if err != nil {
			return err
		}
		// get the validator from the address
		validator, err := s.GetValidator(address)
		if err != nil {
			return err
		}
		// skip if validator is paused or unstaking
		if validator.UnstakingHeight != 0 || validator.MaxPausedHeight != 0 {
			return
		}
		// append the validator to the page
		res = append(res, validator)
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

func (s *StateMachine) SetDelegate(address crypto.AddressI, chainId, stakeForCommittee uint64) lib.ErrorI {
	return s.Set(types.KeyForDelegate(chainId, address, stakeForCommittee), nil)
}

func (s *StateMachine) DeleteDelegate(address crypto.AddressI, chainId, stakeForCommittee uint64) lib.ErrorI {
	return s.Delete(types.KeyForDelegate(chainId, address, stakeForCommittee))
}

// COMMITTEE DATA CODE BELOW
// 'Committee Data' is information saved in state that is aggregated from CertificateResult messages
// This information secures and dictates the distribution of a Committee's reward pool

// UpsertCommitteeData() updates or inserts a committee data to the committees data list
func (s *StateMachine) UpsertCommitteeData(new *lib.CommitteeData) lib.ErrorI {
	// retrieve the committees' data list, the target and index in the list based on the chainId
	committeesData, targetData, idx, err := s.getCommitteeDataAndList(new.ChainId)
	if err != nil {
		return err
	}
	// check the new committee data is not 'out-dated'
	if new.LastChainHeightUpdated <= targetData.LastChainHeightUpdated {
		return lib.ErrInvalidQCCommitteeHeight()
	}
	if new.LastRootHeightUpdated < targetData.LastRootHeightUpdated {
		return lib.ErrInvalidQCRootChainHeight()
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

// OverwriteCommitteeData() overwrites the committee data in state
// note: use UpsertCommitteeData for the safe committee upsert
func (s *StateMachine) OverwriteCommitteeData(d *lib.CommitteeData) lib.ErrorI {
	// retrieve the committees' data list, the target and index in the list based on the chainId
	committeesData, targetData, idx, err := s.getCommitteeDataAndList(d.ChainId)
	if err != nil {
		return err
	}
	// add the target back into the list
	committeesData.List[idx] = targetData
	// set the list back into state
	return s.SetCommitteesData(committeesData)
}

// GetCommitteeData() is a convenience function to retrieve the committee data from the master list
func (s *StateMachine) GetCommitteeData(targetChainId uint64) (*lib.CommitteeData, lib.ErrorI) {
	// pass through call
	_, targetData, _, err := s.getCommitteeDataAndList(targetChainId)
	if err != nil {
		return nil, err
	}
	// return the target committee data and error only
	return targetData, nil
}

// getCommitteeDataAndList() returns the master list of committee data and the specified target data and its index from the target chain id
func (s *StateMachine) getCommitteeDataAndList(targetChainId uint64) (list *lib.CommitteesData, d *lib.CommitteeData, idx int, err lib.ErrorI) {
	// first, get the master list of 'committee data'
	list, err = s.GetCommitteesData()
	if err != nil {
		return
	}
	// linear search for the committee data
	for i, data := range list.List {
		// if target found, return
		if data.ChainId == targetChainId {
			return list, data, i, nil
		}
	}
	// if target is not found in the list...
	// the target index is the new list end
	idx = len(list.List)
	// set the committee data in the returned variable
	d = &lib.CommitteeData{
		ChainId:                targetChainId,
		LastRootHeightUpdated:  0,
		LastChainHeightUpdated: 0,
		PaymentPercents:        make([]*lib.PaymentPercents, 0),
		NumberOfSamples:        0,
	}
	// insert a new committee fund at the end of the list
	list.List = append(list.List, d)
	return
}

// SetCommitteesData() sets a list of List into the state
func (s *StateMachine) SetCommitteesData(f *lib.CommitteesData) lib.ErrorI {
	bz, err := lib.Marshal(f)
	if err != nil {
		return err
	}
	return s.Set(types.CommitteesDataPrefix(), bz)
}

// GetCommitteesData() gets a list of List from the state
func (s *StateMachine) GetCommitteesData() (f *lib.CommitteesData, err lib.ErrorI) {
	bz, err := s.Get(types.CommitteesDataPrefix())
	if err != nil {
		return nil, err
	}
	f = &lib.CommitteesData{
		List: make([]*lib.CommitteeData, 0),
	}
	err = lib.Unmarshal(bz, f)
	return
}

// RetireCommittee marks a committee as non-subsidized for eternity
// This is a useful mechanism to gracefully 'end' a committee
func (s *StateMachine) RetireCommittee(id uint64) lib.ErrorI {
	return s.Set(types.KeyForRetiredCommittee(id), types.RetiredCommitteesPrefix())
}

// CommitteeIsRetired checks if a committee is marked as 'retired' which prevents it from being subsidized for eternity
func (s *StateMachine) CommitteeIsRetired(id uint64) (bool, lib.ErrorI) {
	bz, err := s.Get(types.KeyForRetiredCommittee(id))
	if err != nil {
		return false, err
	}
	return bytes.Equal(types.RetiredCommitteesPrefix(), bz), nil
}

// GetRetiredCommittees() returns a list of the retired chainIds
func (s *StateMachine) GetRetiredCommittees() (result []uint64, err lib.ErrorI) {
	// for each item under the retired committee prefix
	err = s.IterateAndExecute(types.RetiredCommitteesPrefix(), func(key, _ []byte) (e lib.ErrorI) {
		// extract the id from the key
		id, e := types.IdFromKey(key)
		if e != nil {
			return
		}
		result = append(result, id)
		return
	})
	return
}

// SetRetiredCommittees() sets a list of chainIds as retired
func (s *StateMachine) SetRetiredCommittees(ids []uint64) (err lib.ErrorI) {
	for _, id := range ids {
		if err = s.RetireCommittee(id); err != nil {
			return
		}
	}
	return
}
