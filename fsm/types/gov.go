package types

import (
	"encoding/json"
	"fmt"
	"github.com/alecthomas/units"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"google.golang.org/protobuf/proto"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	ParamPrefixCons = "/c/"
	ParamPrefixVal  = "/v/"
	ParamPrefixFee  = "/f/"
	ParamPrefixGov  = "/g/"

	ParamSpaceCons = "cons"
	ParamSpaceVal  = "val"
	ParamSpaceFee  = "fee"
	ParamSpaceGov  = "gov"

	Delimiter = "/"

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

type ProposalWithVote struct {
	Proposal Proposal `json:"proposal"`
	Approve  bool     `json:"approve"`
}

type Poll map[string]PollResult

type PollResult struct {
	ProposalJSON      json.RawMessage          `json:"proposalJSON"`
	ApprovedPower     uint64                   `json:"approvedPower"`
	ApprovedPercent   uint64                   `json:"approvedPercent"`
	RejectedPercent   uint64                   `json:"rejectedPercent"`
	TotalVotedPercent uint64                   `json:"totalVotedPercent"`
	RejectedPower     uint64                   `json:"rejectedPower"`
	TotalVotedPower   uint64                   `json:"totalVotedPower"`
	TotalPower        uint64                   `json:"totalPower"`
	ApproveVotes      []lib.ConsensusValidator `json:"approveVotes"`
	RejectVotes       []lib.ConsensusValidator `json:"rejectVotes"`
}

func PollValidators(vals *lib.ConsensusValidators, path string, logger lib.LoggerI) (poll Poll) {
	poll = make(Poll)
	validatorSet, e := lib.NewValidatorSet(vals)
	if e != nil {
		logger.Error(ErrPollValidator(e).Error())
		return
	}
	for _, v := range vals.ValidatorSet {
		p := make(Proposals)
		u, err := lib.ReplaceURLPort(v.NetAddress, lib.DefaultRPCConfig().RPCPort)
		if err != nil {
			logger.Error(ErrPollValidator(err).Error())
			continue
		}
		u, err = url.JoinPath(u, path)
		if err != nil {
			logger.Error(ErrPollValidator(err).Error())
			continue
		}
		resp, err := http.Get(u)
		if err != nil {
			logger.Error(ErrPollValidator(err).Error())
			continue
		}
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			logger.Error(ErrPollValidator(err).Error())
			continue
		}
		if err = lib.UnmarshalJSON(data, &p); err != nil {
			logger.Error(ErrPollValidator(err).Error())
			continue
		}
		for hash, propWithVote := range p {
			state := poll[hash]
			if len(state.ProposalJSON) == 0 {
				data, err = lib.MarshalJSON(propWithVote.Proposal)
				if err != nil {
					logger.Error(ErrPollValidator(err).Error())
					continue
				}
				state.ProposalJSON = data
				state.TotalPower = validatorSet.TotalPower
			}
			if propWithVote.Approve {
				state.ApproveVotes = append(state.ApproveVotes, *v)
				state.ApprovedPower += v.VotingPower
				state.TotalVotedPower += v.VotingPower
				state.ApprovedPercent = lib.Uint64PercentageDiv(state.ApprovedPower, state.TotalVotedPower)
			} else {
				state.RejectVotes = append(state.ApproveVotes, *v)
				state.RejectedPower += v.VotingPower
				state.TotalVotedPower += v.VotingPower
				state.RejectedPercent = lib.Uint64PercentageDiv(state.RejectedPower, state.TotalVotedPower)
			}
			state.TotalVotedPercent = lib.Uint64PercentageDiv(state.TotalVotedPower, state.TotalPower)
			poll[hash] = state
		}
	}
	return
}

type Proposals map[string]ProposalWithVote

func (p Proposals) NewFromFile(dataDirPath string) lib.ErrorI {
	bz, err := os.ReadFile(filepath.Join(dataDirPath, lib.ProposalsFilePath))
	if err != nil {
		return lib.ErrReadFile(err)
	}
	return lib.UnmarshalJSON(bz, &p)
}

