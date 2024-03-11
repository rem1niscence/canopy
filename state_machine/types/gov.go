package types

import (
	"encoding/json"
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
	ParamBlockSize       = "block_size"
	ParamProtocolVersion = "protocol_version"
)

var _ types.ParamSpace = &ConsensusParams{}

func (x *ConsensusParams) SetUint64(address string, paramName string, value uint64) types.ErrorI {
	switch paramName {
	case ParamBlockSize:
		if address != x.BlockSize.Owner {
			return ErrUnauthorizedParamChange()
		}
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
	case ParamProtocolVersion:
		x.ProtocolVersion.Owner = owner
	default:
		return ErrUnknownParam()
	}
	return nil
}

func (x *ConsensusParams) SetString(address string, paramName string, value string) types.ErrorI {
	switch paramName {
	case ParamProtocolVersion:
		if address != x.ProtocolVersion.Owner {
			return ErrUnauthorizedParamChange()
		}
		if err := CheckProtocolVersion(value); err != nil {
			return err
		}
		x.ProtocolVersion.Value = value
	default:
		return ErrUnknownParam()
	}
	return nil
}

func (x *ConsensusParams) ParseProtocolVersion() (*ProtocolVersion, types.ErrorI) {
	ptr := &ProtocolVersion{}
	if err := json.Unmarshal([]byte(x.ProtocolVersion.Value), ptr); err != nil {
		return nil, ErrUnmarshal(err)
	}
	return ptr, nil
}

func CheckProtocolVersion(v string) types.ErrorI {
	ptr := &ProtocolVersion{}
	if err := json.Unmarshal([]byte(v), ptr); err != nil {
		return ErrUnmarshal(err)
	}
	// TODO more validation?
	return nil
}

func NewProtocolVersion(height uint64, version uint64) (string, types.ErrorI) {
	bz, err := json.Marshal(ProtocolVersion{Height: height, Version: version})
	if err != nil {
		return "", ErrMarshal(err)
	}
	return string(bz), nil
}

// validator param space

var _ types.ParamSpace = &ValidatorParams{}

const (
	ParamValidatorMinimumStake                = "validator_minimum_stake"
	ParamValidatorUnstakingBlocks             = "validator_unstaking_blocks"
	ParamValidatorMinimumPauseBlocks          = "validator_minimum_pause_blocks"
	ParamValidatorMaximumPauseBlocks          = "validator_maximum_pause_blocks"
	ParamValidatorMaximumMissedBlocks         = "validator_maximum_missed_blocks"
	ParamValidatorMaxEvidenceAgeInBlocks      = "validator_maximum_evidence_age_in_blocks"
	ParamValidatorMissedBlocksSlashPercentage = "validator_missed_blocks_slash_percentage"
	ParamValidatorDoubleSignSlashPercentage   = "validator_double_sign_slash_percentage"
	ParamValidatorDoubleSignReporterReward    = "validator_double_sign_reporter_reward"
	ParamValidatorProposerPercentageOfFees    = "validator_proposer_percentage_of_fees"
	ParamValidatorProposerBlockReward         = "validator_proposer_block_reward"
)

func (x *ValidatorParams) SetUint64(address string, paramName string, value uint64) types.ErrorI {
	switch paramName {
	case ParamValidatorUnstakingBlocks:
		if address != x.ValidatorUnstakingBlocks.Owner {
			return ErrUnauthorizedParamChange()
		}
		x.ValidatorUnstakingBlocks.Value = value
	case ParamValidatorMinimumPauseBlocks:
		if address != x.ValidatorMinimumPauseBlocks.Owner {
			return ErrUnauthorizedParamChange()
		}
		x.ValidatorMinimumPauseBlocks.Value = value
	case ParamValidatorMaximumPauseBlocks:
		if address != x.ValidatorMaxPauseBlocks.Owner {
			return ErrUnauthorizedParamChange()
		}
		x.ValidatorMaxPauseBlocks.Value = value
	case ParamValidatorMaximumMissedBlocks:
		if address != x.ValidatorMaximumMissedBlocks.Owner {
			return ErrUnauthorizedParamChange()
		}
		x.ValidatorMaximumMissedBlocks.Value = value
	case ParamValidatorMaxEvidenceAgeInBlocks:
		if address != x.ValidatorMaxEvidenceAgeInBlocks.Owner {
			return ErrUnauthorizedParamChange()
		}
		x.ValidatorMaxEvidenceAgeInBlocks.Value = value
	case ParamValidatorMissedBlocksSlashPercentage:
		if address != x.ValidatorMissedBlocksSlashPercentage.Owner {
			return ErrUnauthorizedParamChange()
		}
		x.ValidatorMissedBlocksSlashPercentage.Value = value
	case ParamValidatorDoubleSignSlashPercentage:
		if address != x.ValidatorDoubleSignSlashPercentage.Owner {
			return ErrUnauthorizedParamChange()
		}
		x.ValidatorDoubleSignSlashPercentage.Value = value
	case ParamValidatorProposerPercentageOfFees:
		if address != x.ValidatorProposerPercentageOfFees.Owner {
			return ErrUnauthorizedParamChange()
		}
		x.ValidatorProposerPercentageOfFees.Value = value
	default:
		return ErrUnknownParam()
	}
	return nil
}

