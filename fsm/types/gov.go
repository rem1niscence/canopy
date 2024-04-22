package types

import (
	"encoding/json"
	"github.com/ginchuco/ginchu/lib"
	"google.golang.org/protobuf/proto"
)

const (
	ParamPrefixCons = "/c/"
	ParamPrefixVal  = "/v/"
	ParamPrefixFee  = "/f/"
	ParamPrefixGov  = "/g/"

	ParamSpaceCons = "consensus"
	ParamSpaceVal  = "validator"
	ParamSpaceFee  = "fee"
	ParamSpaceGov  = "governance"
)

const (
	AcceptAllProposals  = ProposalVoteConfig_ACCEPT_ALL
	ProposalApproveList = ProposalVoteConfig_APPROVE_LIST
	RejectAllProposals  = ProposalVoteConfig_REJECT_ALL
)

type ParamSpace interface {
	Validate() lib.ErrorI
	SetString(paramName string, value string) lib.ErrorI
	SetUint64(paramName string, value uint64) lib.ErrorI
}

type Proposal interface {
	proto.Message
	GetStartHeight() uint64
	GetEndHeight() uint64
}

func IsValidParamSpace(space string) bool {
	switch space {
	case ParamSpaceCons, ParamSpaceVal, ParamSpaceFee, ParamSpaceGov:
		return true
	default:
		return false
	}
}

func prefixForParamSpace(space string) string {
	switch space {
	case ParamSpaceCons:
		return ParamPrefixCons
	case ParamSpaceVal:
		return ParamPrefixVal
	case ParamSpaceFee:
		return ParamPrefixFee
	case ParamSpaceGov:
		return ParamPrefixGov
	default:
		panic("unknown param space")
	}
}

func (x *Params) Validate() lib.ErrorI {
	if err := x.Consensus.Validate(); err != nil {
		return err
	}
	if err := x.Fee.Validate(); err != nil {
		return err
	}
	if err := x.Validator.Validate(); err != nil {
		return err
	}
	return x.Governance.Validate()
}

// consensus param space

const (
	ParamBlockSize       = "block_size"
	ParamProtocolVersion = "protocol_version"
)

var _ ParamSpace = &ConsensusParams{}

func (x *ConsensusParams) Validate() lib.ErrorI {
	if x.BlockSize == 0 {
		return ErrInvalidParam(ParamBlockSize)
	}
	if _, err := x.ParseProtocolVersion(); err != nil {
		return err
	}
	return nil
}

func (x *ConsensusParams) SetUint64(paramName string, value uint64) lib.ErrorI {
	switch paramName {
	case ParamBlockSize:
		x.BlockSize = value
	default:
		return ErrUnknownParam()
	}
	return x.Validate()
}

func (x *ConsensusParams) SetString(paramName string, value string) lib.ErrorI {
	switch paramName {
	case ParamProtocolVersion:
		if err := CheckProtocolVersion(value); err != nil {
			return err
		}
		x.ProtocolVersion = value
	default:
		return ErrUnknownParam()
	}
	return x.Validate()
}

func (x *ConsensusParams) ParseProtocolVersion() (*ProtocolVersion, lib.ErrorI) {
	ptr := &ProtocolVersion{}
	if err := json.Unmarshal([]byte(x.ProtocolVersion), ptr); err != nil {
		return nil, lib.ErrUnmarshal(err)
	}
	return ptr, nil
}

func CheckProtocolVersion(v string) lib.ErrorI {
	ptr := &ProtocolVersion{}
	if err := json.Unmarshal([]byte(v), ptr); err != nil {
		return lib.ErrUnmarshal(err)
	}
	// TODO more validation?
	return nil
}

func NewProtocolVersion(height uint64, version uint64) (string, lib.ErrorI) {
	bz, err := json.Marshal(ProtocolVersion{Height: height, Version: version})
	if err != nil {
		return "", lib.ErrMarshal(err)
	}
	return string(bz), nil
}

