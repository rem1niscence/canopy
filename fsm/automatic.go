package fsm

import (
	"github.com/canopy-network/canopy/fsm/types"
	"github.com/canopy-network/canopy/lib"
)

// BeginBlock() is code that is executed at the start of `applying` the block
func (s *StateMachine) BeginBlock() lib.ErrorI {
	// if at genesis skip the BeginBlock
	if s.Height() <= 1 {
		return nil
	}
	// enforce upgrades
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
	// if not base-chain: the committee won't match the certificate result
	// so just set the committee to nil to ignore the byzantine evidence
	// the byzantine evidence is handled at `Transaction Level` via
	// HandleMessageCertificateResults
	if !s.Config.IsBaseChain() {
		return s.HandleCertificateResults(lastCertificate, nil)
	}
	// if is base-chain: load the committee from state as the certificate result
	// will match the evidence and there's no Transaction to HandleMessageCertificateResults
	committee, err := s.LoadCommittee(s.Config.ChainId, s.Height()-1)
	if err != nil {
		return err
	}
	return s.HandleCertificateResults(lastCertificate, &committee)
}

// EndBlock() is code that is executed at the end of `applying` the block
func (s *StateMachine) EndBlock(proposerAddress []byte) (err lib.ErrorI) {
	// update addresses who proposed the block
	if err = s.UpdateLastProposers(proposerAddress); err != nil {
		return err
	}
	// distribute the committee rewards based on the various Proposal Transactions
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

// HandleCertificateResults() is a handler for the results of a quorum certificate
func (s *StateMachine) HandleCertificateResults(qc *lib.QuorumCertificate, committee *lib.ValidatorSet) lib.ErrorI {
	// ensure the certificate results are not nil
	if qc == nil || qc.Results == nil {
		return lib.ErrNilCertResults()
	}
	// get the last data for the committee
	data, err := s.GetCommitteeData(qc.Header.CommitteeId)
	if err != nil {
		return err
	}
	// ensure the canopy height isn't too old
	if qc.Header.CanopyHeight < data.LastCanopyHeightUpdated {
		return lib.ErrInvalidQCBaseChainHeight()
	} // ensure the committee height isn't too old
	if qc.Header.Height <= data.LastChainHeightUpdated {
		return lib.ErrInvalidQCCommitteeHeight()
	}
	results, committeeId := qc.Results, qc.Header.CommitteeId
	// handle the token swaps
	if err = s.HandleCommitteeSwaps(results.Orders, committeeId); err != nil {
		return err
	}
	// index the checkpoint
	if err = s.HandleCheckpoint(committeeId, results); err != nil {
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
	// handle retired status
	if qc.Results.Retired {
		if err = s.RetireCommittee(qc.Header.CommitteeId); err != nil {
			return err
		}
	}
	// update the committee data
	return s.UpsertCommitteeData(&lib.CommitteeData{
		CommitteeId:             committeeId,
		LastCanopyHeightUpdated: qc.Header.CanopyHeight,
		LastChainHeightUpdated:  qc.Header.Height,
		PaymentPercents:         results.RewardRecipients.PaymentPercents,
	})
}

// HandleCheckpoint() handles the `checkpoint-as-a-service` base-chain functionality
// NOTE: this will index self checkpoints - but allows for sub-chain checkpointing too
func (s *StateMachine) HandleCheckpoint(committeeId uint64, results *lib.CertificateResult) (err lib.ErrorI) {
	storeI := s.store.(lib.StoreI)
	// index the checkpoint
	if results.Checkpoint != nil {
		// retrieve the last saved checkpoint for this chain
		mostRecentCheckpoint, e := storeI.GetMostRecentCheckpoint(committeeId)
		if e != nil {
			return e
		}
		// ensure checkpoint isn't older than the most recent
		if results.Checkpoint.Height <= mostRecentCheckpoint.Height {
			return types.ErrInvalidCheckpoint()
		}
		// index the checkpoint
		if err = storeI.IndexCheckpoint(committeeId, results.Checkpoint); err != nil {
			return err
		}
	}
	return
}

// LAST PROPOSERS CODE BELOW

// UpdateLastProposers() adds an address to the 'last proposers'
func (s *StateMachine) UpdateLastProposers(address []byte) lib.ErrorI {
	keys, err := s.GetLastProposers()
	if err != nil {
		return err
	}
	if keys == nil || len(keys.Addresses) == 0 {
		keys = new(lib.Proposers)
		keys.Addresses = [][]byte{{}, {}, {}, {}, {}}
	}
	index := s.Height() % 5
	keys.Addresses[index] = address
	return s.SetLastProposers(keys)
}

// GetLastProposers() returns the last Proposer addresses saved in the state
func (s *StateMachine) GetLastProposers() (*lib.Proposers, lib.ErrorI) {
	bz, err := s.Get(types.LastProposersPrefix())
	if err != nil {
		return nil, err
	}
	ptr := new(lib.Proposers)
	if err = lib.Unmarshal(bz, ptr); err != nil {
		return nil, err
	}
	return ptr, nil
}

// SetLastProposers() saves the last Proposer addresses in the state
func (s *StateMachine) SetLastProposers(keys *lib.Proposers) lib.ErrorI {
	bz, err := lib.Marshal(keys)
	if err != nil {
		return err
	}
	return s.Set(types.LastProposersPrefix(), bz)
}
