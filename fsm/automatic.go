package fsm

import (
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
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
	return nil
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