func (p Proposals) Add(proposal Proposal, approve bool) lib.ErrorI {
	bz, err := lib.Marshal(proposal)
	if err != nil {
		return err
	}
	p[crypto.HashString(bz)] = ProposalWithVote{proposal, approve}
	return nil
}

func (p Proposals) Del(proposal Proposal) {
	bz, _ := lib.Marshal(proposal)
	delete(p, crypto.HashString(bz))
}

func (p Proposals) SaveToFile(dataDirPath string) lib.ErrorI {
	bz, err := lib.MarshalJSONIndent(p)
	if err != nil {
		return err
	}
	if e := os.WriteFile(filepath.Join(dataDirPath, lib.ProposalsFilePath), bz, os.ModePerm); e != nil {
		return lib.ErrWriteFile(e)
	}
	return nil
}

func NewProposalFromBytes(b []byte) (Proposal, lib.ErrorI) {
	cp, dt := new(MessageChangeParameter), new(MessageDAOTransfer)
	if err := lib.UnmarshalJSON(b, cp); err != nil {
		if err = lib.UnmarshalJSON(b, dt); err != nil {
			return nil, err
		}
		return dt, nil
	}
	return cp, nil
}

func (p *ProposalWithVote) UnmarshalJSON(b []byte) (err error) {
	j := new(struct {
		Proposal json.RawMessage `json:"proposal"`
		Approve  bool            `json:"approve"`
	})
	if err = json.Unmarshal(b, &j); err != nil {
		return
	}
	p.Approve = j.Approve
	p.Proposal, err = NewProposalFromBytes(j.Proposal)
	return
}

func DefaultParams() *Params {
	return &Params{
		Consensus: &ConsensusParams{
			BlockSize:       uint64(units.MB),
			ProtocolVersion: NewProtocolVersion(0, 0),
		},
		Validator: &ValidatorParams{
			ValidatorMinStake:                   1000000,
			ValidatorMaxCount:                   1000,
			ValidatorUnstakingBlocks:            2,
			ValidatorMaxPauseBlocks:             4380,
			ValidatorDoubleSignSlashPercentage:  25,
			ValidatorBadProposalSlashPercentage: 1,
			ValidatorNonSignSlashPercentage:     1,
			ValidatorMaxNonSign:                 4,
			ValidatorNonSignWindow:              10,
			ValidatorBlockReward:                1000000,
		},
		Fee: &FeeParams{
			MessageSendFee:            10000,
			MessageStakeFee:           10000,
			MessageEditStakeFee:       10000,
			MessageUnstakeFee:         10000,
			MessagePauseFee:           10000,
			MessageUnpauseFee:         10000,
			MessageChangeParameterFee: 10000,
			MessageDaoTransferFee:     10000,
		},
		Governance: &GovernanceParams{DaoRewardPercentage: 10},
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
		if _, err := CheckProtocolVersion(value); err != nil {
			return err
		}
		x.ProtocolVersion = value
	default:
		return ErrUnknownParam()
	}
	return x.Validate()
}

func (x *ConsensusParams) ParseProtocolVersion() (*ProtocolVersion, lib.ErrorI) {
	return CheckProtocolVersion(x.ProtocolVersion)
}

func CheckProtocolVersion(v string) (*ProtocolVersion, lib.ErrorI) {
	ptr := new(ProtocolVersion)
	arr := strings.Split(v, Delimiter)
	if len(arr) != 2 {
		return nil, ErrInvalidProtocolVersion()
	}
	version, err := strconv.Atoi(arr[0])
	if err != nil {
		return nil, ErrInvalidProtocolVersion()
	}
	height, err := strconv.Atoi(arr[1])
	if err != nil {
		return nil, ErrInvalidProtocolVersion()
	}
	ptr.Height, ptr.Version = uint64(height), uint64(version)
	return ptr, nil
}