func (x *ValidatorParams) SetString(address string, paramName string, value string) types.ErrorI {
	switch paramName {
	case ParamValidatorMinimumStake:
		if address != x.ValidatorMinimumStake.Owner {
			return ErrUnauthorizedParamChange()
		}
		x.ValidatorMinimumStake.Value = value
	case ParamValidatorDoubleSignReporterReward:
		if address != x.ValidatorDoubleSignReporterReward.Owner {
			return ErrUnauthorizedParamChange()
		}
		x.ValidatorDoubleSignReporterReward.Value = value
	case ParamValidatorProposerBlockReward:
		if address != x.ValidatorProposerBlockReward.Owner {
			return ErrUnauthorizedParamChange()
		}
		x.ValidatorProposerBlockReward.Value = value
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
	case ParamValidatorMissedBlocksSlashPercentage:
		x.ValidatorMissedBlocksSlashPercentage.Owner = owner
	case ParamValidatorDoubleSignSlashPercentage:
		x.ValidatorDoubleSignSlashPercentage.Owner = owner
	case ParamValidatorDoubleSignReporterReward:
		x.ValidatorDoubleSignReporterReward.Owner = owner
	case ParamValidatorProposerPercentageOfFees:
		x.ValidatorProposerPercentageOfFees.Owner = owner
	case ParamValidatorProposerBlockReward:
		x.ValidatorProposerBlockReward.Owner = owner
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

func (x *FeeParams) SetString(address string, paramName string, value string) types.ErrorI {
	switch paramName {
	case ParamMessageSendFee:
		if address != x.MessageSendFee.Owner {
			return ErrUnauthorizedParamChange()
		}
		x.MessageSendFee.Value = value
	case ParamMessageStakeFee:
		if address != x.MessageStakeFee.Owner {
			return ErrUnauthorizedParamChange()
		}
		x.MessageStakeFee.Value = value
	case ParamMessageEditStakeFee:
		if address != x.MessageEditStakeFee.Owner {
			return ErrUnauthorizedParamChange()
		}
		x.MessageEditStakeFee.Value = value
	case ParamMessageUnstakeFee:
		if address != x.MessageUnstakeFee.Owner {
			return ErrUnauthorizedParamChange()
		}
		x.MessageUnstakeFee.Value = value
	case ParamMessagePauseFee:
		if address != x.MessagePauseFee.Owner {
			return ErrUnauthorizedParamChange()
		}
		x.MessagePauseFee.Value = value
	case ParamMessageUnpauseFee:
		if address != x.MessageUnpauseFee.Owner {
			return ErrUnauthorizedParamChange()
		}
		x.MessageUnpauseFee.Value = value
	case ParamMessageChangeParameterFee:
		if address != x.MessageChangeParameterFee.Owner {
			return ErrUnauthorizedParamChange()
		}
		x.MessageChangeParameterFee.Value = value
	case ParamMessageDoubleSignFee:
		if address != x.MessageDoubleSignFee.Owner {
			return ErrUnauthorizedParamChange()
		}
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

func (x *FeeParams) SetUint64(address string, paramName string, value uint64) types.ErrorI {
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

func (x *GovernanceParams) SetUint64(address string, paramName string, value uint64) types.ErrorI {
	return ErrUnknownParam()
}

func (x *GovernanceParams) SetString(address string, paramName string, value string) types.ErrorI {
	return ErrUnknownParam()
}

func stripKeywordOwnerFromParamName(pn string) string {
	return strings.Replace(pn, ParamKeywordOwner, "", -1)
}
