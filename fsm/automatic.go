package fsm

import (
	"github.com/canopy-network/canopy/fsm/types"
	"github.com/canopy-network/canopy/lib"
)

// BeginBlock() is code that is executed at the start of `applying` the block
func (s *StateMachine) BeginBlock() lib.ErrorI {
	// prevent attempting to load the certificate for height 0
	if s.Height() <= 1 {
		return nil
	}
	// enforce protocol upgrades
	if err := s.CheckProtocolVersion(); err != nil {
		return err
	}
	// reward committees
	if err := s.FundCommitteeRewardPools(); err != nil {
		return err
	}
	// handle last certificate results
	lastCertificate, err := s.LoadCertificate(s.Height() - 1)
	if err != nil {
		return err
	}
	// load the root chain id at the certificate height
	rootChainId, err := s.LoadRootChainId(s.Height() - 1)
	if err != nil {
		return err
	}
	// if not root-Chain: the committee won't match the certificate result
	// so just set the committee to nil to ignore the byzantine evidence
	// the byzantine evidence is handled at `Transaction Level` on the root
	// chain with a HandleMessageCertificateResults
	if s.Config.ChainId != rootChainId {
		return s.HandleCertificateResults(lastCertificate, nil)
	}
	// if is root-chain: load the committee from state as the certificate result
	// will match the evidence and there's no Transaction to HandleMessageCertificateResults
	committee, err := s.LoadCommittee(s.Config.ChainId, s.Height()-1)
	if err != nil {
		return err
	}
	return s.HandleCertificateResults(lastCertificate, &committee)
}

// EndBlock() is code that is executed at the end of `applying` the block
func (s *StateMachine) EndBlock(proposerAddress []byte) (err lib.ErrorI) {
	// update the list of addresses who proposed the last blocks
	// this information is used for leader election
	if err = s.UpdateLastProposers(proposerAddress); err != nil {
		return err
	}
	// distribute the committee rewards based on the various certificate results
	if err = s.DistributeCommitteeRewards(); err != nil {
		return err
	}
	// force unstakes validators who have been paused for MaxPauseBlocks
	if err = s.ForceUnstakeMaxPaused(); err != nil {
		return
	}
	// delete validators who are finishing unstaking
	if err = s.DeleteFinishedUnstaking(); err != nil {
		return
	}
	return
}

// CheckProtocolVersion() compares the protocol version against the governance enforced version
func (s *StateMachine) CheckProtocolVersion() (err lib.ErrorI) {
	// get the governance parameters
	params, err := s.GetParamsCons()
	if err != nil {
		return
	}
	// get the protocol version
	version, err := params.ParseProtocolVersion()
	if err != nil {
		return
	}
	// ensure that the software version is correct
	if s.Height() >= version.Height && s.ProtocolVersion < version.Version {
		return types.ErrInvalidProtocolVersion()
	}
	return
}

