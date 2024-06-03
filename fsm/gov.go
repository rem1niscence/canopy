package fsm

import (
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"google.golang.org/protobuf/proto"
)

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

func (s *StateMachine) SetParamsCons(c *types.ConsensusParams) lib.ErrorI {
	return s.setParams(types.ParamSpaceCons, c)
}

func (s *StateMachine) SetParamsVal(v *types.ValidatorParams) lib.ErrorI {
	return s.setParams(types.ParamSpaceVal, v)
}

func (s *StateMachine) SetParamsGov(g *types.GovernanceParams) lib.ErrorI {
	return s.setParams(types.ParamSpaceGov, g)
}

func (s *StateMachine) SetParamsFee(f *types.FeeParams) lib.ErrorI {
	return s.setParams(types.ParamSpaceFee, f)
}

func (s *StateMachine) setParams(space string, p proto.Message) lib.ErrorI {
	bz, err := lib.Marshal(p)
	if err != nil {
		return err
	}
	return s.Set(types.KeyForParams(space), bz)
}

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

func (s *StateMachine) GetParamsCons() (ptr *types.ConsensusParams, err lib.ErrorI) {
	ptr = new(types.ConsensusParams)
	err = s.getParams(types.ParamSpaceCons, ptr, types.ErrEmptyConsParams)
	return
}

func (s *StateMachine) GetParamsVal() (ptr *types.ValidatorParams, err lib.ErrorI) {
	ptr = new(types.ValidatorParams)
	err = s.getParams(types.ParamSpaceVal, ptr, types.ErrEmptyValParams)
	return
}

func (s *StateMachine) GetParamsGov() (ptr *types.GovernanceParams, err lib.ErrorI) {
	ptr = new(types.GovernanceParams)
	err = s.getParams(types.ParamSpaceGov, ptr, types.ErrEmptyGovParams)
	return
}

func (s *StateMachine) GetParamsFee() (ptr *types.FeeParams, err lib.ErrorI) {
	ptr = new(types.FeeParams)
	err = s.getParams(types.ParamSpaceFee, ptr, types.ErrEmptyFeeParams)
	return
}

func (s *StateMachine) ApproveProposal(msg types.Proposal) lib.ErrorI {
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
		proposals := make(types.Proposals)
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
