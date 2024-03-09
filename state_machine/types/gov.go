package types

import (
	"github.com/ginchuco/ginchu/types"
	"strings"
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

	ParamKeywordOwner = "_owner"
)

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

// consensus param space

const (
	ParamBlockSize = "block_size"
)

var _ types.ParamSpace = &ConsensusParams{}

func (x *ConsensusParams) SetUint64(paramName string, value uint64) types.ErrorI {
	switch paramName {
	case ParamBlockSize:
		x.BlockSize.Value = value
	default:
		return ErrUnknownParam()
	}
	return nil
}

func (x *ConsensusParams) SetOwner(paramName string, owner string) types.ErrorI {
	name := stripKeywordOwnerFromParamName(paramName)
	switch name {
	case ParamBlockSize:
		x.BlockSize.Owner = owner
	default:
		return ErrUnknownParam()
	}
	return nil
}

func (x *ConsensusParams) SetString(_ string, _ string) types.ErrorI {
	return ErrUnknownParam()
}

// validator param space

var _ types.ParamSpace = &ValidatorParams{}

const (
	ParamValidatorMinimumStake               = "validator_minimum_stake"
	ParamValidatorUnstakingBlocks            = "validator_unstaking_blocks"
	ParamValidatorMinimumPauseBlocks         = "validator_minimum_pause_blocks"
	ParamValidatorMaximumPauseBlocks         = "validator_maximum_pause_blocks"
	ParamValidatorMaximumMissedBlocks        = "validator_maximum_missed_blocks"
	ParamValidatorMaxEvidenceAgeInBlocks     = "validator_maximum_evidence_age_in_blocks"
	ParamValidatorMissedBlocksBurnPercentage = "validator_missed_blocks_burn_percentage"
	ParamValidatorDoubleSignBurnPercentage   = "validator_double_sign_burn_percentage"
	ParamValidatorProposerPercentageOfFees   = "validator_proposer_percentage_of_fees"
)

func (x *ValidatorParams) SetUint64(paramName string, value uint64) types.ErrorI {
	switch paramName {
	case ParamValidatorUnstakingBlocks:
		x.ValidatorUnstakingBlocks.Value = value
	case ParamValidatorMinimumPauseBlocks:
		x.ValidatorMinimumPauseBlocks.Value = value
	case ParamValidatorMaximumPauseBlocks:
		x.ValidatorMaxPauseBlocks.Value = value
	case ParamValidatorMaximumMissedBlocks:
		x.ValidatorMaximumMissedBlocks.Value = value
	case ParamValidatorMaxEvidenceAgeInBlocks:
		x.ValidatorMaxEvidenceAgeInBlocks.Value = value
	case ParamValidatorMissedBlocksBurnPercentage:
		x.ValidatorMissedBlocksBurnPercentage.Value = value
	case ParamValidatorDoubleSignBurnPercentage:
		x.ValidatorDoubleSignBurnPercentage.Value = value
	case ParamValidatorProposerPercentageOfFees:
		x.ValidatorProposerPercentageOfFees.Value = value
	default:
		return ErrUnknownParam()
	}
	return nil
}

func (x *ValidatorParams) SetString(paramName string, value string) types.ErrorI {
	switch paramName {
	case ParamValidatorMinimumStake:
		x.ValidatorMinimumStake.Value = value
	default:
		return ErrUnknownParam()
	}
	return nil
}

func (x *ValidatorParams) SetOwner(paramName string, owner string) types.ErrorI {
	name := stripKeywordOwnerFromParamName(paramName)
	switch name {
	case ParamValidatorMinimumStake:
		x.ValidatorMinimumStake.Owner = owner
	case ParamValidatorUnstakingBlocks:
		x.ValidatorUnstakingBlocks.Owner = owner
	case ParamValidatorMinimumPauseBlocks:
		x.ValidatorMinimumPauseBlocks.Owner = owner
	case ParamValidatorMaximumPauseBlocks:
		x.ValidatorMaxPauseBlocks.Owner = owner
	case ParamValidatorMaximumMissedBlocks:
		x.ValidatorMaximumMissedBlocks.Owner = owner
	case ParamValidatorMaxEvidenceAgeInBlocks:
		x.ValidatorMaxEvidenceAgeInBlocks.Owner = owner
	case ParamValidatorMissedBlocksBurnPercentage:
		x.ValidatorMissedBlocksBurnPercentage.Owner = owner
	case ParamValidatorDoubleSignBurnPercentage:
		x.ValidatorDoubleSignBurnPercentage.Owner = owner
	case ParamValidatorProposerPercentageOfFees:
		x.ValidatorProposerPercentageOfFees.Owner = owner
	default:
		return ErrUnknownParam()
	}
	return nil
}

// fee param space

var _ types.ParamSpace = &FeeParams{}

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

func (x *FeeParams) SetString(paramName string, value string) types.ErrorI {
	switch paramName {
	case ParamMessageSendFee:
		x.MessageSendFee.Value = value
	case ParamMessageStakeFee:
		x.MessageStakeFee.Value = value
	case ParamMessageEditStakeFee:
		x.MessageEditStakeFee.Value = value
	case ParamMessageUnstakeFee:
		x.MessageUnstakeFee.Value = value
	case ParamMessagePauseFee:
		x.MessagePauseFee.Value = value
	case ParamMessageUnpauseFee:
		x.MessageUnpauseFee.Value = value
	case ParamMessageChangeParameterFee:
		x.MessageChangeParameterFee.Value = value
	case ParamMessageDoubleSignFee:
		x.MessageDoubleSignFee.Value = value
	default:
		return ErrUnknownParam()
	}
	return nil
}

func (x *FeeParams) SetOwner(paramName string, owner string) types.ErrorI {
	name := stripKeywordOwnerFromParamName(paramName)
	switch name {
	case ParamMessageSendFee:
		x.MessageSendFee.Owner = owner
	case ParamMessageStakeFee:
		x.MessageStakeFee.Owner = owner
	case ParamMessageEditStakeFee:
		x.MessageEditStakeFee.Owner = owner
	case ParamMessageUnstakeFee:
		x.MessageUnstakeFee.Owner = owner
	case ParamMessagePauseFee:
		x.MessagePauseFee.Owner = owner
	case ParamMessageUnpauseFee:
		x.MessageUnpauseFee.Owner = owner
	case ParamMessageChangeParameterFee:
		x.MessageChangeParameterFee.Owner = owner
	case ParamMessageDoubleSignFee:
		x.MessageDoubleSignFee.Owner = owner
	default:
		return ErrUnknownParam()
	}
	return nil
}

func (x *FeeParams) SetUint64(_ string, _ uint64) types.ErrorI {
	return ErrUnknownParam()
}

// governance param space

const (
	ParamACLOwner = "acl"
)

var _ types.ParamSpace = &GovernanceParams{}

func (x *GovernanceParams) SetOwner(paramName string, owner string) types.ErrorI {
	switch paramName {
	case ParamACLOwner:
		x.AclOwner = owner
	default:
		return ErrUnknownParam()
	}
	return nil
}

func (x *GovernanceParams) SetUint64(_ string, _ uint64) types.ErrorI {
	return ErrUnknownParam()
}

func (x *GovernanceParams) SetString(paramName string, value string) types.ErrorI {
	return ErrUnknownParam()
}

func stripKeywordOwnerFromParamName(pn string) string {
	return strings.Replace(pn, ParamKeywordOwner, "", -1)
}