// HandleCertificateResults() is a handler for the results of a quorum certificate
func (s *StateMachine) HandleCertificateResults(qc *lib.QuorumCertificate, committee *lib.ValidatorSet) lib.ErrorI {
	// ensure the certificate results are not nil
	if qc == nil || qc.Results == nil {
		return lib.ErrNilCertResults()
	}
	// ensure the committee isn't retired
	retired, err := s.CommitteeIsRetired(qc.Header.ChainId)
	if err != nil {
		return err
	}
	// block the certificate results message
	if retired {
		return types.ErrNonSubsidizedCommittee()
	}
	// get the last data for the committee
	data, err := s.GetCommitteeData(qc.Header.ChainId)
	if err != nil {
		return err
	}
	// ensure the root height isn't too old
	if qc.Header.RootHeight < data.LastRootHeightUpdated {
		return lib.ErrInvalidQCRootChainHeight()
	}
	// ensure the chain height isn't too old
	if qc.Header.Height <= data.LastChainHeightUpdated {
		return lib.ErrInvalidQCCommitteeHeight()
	}
	results, chainId := qc.Results, qc.Header.ChainId
	// handle the token swaps ordered by the quorum
	s.HandleCommitteeSwaps(results.Orders, chainId)
	// index the 'nested chain' checkpoint
	if err = s.HandleCheckpoint(chainId, results); err != nil {
		return err
	}
	// handle byzantine evidence
	nonSignerPercent, err := s.HandleByzantine(qc, committee)
	if err != nil {
		return err
	}
	// reduce all payment percents proportional to the non-signer percent
	for i, p := range results.RewardRecipients.PaymentPercents {
		results.RewardRecipients.PaymentPercents[i].Percent = lib.Uint64ReducePercentage(p.Percent, uint64(nonSignerPercent))
	}
	// if the quorum is signalling 'retire' for a 'nestedChain'
	if qc.Results.Retired && qc.Header.ChainId != s.Config.ChainId {
		// retire the committeeId on this root
		if err = s.RetireCommittee(qc.Header.ChainId); err != nil {
			return err
		}
	}
	// update the committee data
	return s.UpsertCommitteeData(&lib.CommitteeData{
		ChainId:                chainId,
		LastRootHeightUpdated:  qc.Header.RootHeight,
		LastChainHeightUpdated: qc.Header.Height,
		PaymentPercents:        results.RewardRecipients.PaymentPercents,
	})
}

// HandleCheckpoint() handles the `checkpoint-as-a-service` root-Chain functionality
// NOTE: this will index self checkpoints - but allows for nested-chain checkpointing too
func (s *StateMachine) HandleCheckpoint(chainId uint64, results *lib.CertificateResult) (err lib.ErrorI) {
	storeI := s.store.(lib.StoreI)
	// index the checkpoint
	if results.Checkpoint != nil && len(results.Checkpoint.BlockHash) != 0 {
		// retrieve the last saved checkpoint for this chain
		mostRecentCheckpoint, e := storeI.GetMostRecentCheckpoint(chainId)
		if e != nil {
			return e
		}
		// ensure checkpoint isn't older than the most recent
		if results.Checkpoint.Height <= mostRecentCheckpoint.Height {
			return types.ErrInvalidCheckpoint()
		}
		// index the checkpoint
		if err = storeI.IndexCheckpoint(chainId, results.Checkpoint); err != nil {
			return err
		}
	}
	return
}

// LAST PROPOSERS CODE BELOW

// UpdateLastProposers() adds an address to the 'last proposers'
func (s *StateMachine) UpdateLastProposers(address []byte) lib.ErrorI {
	// get the addresses of the last proposers array from the state
	list, err := s.GetLastProposers()
	if err != nil {
		return err
	}
	// if the list of addresses are empty
	if list == nil || len(list.Addresses) == 0 {
		list = new(lib.Proposers)
		list.Addresses = [][]byte{{}, {}, {}, {}, {}}
	}
	// determine the index based on the current height
	index := s.Height() % 5
	// set the address at the index
	list.Addresses[index] = address
	// set the list in state
	return s.SetLastProposers(list)
}

// GetLastProposers() returns the last Proposer addresses saved in the state
func (s *StateMachine) GetLastProposers() (*lib.Proposers, lib.ErrorI) {
	// get the bytes for the last proposers using the last proposers prefix
	bz, err := s.Get(types.LastProposersPrefix())
	if err != nil {
		return nil, err
	}
	// ensure no nil proposers list by creating a new reference object
	ptr := new(lib.Proposers)
	// convert the proposers list to the object
	if err = lib.Unmarshal(bz, ptr); err != nil {
		return nil, err
	}
	// return
	return ptr, nil
}

// SetLastProposers() saves the last Proposer addresses in the state
func (s *StateMachine) SetLastProposers(keys *lib.Proposers) lib.ErrorI {
	// convert the proposers list to bytes
	bz, err := lib.Marshal(keys)
	if err != nil {
		return err
	}
	// set the bytes under the proposers prefix key
	return s.Set(types.LastProposersPrefix(), bz)
}
