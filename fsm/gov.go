package fsm

import (
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"google.golang.org/protobuf/proto"
)

// UpdateParam() updates a governance parameter keyed by space and name
func (s *StateMachine) UpdateParam(paramSpace, paramName string, value proto.Message) (err lib.ErrorI) {
	// save the previous parameters to check for updates
	previousParams, err := s.GetParams()
	if err != nil {
		return
	}
	// retrieve the space from the string
	var sp types.ParamSpace
	switch paramSpace {
	case types.ParamSpaceCons:
		sp, err = s.GetParamsCons()
	case types.ParamSpaceVal:
		sp, err = s.GetParamsVal()
	case types.ParamSpaceFee:
		sp, err = s.GetParamsFee()
	case types.ParamSpaceGov:
		sp, err = s.GetParamsGov()
	default:
		return types.ErrUnknownParamSpace()
	}
	if err != nil {
		return err
	}
	// set the value based on the type
	switch v := value.(type) {
	case *lib.UInt64Wrapper:
		err = sp.SetUint64(paramName, v.Value)
	case *lib.StringWrapper:
		err = sp.SetString(paramName, v.Value)
	default:
		return types.ErrUnknownParamType(value)
	}
	if err != nil {
		return err
	}
	// set the param space back in state
	switch paramSpace {
	case types.ParamSpaceCons:
		return s.SetParamsCons(sp.(*types.ConsensusParams))
	case types.ParamSpaceVal:
		return s.SetParamsVal(sp.(*types.ValidatorParams))
	case types.ParamSpaceFee:
		return s.SetParamsFee(sp.(*types.FeeParams))
	case types.ParamSpaceGov:
		return s.SetParamsGov(sp.(*types.GovernanceParams))
	}
	// adjust the state if necessary
	return s.ConformStateToParamUpdate(previousParams)
}

// ConformStateToParamUpdate() ensures the state does not violate the new values of the governance parameters
// NOTE: at the moment only MaxCommitteeSize requires an adjustment with the exception of MinSellOrderSize which
// is purposefully allowed to violate new updates
func (s *StateMachine) ConformStateToParamUpdate(previousParams *types.Params) lib.ErrorI {
	// retrieve the params from state
	params, err := s.GetParams()
	if err != nil {
		return err
	}
	// check for a change in MaxCommitteeSize
	if previousParams.Validator.ValidatorMaxCommitteeSize <= params.Validator.ValidatorMaxCommittees {
		return nil
	}
	// shrinking MaxCommitteeSize must be immediately enforced to ensure no 'grandfathered' in violators
	maxCommitteeSize := int(params.Validator.ValidatorMaxCommittees)
	// maintain a counter for pseudorandom removal of the 'chain ids'
	var idx int
	// for each validator, remove the excess ids in a pseudorandom fashion
	return s.IterateAndExecute(types.ValidatorPrefix(), func(_, value []byte) lib.ErrorI {
		// convert bytes into a validator object
		v, e := s.unmarshalValidator(value)
		if e != nil {
			return e
		}
		// check the number of committees for this validator and see if it's above the maximum
		numCommittees := len(v.Committees)
		if numCommittees <= maxCommitteeSize {
			return nil
		}
		// create a variable to hold a copy of the new committees
		newCommittees := make([]uint64, len(v.Committees))
		// copy the committees
		copy(newCommittees, v.Committees)
		// if it's above the maximum allowed amount
		for ; numCommittees > maxCommitteeSize; numCommittees-- {
			// calculate a pseudorandom index
			pseudoRandomIndex := idx % numCommittees
			// remove the pseudorandom index from committees
			newCommittees = append(newCommittees[:pseudoRandomIndex], newCommittees[pseudoRandomIndex+1:]...)
			// increment the index to further the 'pseuorandom' property
			idx++
		}
		// update the committees or delegations
		if !v.Delegate {
			if err = s.UpdateCommittees(crypto.NewAddress(v.Address), v, v.StakedAmount, newCommittees); err != nil {
				return err
			}
		} else {
			if err = s.UpdateDelegations(crypto.NewAddress(v.Address), v, v.StakedAmount, newCommittees); err != nil {
				return err
			}
		}
		// update the validator and its committees
		v.Committees = newCommittees
		// set the validator back into state
		return s.SetValidator(v)
	})
}

// SetParams() writes an entire Params object into state
func (s *StateMachine) SetParams(p *types.Params) lib.ErrorI {
	if err := s.SetParamsCons(p.GetConsensus()); err != nil {
		return err
	}
	if err := s.SetParamsVal(p.GetValidator()); err != nil {
		return err
	}
	if err := s.SetParamsFee(p.GetFee()); err != nil {
		return err
	}
	return s.SetParamsGov(p.GetGovernance())
}