// validator param space

var _ ParamSpace = &ValidatorParams{}

const (
	ParamValidatorMinStake                  = "validator_min_stake"
	ParamValidatorMaxCount                  = "validator_max_count"
	ParamValidatorUnstakingBlocks           = "validator_unstaking_blocks"
	ParamValidatorMaxPauseBlocks            = "validator_max_pause_blocks"
	ParamValidatorMaxEvidenceAgeInBlocks    = "validator_max_evidence_age_in_blocks"
	ParamValidatorBadProposeSlashPercentage = "validator_bad_propose_slash_percentage"
	ParamValidatorNonSignSlashPercentage    = "validator_non_sign_slash_percentage"
	ParamValidatorMaxNonSign                = "validator_max_missed_sign"
	ParamValidatorNonSignWindow             = "validator_non_sign_window"
	ParamValidatorDoubleSignSlashPercentage = "validator_double_sign_slash_percentage"
	ParamValidatorProposerPercentageOfFees  = "validator_proposer_percentage_of_fees"
	ParamValidatorBlockReward               = "validator_block_reward"
)

func (x *ValidatorParams) Validate() lib.ErrorI {
	if x.ValidatorMinStake == 0 {
		return ErrInvalidParam(ParamValidatorMinStake)
	}
	if x.ValidatorMaxCount == 0 {
		return ErrInvalidParam(ParamValidatorMaxCount)
	}
	if x.ValidatorUnstakingBlocks == 0 {
		return ErrInvalidParam(ParamValidatorUnstakingBlocks)
	}
	if x.ValidatorMaxPauseBlocks == 0 {
		return ErrInvalidParam(ParamValidatorMaxPauseBlocks)
	}
	if x.ValidatorMaxEvidenceAgeInBlocks == 0 {
		return ErrInvalidParam(ParamValidatorMaxEvidenceAgeInBlocks)
	}
	if x.ValidatorBadProposalSlashPercentage > 100 {
		return ErrInvalidParam(ParamValidatorBadProposeSlashPercentage)
	}
	if x.ValidatorNonSignSlashPercentage > 100 {
		return ErrInvalidParam(ParamValidatorNonSignSlashPercentage)
	}
	if x.ValidatorNonSignWindow == 0 {
		return ErrInvalidParam(ParamValidatorNonSignWindow)
	}
	if x.ValidatorMaxNonSign < x.ValidatorNonSignWindow {
		return ErrInvalidParam(ParamValidatorMaxNonSign)
	}
	if x.ValidatorDoubleSignSlashPercentage > 100 {
		return ErrInvalidParam(ParamValidatorDoubleSignSlashPercentage)
	}
	if x.ValidatorProposerPercentageOfFees > 100 {
		return ErrInvalidParam(ParamValidatorProposerPercentageOfFees)
	}
	if x.ValidatorBlockReward == 0 {
		return ErrInvalidParam(ParamValidatorBlockReward)
	}
	return nil
}

func (x *ValidatorParams) SetUint64(paramName string, value uint64) lib.ErrorI {
	switch paramName {
	case ParamValidatorUnstakingBlocks:
		x.ValidatorUnstakingBlocks = value
	case ParamValidatorMaxCount:
		x.ValidatorMaxCount = value
	case ParamValidatorMaxPauseBlocks:
		x.ValidatorMaxPauseBlocks = value
	case ParamValidatorMaxEvidenceAgeInBlocks:
		x.ValidatorMaxEvidenceAgeInBlocks = value
	case ParamValidatorBadProposeSlashPercentage:
		x.ValidatorBadProposalSlashPercentage = value
	case ParamValidatorNonSignWindow:
		x.ValidatorNonSignWindow = value
	case ParamValidatorMaxNonSign:
		x.ValidatorMaxNonSign = value
	case ParamValidatorNonSignSlashPercentage:
		x.ValidatorNonSignSlashPercentage = value
	case ParamValidatorDoubleSignSlashPercentage:
		x.ValidatorDoubleSignSlashPercentage = value
	case ParamValidatorProposerPercentageOfFees:
		x.ValidatorProposerPercentageOfFees = value
	case ParamValidatorMinStake:
		x.ValidatorMinStake = value
	case ParamValidatorBlockReward:
		x.ValidatorBlockReward = value
	default:
		return ErrUnknownParam()
	}
	return x.Validate()
}

