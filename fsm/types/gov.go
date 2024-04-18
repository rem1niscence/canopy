package types

import (
	"encoding/json"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
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

type ParamSpace interface {
	Validate() lib.ErrorI
	SetString(address string, paramName string, value string) lib.ErrorI
	SetUint64(address string, paramName string, value uint64) lib.ErrorI
	SetOwner(paramName string, owner string) lib.ErrorI
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
	if x.BlockSize.Value == 0 {
		return ErrInvalidParam(ParamBlockSize)
	}
	if _, err := x.ParseProtocolVersion(); err != nil {
		return err
	}
	return nil
}

func (x *ConsensusParams) SetUint64(address string, paramName string, value uint64) lib.ErrorI {
	switch paramName {
	case ParamBlockSize:
		if address != x.BlockSize.Owner {
			return ErrUnauthorizedParamChange()
		}
		x.BlockSize.Value = value
	default:
		return ErrUnknownParam()
	}
	return x.Validate()
}

func (x *ConsensusParams) SetOwner(paramName string, owner string) lib.ErrorI {
	name := stripKeywordOwnerFromParamName(paramName)
	switch name {
	case ParamBlockSize:
		x.BlockSize.Owner = owner
	case ParamProtocolVersion:
		x.ProtocolVersion.Owner = owner
	default:
		return ErrUnknownParam()
	}
	return x.Validate()
}

func (x *ConsensusParams) SetString(address string, paramName string, value string) lib.ErrorI {
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
	return x.Validate()
}

func (x *ConsensusParams) ParseProtocolVersion() (*ProtocolVersion, lib.ErrorI) {
	ptr := &ProtocolVersion{}
	if err := json.Unmarshal([]byte(x.ProtocolVersion.Value), ptr); err != nil {
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
	ParamValidatorFaultySignSlashPercentage = "validator_faulty_sign_slash_percentage"
	ParamValidatorNonSignSlashPercentage    = "validator_non_sign_slash_percentage"
	ParamValidatorMaxNonSign                = "validator_max_missed_sign"
	ParamValidatorNonSignWindow             = "validator_non_sign_window"
	ParamValidatorDoubleSignSlashPercentage = "validator_double_sign_slash_percentage"
	ParamValidatorDoubleSignReporterReward  = "validator_double_sign_reporter_reward"
	ParamValidatorProposerPercentageOfFees  = "validator_proposer_percentage_of_fees"
	ParamValidatorBlockReward               = "validator_block_reward"
)

func (x *ValidatorParams) Validate() lib.ErrorI {
	if i, err := lib.StringToBigInt(x.ValidatorMinStake.Value); err != nil || lib.BigIsZero(i) {
		return ErrInvalidParam(ParamValidatorMinStake)
	}
	if _, err := crypto.NewAddressFromString(x.ValidatorMinStake.Owner); err != nil {
		return ErrInvalidOwner(ParamValidatorMinStake)
	}
	if x.ValidatorMaxCount.Value == 0 {
		return ErrInvalidParam(ParamValidatorMaxCount)
	}
	if _, err := crypto.NewAddressFromString(x.ValidatorMaxCount.Owner); err != nil {
		return ErrInvalidOwner(ParamValidatorMaxCount)
	}
	if x.ValidatorUnstakingBlocks.Value == 0 {
		return ErrInvalidParam(ParamValidatorUnstakingBlocks)
	}
	if _, err := crypto.NewAddressFromString(x.ValidatorUnstakingBlocks.Owner); err != nil {
		return ErrInvalidOwner(ParamValidatorUnstakingBlocks)
	}
	if x.ValidatorMaxPauseBlocks.Value == 0 {
		return ErrInvalidParam(ParamValidatorMaxPauseBlocks)
	}
	if _, err := crypto.NewAddressFromString(x.ValidatorMaxPauseBlocks.Owner); err != nil {
		return ErrInvalidOwner(ParamValidatorMaxPauseBlocks)
	}
	if x.ValidatorMaxEvidenceAgeInBlocks.Value == 0 {
		return ErrInvalidParam(ParamValidatorMaxEvidenceAgeInBlocks)
	}
	if _, err := crypto.NewAddressFromString(x.ValidatorMaxEvidenceAgeInBlocks.Owner); err != nil {
		return ErrInvalidOwner(ParamValidatorMaxEvidenceAgeInBlocks)
	}
	if x.ValidatorBadProposalSlashPercentage.Value > 100 {
		return ErrInvalidParam(ParamValidatorBadProposeSlashPercentage)
	}
	if _, err := crypto.NewAddressFromString(x.ValidatorBadProposalSlashPercentage.Owner); err != nil {
		return ErrInvalidOwner(ParamValidatorBadProposeSlashPercentage)
	}
	if x.ValidatorFaultySignSlashPercentage.Value > 100 {
		return ErrInvalidParam(ParamValidatorFaultySignSlashPercentage)
	}
	if _, err := crypto.NewAddressFromString(x.ValidatorFaultySignSlashPercentage.Owner); err != nil {
		return ErrInvalidOwner(ParamValidatorFaultySignSlashPercentage)
	}
	if x.ValidatorNonSignSlashPercentage.Value > 100 {
		return ErrInvalidParam(ParamValidatorNonSignSlashPercentage)
	}
	if _, err := crypto.NewAddressFromString(x.ValidatorNonSignSlashPercentage.Owner); err != nil {
		return ErrInvalidOwner(ParamValidatorNonSignSlashPercentage)
	}
	if x.ValidatorNonSignWindow.Value == 0 {
		return ErrInvalidParam(ParamValidatorNonSignWindow)
	}
	if _, err := crypto.NewAddressFromString(x.ValidatorNonSignWindow.Owner); err != nil {
		return ErrInvalidOwner(ParamValidatorNonSignWindow)
	}
	if x.ValidatorMaxNonSign.Value < x.ValidatorNonSignWindow.Value {
		return ErrInvalidParam(ParamValidatorMaxNonSign)
	}
	if _, err := crypto.NewAddressFromString(x.ValidatorMaxNonSign.Owner); err != nil {
		return ErrInvalidOwner(ParamValidatorMaxNonSign)
	}
	if x.ValidatorDoubleSignSlashPercentage.Value > 100 {
		return ErrInvalidParam(ParamValidatorDoubleSignSlashPercentage)
	}
	if _, err := crypto.NewAddressFromString(x.ValidatorDoubleSignSlashPercentage.Owner); err != nil {
		return ErrInvalidOwner(ParamValidatorDoubleSignSlashPercentage)
	}
	if _, err := lib.StringToBigInt(x.ValidatorDoubleSignReporterReward.Value); err != nil {
		return ErrInvalidParam(ParamValidatorDoubleSignReporterReward)
	}
	if _, err := crypto.NewAddressFromString(x.ValidatorDoubleSignReporterReward.Owner); err != nil {
		return ErrInvalidOwner(ParamValidatorDoubleSignReporterReward)
	}
	if x.ValidatorProposerPercentageOfFees.Value > 100 {
		return ErrInvalidParam(ParamValidatorProposerPercentageOfFees)
	}
	if _, err := crypto.NewAddressFromString(x.ValidatorProposerPercentageOfFees.Owner); err != nil {
		return ErrInvalidOwner(ParamValidatorProposerPercentageOfFees)
	}
	if _, err := lib.StringToBigInt(x.ValidatorBlockReward.Value); err != nil {
		return ErrInvalidParam(ParamValidatorBlockReward)
	}
	if _, err := crypto.NewAddressFromString(x.ValidatorBlockReward.Owner); err != nil {
		return ErrInvalidOwner(ParamValidatorBlockReward)
	}
	return nil
}

func (x *ValidatorParams) SetUint64(address string, paramName string, value uint64) lib.ErrorI {
	switch paramName {
	case ParamValidatorUnstakingBlocks:
		if address != x.ValidatorUnstakingBlocks.Owner {
			return ErrUnauthorizedParamChange()
		}
		x.ValidatorUnstakingBlocks.Value = value
	case ParamValidatorMaxCount:
		if address != x.ValidatorMaxCount.Owner {
			return ErrUnauthorizedParamChange()
		}
		x.ValidatorMaxCount.Value = value
	case ParamValidatorMaxPauseBlocks:
		if address != x.ValidatorMaxPauseBlocks.Owner {
			return ErrUnauthorizedParamChange()
		}
		x.ValidatorMaxPauseBlocks.Value = value
	case ParamValidatorMaxEvidenceAgeInBlocks:
		if address != x.ValidatorMaxEvidenceAgeInBlocks.Owner {
			return ErrUnauthorizedParamChange()
		}
		x.ValidatorMaxEvidenceAgeInBlocks.Value = value
	case ParamValidatorBadProposeSlashPercentage:
		if address != x.ValidatorBadProposalSlashPercentage.Owner {
			return ErrUnauthorizedParamChange()
		}
		x.ValidatorBadProposalSlashPercentage.Value = value
	case ParamValidatorFaultySignSlashPercentage:
		if address != x.ValidatorFaultySignSlashPercentage.Owner {
			return ErrUnauthorizedParamChange()
		}
		x.ValidatorFaultySignSlashPercentage.Value = value
	case ParamValidatorNonSignWindow:
		if address != x.ValidatorNonSignWindow.Owner {
			return ErrUnauthorizedParamChange()
		}
		x.ValidatorNonSignWindow.Value = value
	case ParamValidatorMaxNonSign:
		if address != x.ValidatorMaxNonSign.Owner {
			return ErrUnauthorizedParamChange()
		}
		x.ValidatorMaxNonSign.Value = value
	case ParamValidatorNonSignSlashPercentage:
		if address != x.ValidatorNonSignSlashPercentage.Owner {
			return ErrUnauthorizedParamChange()
		}
		x.ValidatorNonSignSlashPercentage.Value = value
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
	return x.Validate()
}

func (x *ValidatorParams) SetString(address string, paramName string, value string) lib.ErrorI {
	switch paramName {
	case ParamValidatorMinStake:
		if address != x.ValidatorMinStake.Owner {
			return ErrUnauthorizedParamChange()
		}
		x.ValidatorMinStake.Value = value
	case ParamValidatorDoubleSignReporterReward:
		if address != x.ValidatorDoubleSignReporterReward.Owner {
			return ErrUnauthorizedParamChange()
		}
		x.ValidatorDoubleSignReporterReward.Value = value
	case ParamValidatorBlockReward:
		if address != x.ValidatorBlockReward.Owner {
			return ErrUnauthorizedParamChange()
		}
		x.ValidatorBlockReward.Value = value
	default:
		return ErrUnknownParam()
	}
	return x.Validate()
}

func (x *ValidatorParams) SetOwner(paramName string, owner string) lib.ErrorI {
	name := stripKeywordOwnerFromParamName(paramName)
	switch name {
	case ParamValidatorMinStake:
		x.ValidatorMinStake.Owner = owner
	case ParamValidatorMaxCount:
		x.ValidatorMaxCount.Owner = owner
	case ParamValidatorUnstakingBlocks:
		x.ValidatorUnstakingBlocks.Owner = owner
	case ParamValidatorMaxPauseBlocks:
		x.ValidatorMaxPauseBlocks.Owner = owner
	case ParamValidatorMaxEvidenceAgeInBlocks:
		x.ValidatorMaxEvidenceAgeInBlocks.Owner = owner
	case ParamValidatorNonSignWindow:
		x.ValidatorNonSignWindow.Owner = owner
	case ParamValidatorMaxNonSign:
		x.ValidatorMaxNonSign.Owner = owner
	case ParamValidatorBadProposeSlashPercentage:
		x.ValidatorBadProposalSlashPercentage.Owner = owner
	case ParamValidatorFaultySignSlashPercentage:
		x.ValidatorFaultySignSlashPercentage.Owner = owner
	case ParamValidatorNonSignSlashPercentage:
		x.ValidatorNonSignSlashPercentage.Owner = owner
	case ParamValidatorDoubleSignSlashPercentage:
		x.ValidatorDoubleSignSlashPercentage.Owner = owner
	case ParamValidatorDoubleSignReporterReward:
		x.ValidatorDoubleSignReporterReward.Owner = owner
	case ParamValidatorProposerPercentageOfFees:
		x.ValidatorProposerPercentageOfFees.Owner = owner
	case ParamValidatorBlockReward:
		x.ValidatorBlockReward.Owner = owner
	default:
		return ErrUnknownParam()
	}
	return x.Validate()
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
	if i, err := lib.StringToBigInt(x.MessageSendFee.Value); err != nil || lib.BigIsZero(i) {
		return ErrInvalidParam(ParamMessageSendFee)
	}
	if _, err := crypto.NewAddressFromString(x.MessageSendFee.Owner); err != nil {
		return ErrInvalidOwner(ParamMessageSendFee)
	}
	if i, err := lib.StringToBigInt(x.MessageStakeFee.Value); err != nil || lib.BigIsZero(i) {
		return ErrInvalidParam(ParamMessageStakeFee)
	}
	if _, err := crypto.NewAddressFromString(x.MessageStakeFee.Owner); err != nil {
		return ErrInvalidOwner(ParamMessageStakeFee)
	}
	if i, err := lib.StringToBigInt(x.MessageEditStakeFee.Value); err != nil || lib.BigIsZero(i) {
		return ErrInvalidParam(ParamMessageEditStakeFee)
	}
	if _, err := crypto.NewAddressFromString(x.MessageEditStakeFee.Owner); err != nil {
		return ErrInvalidOwner(ParamMessageEditStakeFee)
	}
	if i, err := lib.StringToBigInt(x.MessageUnstakeFee.Value); err != nil || lib.BigIsZero(i) {
		return ErrInvalidParam(ParamMessageUnstakeFee)
	}
	if _, err := crypto.NewAddressFromString(x.MessageUnstakeFee.Owner); err != nil {
		return ErrInvalidOwner(ParamMessageUnstakeFee)
	}
	if i, err := lib.StringToBigInt(x.MessagePauseFee.Value); err != nil || lib.BigIsZero(i) {
		return ErrInvalidParam(ParamMessagePauseFee)
	}
	if _, err := crypto.NewAddressFromString(x.MessagePauseFee.Owner); err != nil {
		return ErrInvalidOwner(ParamMessagePauseFee)
	}
	if i, err := lib.StringToBigInt(x.MessageUnpauseFee.Value); err != nil || lib.BigIsZero(i) {
		return ErrInvalidParam(ParamMessageUnpauseFee)
	}
	if _, err := crypto.NewAddressFromString(x.MessageUnpauseFee.Owner); err != nil {
		return ErrInvalidOwner(ParamMessageUnpauseFee)
	}
	if i, err := lib.StringToBigInt(x.MessageChangeParameterFee.Value); err != nil || lib.BigIsZero(i) {
		return ErrInvalidParam(ParamMessageChangeParameterFee)
	}
	if _, err := crypto.NewAddressFromString(x.MessageChangeParameterFee.Owner); err != nil {
		return ErrInvalidOwner(ParamMessageChangeParameterFee)
	}
	if i, err := lib.StringToBigInt(x.MessageDoubleSignFee.Value); err != nil || lib.BigIsZero(i) {
		return ErrInvalidParam(ParamMessageDoubleSignFee)
	}
	if _, err := crypto.NewAddressFromString(x.MessageDoubleSignFee.Owner); err != nil {
		return ErrInvalidOwner(ParamMessageDoubleSignFee)
	}
	return nil
}

func (x *FeeParams) SetString(address string, paramName string, value string) lib.ErrorI {
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
	return x.Validate()
}

func (x *FeeParams) SetOwner(paramName string, owner string) lib.ErrorI {
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
	return x.Validate()
}

func (x *FeeParams) SetUint64(address string, paramName string, value uint64) lib.ErrorI {
	return ErrUnknownParam()
}

// governance param space

const (
	ParamACLOwner = "acl"
)

var _ ParamSpace = &GovernanceParams{}

func (x *GovernanceParams) Validate() lib.ErrorI {
	if _, err := crypto.NewAddressFromString(x.AclOwner); err != nil {
		return ErrInvalidOwner(ParamACLOwner)
	}
	return nil
}

func (x *GovernanceParams) SetOwner(paramName string, owner string) lib.ErrorI {
	switch paramName {
	case ParamACLOwner:
		x.AclOwner = owner
	default:
		return ErrUnknownParam()
	}
	return x.Validate()
}

func (x *GovernanceParams) SetUint64(_ string, _ string, _ uint64) lib.ErrorI {
	return ErrUnknownParam()
}

func (x *GovernanceParams) SetString(_ string, _ string, _ string) lib.ErrorI {
	return ErrUnknownParam()
}

func stripKeywordOwnerFromParamName(pn string) string {
	return strings.Replace(pn, ParamKeywordOwner, "", -1)
}