func NewProtocolVersion(height uint64, version uint64) string {
	return fmt.Sprintf("%d%s%d", version, Delimiter, height)
}

func FormatParamSpace(paramSpace string) string {
	paramSpace = strings.ToLower(paramSpace)
	switch {
	case strings.Contains(paramSpace, "con"):
		return ParamSpaceCons
	case strings.Contains(paramSpace, "gov"):
		return ParamSpaceGov
	case strings.Contains(paramSpace, "fee"):
		return ParamSpaceFee
	case strings.Contains(paramSpace, "val"):
		return ParamSpaceVal
	}
	return paramSpace
}

func IsStringParam(paramSpace, paramKey string) (bool, lib.ErrorI) {
	testValueStr, testValue := NewProtocolVersion(1, 1), uint64(2)
	params := DefaultParams()
	switch paramSpace {
	case ParamSpaceVal:
		if err := params.Validator.SetString(paramKey, testValueStr); err != nil {
			if err = params.Validator.SetUint64(paramKey, testValue); err != nil {
				return false, err
			}
			return false, nil
		}
		return true, nil
	case ParamSpaceCons:
		if err := params.Consensus.SetString(paramKey, testValueStr); err != nil {
			if err = params.Consensus.SetUint64(paramKey, testValue); err != nil {
				return false, err
			}
			return false, nil
		}
		return true, nil
	case ParamSpaceGov:
		if err := params.Governance.SetString(paramKey, testValueStr); err != nil {
			if err = params.Governance.SetUint64(paramKey, testValue); err != nil {
				return false, err
			}
			return false, nil
		}
		return true, nil
	case ParamSpaceFee:
		if err := params.Fee.SetString(paramKey, testValueStr); err != nil {
			if err = params.Fee.SetUint64(paramKey, testValue); err != nil {
				return false, err
			}
			return false, nil
		}
		return true, nil
	default:
		return false, ErrUnknownParamSpace()
	}
}

// validator param space

var _ ParamSpace = &ValidatorParams{}

const (
	ParamValidatorMinStake                  = "validator_min_stake"
	ParamValidatorMaxCount                  = "validator_max_count"
	ParamValidatorUnstakingBlocks           = "validator_unstaking_blocks"
	ParamValidatorMaxPauseBlocks            = "validator_max_pause_blocks"
	ParamValidatorBadProposeSlashPercentage = "validator_bad_propose_slash_percentage"
	ParamValidatorNonSignSlashPercentage    = "validator_non_sign_slash_percentage"
	ParamValidatorMaxNonSign                = "validator_max_non_sign"
	ParamValidatorNonSignWindow             = "validator_non_sign_window"
	ParamValidatorDoubleSignSlashPercentage = "validator_double_sign_slash_percentage"
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
	if x.ValidatorBadProposalSlashPercentage > 100 {
		return ErrInvalidParam(ParamValidatorBadProposeSlashPercentage)
	}
	if x.ValidatorNonSignSlashPercentage > 100 {
		return ErrInvalidParam(ParamValidatorNonSignSlashPercentage)
	}
	if x.ValidatorNonSignWindow == 0 {
		return ErrInvalidParam(ParamValidatorNonSignWindow)
	}
	if x.ValidatorMaxNonSign > x.ValidatorNonSignWindow {
		return ErrInvalidParam(ParamValidatorMaxNonSign)
	}
	if x.ValidatorDoubleSignSlashPercentage > 100 {
		return ErrInvalidParam(ParamValidatorDoubleSignSlashPercentage)
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
	ParamMessageDAOTransferFee     = "message_dao_transfer_fee"
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
	if x.MessageDaoTransferFee == 0 {
		return ErrInvalidParam(ParamMessageDAOTransferFee)
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
	case ParamMessageDAOTransferFee:
		x.MessageDaoTransferFee = value
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