// SetParamsCons() sets Consensus params into state
func (s *StateMachine) SetParamsCons(c *types.ConsensusParams) lib.ErrorI {
	return s.setParams(types.ParamSpaceCons, c)
}

// SetParamsVal() sets Validator params into state
func (s *StateMachine) SetParamsVal(v *types.ValidatorParams) lib.ErrorI {
	return s.setParams(types.ParamSpaceVal, v)
}

// SetParamsGov() sets governance params into state
func (s *StateMachine) SetParamsGov(g *types.GovernanceParams) lib.ErrorI {
	return s.setParams(types.ParamSpaceGov, g)
}

// SetParamsFee() sets fee params into state
func (s *StateMachine) SetParamsFee(f *types.FeeParams) lib.ErrorI {
	return s.setParams(types.ParamSpaceFee, f)
}

// setParams() converts the ParamSpace into bytes and sets them in state
func (s *StateMachine) setParams(space string, p proto.Message) lib.ErrorI {
	bz, err := lib.Marshal(p)
	if err != nil {
		return err
	}
	return s.Set(types.KeyForParams(space), bz)
}

// GetParams() returns the aggregated ParamSpaces in a single Params object
func (s *StateMachine) GetParams() (*types.Params, lib.ErrorI) {
	cons, err := s.GetParamsCons()
	if err != nil {
		return nil, err
	}
	val, err := s.GetParamsVal()
	if err != nil {
		return nil, err
	}
	fee, err := s.GetParamsFee()
	if err != nil {
		return nil, err
	}
	gov, err := s.GetParamsGov()
	if err != nil {
		return nil, err
	}
	return &types.Params{
		Consensus:  cons,
		Validator:  val,
		Fee:        fee,
		Governance: gov,
	}, nil
}

// GetParamsCons() returns the current state of the governance params in the Consensus space
func (s *StateMachine) GetParamsCons() (ptr *types.ConsensusParams, err lib.ErrorI) {
	ptr = new(types.ConsensusParams)
	err = s.getParams(types.ParamSpaceCons, ptr, types.ErrEmptyConsParams)
	return
}

// GetParamsVal() returns the current state of the governance params in the Validator space
func (s *StateMachine) GetParamsVal() (ptr *types.ValidatorParams, err lib.ErrorI) {
	ptr = new(types.ValidatorParams)
	err = s.getParams(types.ParamSpaceVal, ptr, types.ErrEmptyValParams)
	return
}

// GetParamsGov() returns the current state of the governance params in the Governance space
func (s *StateMachine) GetParamsGov() (ptr *types.GovernanceParams, err lib.ErrorI) {
	ptr = new(types.GovernanceParams)
	err = s.getParams(types.ParamSpaceGov, ptr, types.ErrEmptyGovParams)
	return
}

// GetParamsFee() returns the current state of the governance params in the Fee space
func (s *StateMachine) GetParamsFee() (ptr *types.FeeParams, err lib.ErrorI) {
	ptr = new(types.FeeParams)
	err = s.getParams(types.ParamSpaceFee, ptr, types.ErrEmptyFeeParams)
	return
}

// ApproveProposal() validates a 'GovProposal' message (ex. MsgChangeParameter or MsgDAOTransfer)
// - checks message sent between start height and end height
// - if APPROVE_ALL set or proposal on the APPROVE_LIST then no error
// - else return ErrRejectProposal
func (s *StateMachine) ApproveProposal(msg types.GovProposal) lib.ErrorI {
	if s.Height() < msg.GetStartHeight() || s.Height() > msg.GetEndHeight() {
		return types.ErrRejectProposal()
	}
	// handle the proposal based on config
	switch s.proposeVoteConfig {
	case types.ProposalApproveList: // approve based on list
		// convert the msg into bytes
		bz, err := lib.Marshal(msg)
		if err != nil {
			return err
		}
		// read the 'approve list' from the datadirectory
		proposals := make(types.GovProposals)
		if err = proposals.NewFromFile(s.Config.DataDirPath); err != nil {
			return err
		}
		// check on this specific message for explicit rejection or complete omission
		if value, ok := proposals[crypto.HashString(bz)]; !ok || !value.Approve {
			return types.ErrRejectProposal()
		}
		return nil
	case types.RejectAllProposals: // reject all
		return types.ErrRejectProposal()
	default: // approve all
		return nil
	}
}

// getParams() is a generic helper function loads the params for a specific ParamSpace into a ptr object
func (s *StateMachine) getParams(space string, ptr any, emptyErr func() lib.ErrorI) lib.ErrorI {
	bz, err := s.Get(types.KeyForParams(space))
	if err != nil {
		return err
	}
	if bz == nil {
		return emptyErr()
	}
	if err = lib.Unmarshal(bz, ptr); err != nil {
		return err
	}
	return nil
}
