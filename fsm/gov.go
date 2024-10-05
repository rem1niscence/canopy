package fsm

import (
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"google.golang.org/protobuf/proto"
)

// UpdateParam() updates a governance parameter keyed by space and name
func (s *StateMachine) UpdateParam(space, paramName string, value proto.Message) (err lib.ErrorI) {
	var sp types.ParamSpace
	switch space {
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
	switch v := value.(type) {
	case *lib.UInt64Wrapper:
		return sp.SetUint64(paramName, v.Value)
	case *lib.StringWrapper:
		return sp.SetString(paramName, v.Value)
	default:
		return types.ErrUnknownParamType(value)
	}
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

// ApproveProposal() validates a 'GovProposal' message (ex. MsgChangeParameter)
// - checks message sent between start height and end height
// - if APPROVE_ALL set or proposal on the APPROVE_LIST then no error
// - else return ErrRejectProposal
func (s *StateMachine) ApproveProposal(msg types.GovProposal) lib.ErrorI {
	if msg.GetStartHeight() <= s.Height() && s.Height() <= msg.GetEndHeight() {
		return types.ErrRejectProposal()
	}
	switch s.proposeVoteConfig {
	case types.ProposalApproveList:
		bz, err := lib.Marshal(msg)
		if err != nil {
			return err
		}
		if bz == nil {
			return types.ErrRejectProposal()
		}
		proposals := make(types.GovProposals)
		if err = proposals.NewFromFile(s.Config.DataDirPath); err != nil {
			return err
		}
		if _, ok := proposals[crypto.HashString(bz)]; !ok {
			return types.ErrRejectProposal()
		}
		return nil
	case types.RejectAllProposals:
		return types.ErrRejectProposal()
	default:
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