func (x *ValidatorParams) SetString(_ string, _ string) lib.ErrorI {
	return ErrUnknownParam()
}

// fee param space

var _ ParamSpace = &FeeParams{}

const (
	ParamMessageSendFee            = "message_send_fee"
	ParamMessageStakeFee           = "message_stake_fee"
	ParamMessageEditStakeFee       = "message_edit_stake_fee"
	ParamMessageUnstakeFee         = "message_unstake_fee"
	ParamMessagePauseFee           = "message_pause_fee"
	ParamMessageUnpauseFee         = "message_unpause_fee"
	ParamMessageChangeParameterFee = "message_change_parameter_fee"
	ParamMessageDoubleSignFee      = "message_double_sign_fee"
)

func (x *FeeParams) Validate() lib.ErrorI {
	if x.MessageSendFee == 0 {
		return ErrInvalidParam(ParamMessageSendFee)
	}
	if x.MessageStakeFee == 0 {
		return ErrInvalidParam(ParamMessageStakeFee)
	}
	if x.MessageEditStakeFee == 0 {
		return ErrInvalidParam(ParamMessageEditStakeFee)
	}
	if x.MessageUnstakeFee == 0 {
		return ErrInvalidParam(ParamMessageUnstakeFee)
	}
	if x.MessagePauseFee == 0 {
		return ErrInvalidParam(ParamMessagePauseFee)
	}
	if x.MessageUnpauseFee == 0 {
		return ErrInvalidParam(ParamMessageUnpauseFee)
	}
	if x.MessageChangeParameterFee == 0 {
		return ErrInvalidParam(ParamMessageChangeParameterFee)
	}
	if x.MessageDoubleSignFee == 0 {
		return ErrInvalidParam(ParamMessageDoubleSignFee)
	}
	return nil
}

func (x *FeeParams) SetString(_ string, _ string) lib.ErrorI {
	return ErrUnknownParam()
}

func (x *FeeParams) SetUint64(paramName string, value uint64) lib.ErrorI {
	switch paramName {
	case ParamMessageSendFee:
		x.MessageSendFee = value
	case ParamMessageStakeFee:
		x.MessageStakeFee = value
	case ParamMessageEditStakeFee:
		x.MessageEditStakeFee = value
	case ParamMessageUnstakeFee:
		x.MessageUnstakeFee = value
	case ParamMessagePauseFee:
		x.MessagePauseFee = value
	case ParamMessageUnpauseFee:
		x.MessageUnpauseFee = value
	case ParamMessageChangeParameterFee:
		x.MessageChangeParameterFee = value
	case ParamMessageDoubleSignFee:
		x.MessageDoubleSignFee = value
	default:
		return ErrUnknownParam()
	}
	return x.Validate()
}

// governance param space

const (
	ParamDAORewardPercentage = "dao_reward_percentage"
)

var _ ParamSpace = &GovernanceParams{}

func (x *GovernanceParams) Validate() lib.ErrorI {
	if x.DaoRewardPercentage > 100 {
		return ErrInvalidParam(ParamDAORewardPercentage)
	}
	return nil
}

func (x *GovernanceParams) SetUint64(paramName string, value uint64) lib.ErrorI {
	switch paramName {
	case ParamDAORewardPercentage:
		x.DaoRewardPercentage = value
	default:
		return ErrUnknownParam()
	}
	return x.Validate()
}

func (x *GovernanceParams) SetString(_ string, _ string) lib.ErrorI {
	return ErrUnknownParam()
}
