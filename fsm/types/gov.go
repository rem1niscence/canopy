package types

import (
	"encoding/json"
	"fmt"
	"github.com/alecthomas/units"
	"github.com/ginchuco/canopy/lib"
	"github.com/ginchuco/canopy/lib/crypto"
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
	ParamPrefixCons = "/c/" // store key prefix for Consensus param space
	ParamPrefixVal  = "/v/" // store key prefix for Validators param space
	ParamPrefixFee  = "/f/" // store key prefix for Fee param space
	ParamPrefixGov  = "/g/" // store key prefix for Gov param space

	ParamSpaceCons = "cons" // name of Consensus param space
	ParamSpaceVal  = "val"  // name of Validator param space
	ParamSpaceFee  = "fee"  // name of Fee param space
	ParamSpaceGov  = "gov"  // name of Governance param space

	Delimiter = "/" // standard delimiter for protocol version

	AcceptAllProposals  = GovProposalVoteConfig_ACCEPT_ALL
	ProposalApproveList = GovProposalVoteConfig_APPROVE_LIST
	RejectAllProposals  = GovProposalVoteConfig_REJECT_ALL
)

// ParamSpace is a distinct, isolated category within the overarching Params structure
type ParamSpace interface {
	Check() lib.ErrorI
	SetString(paramName string, value string) lib.ErrorI // SetString() update a string parameter in the structure
	SetUint64(paramName string, value uint64) lib.ErrorI // SetUint64() update a uint64 parameter in the structure
}

// DefaultParams() returns the developer set params
func DefaultParams() *Params {
	return &Params{
		Consensus: &ConsensusParams{
			BlockSize:       uint64(units.MB),
			ProtocolVersion: NewProtocolVersion(0, 1),
		},
		Validator: &ValidatorParams{
			ValidatorUnstakingBlocks:                2,
			ValidatorMaxPauseBlocks:                 4380,
			ValidatorDoubleSignSlashPercentage:      10,
			ValidatorBadProposalSlashPercentage:     1,
			ValidatorNonSignSlashPercentage:         1,
			ValidatorMaxNonSign:                     4,
			ValidatorNonSignWindow:                  10,
			ValidatorMaxCommittees:                  15,
			ValidatorMaxCommitteeSize:               100,
			ValidatorBlockReward:                    1000000,
			ValidatorEarlyWithdrawalPenalty:         20,
			ValidatorDelegateUnstakingBlocks:        2,
			ValidatorMinimumOrderSize:               100000000000,
			ValidatorMinimumPercentForPaidCommittee: 33,
			ValidatorMaxSlashPerCommittee:           15,
		},
		Fee: &FeeParams{
			MessageSendFee:               10000,
			MessageStakeFee:              10000,
			MessageEditStakeFee:          10000,
			MessageUnstakeFee:            10000,
			MessagePauseFee:              10000,
			MessageUnpauseFee:            10000,
			MessageChangeParameterFee:    10000,
			MessageDaoTransferFee:        10000,
			MessageCertificateResultsFee: 0,
			MessageSubsidyFee:            10000,
			MessageCreateOrderFee:        10000,
			MessageEditOrderFee:          10000,
			MessageDeleteOrderFee:        10000,
		},
		Governance: &GovernanceParams{DaoRewardPercentage: 10},
	}
}

// Check() validates the Params object
func (x *Params) Check() lib.ErrorI {
	if err := x.Consensus.Check(); err != nil {
		return err
	}
	if err := x.Fee.Check(); err != nil {
		return err
	}
	if err := x.Validator.Check(); err != nil {
		return err
	}
	return x.Governance.Check()
}

// consensus param space

const (
	ParamBlockSize       = "block_size"       // size of the block - header
	ParamProtocolVersion = "protocol_version" // current protocol version (upgrade enforcement)
)

var _ ParamSpace = &ConsensusParams{}

// Check() validates the consensus params
func (x *ConsensusParams) Check() lib.ErrorI {
	if x.BlockSize == 0 {
		return ErrInvalidParam(ParamBlockSize)
	}
	if _, err := x.ParseProtocolVersion(); err != nil {
		return err
	}
	return nil
}

// SetUint64() update a uint64 parameter in the structure
func (x *ConsensusParams) SetUint64(paramName string, value uint64) lib.ErrorI {
	switch paramName {
	case ParamBlockSize:
		x.BlockSize = value
	default:
		return ErrUnknownParam()
	}
	return x.Check()
}

// SetString() update a string parameter in the structure
func (x *ConsensusParams) SetString(paramName string, value string) lib.ErrorI {
	switch paramName {
	case ParamProtocolVersion:
		if _, err := checkProtocolVersion(value); err != nil {
			return err
		}
		x.ProtocolVersion = value
	default:
		return ErrUnknownParam()
	}
	return x.Check()
}

// ParseProtocolVersion() validates the format of the Protocol version string and returns the ProtocolVersion object
func (x *ConsensusParams) ParseProtocolVersion() (*ProtocolVersion, lib.ErrorI) {
	return checkProtocolVersion(x.ProtocolVersion)
}

// checkProtocolVersion (helper) validates the format of the Protocol version string and returns the ProtocolVersion object
func checkProtocolVersion(v string) (*ProtocolVersion, lib.ErrorI) {
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

// NewProtocolVersion() creates a properly formatted protocol version string
func NewProtocolVersion(height uint64, version uint64) string {
	return fmt.Sprintf("%d%s%d", version, Delimiter, height)
}

// FormatParamSpace() converts a user inputted ParamSpace string into the proper ParamSpace name
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

// IsStringParam() validates if the param is a string by space and key
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
	ParamValidatorUnstakingBlocks                = "validator_unstaking_blocks"                   // number of blocks a committee member must be 'unstaking' for
	ParamValidatorMaxPauseBlocks                 = "validator_max_pause_blocks"                   // maximum blocks a validator may be paused for before force-unstaking
	ParamValidatorBadProposeSlashPercentage      = "validator_bad_propose_slash_percentage"       // how much a bad proposer is slashed (% of stake)
	ParamValidatorNonSignSlashPercentage         = "validator_non_sign_slash_percentage"          // how much a non-signer is slashed if exceeds threshold in window (% of stake)
	ParamValidatorMaxNonSign                     = "validator_max_non_sign"                       // how much a committee member can not sign before being slashed
	ParamValidatorNonSignWindow                  = "validator_non_sign_window"                    // how frequently the non-sign-count is reset
	ParamValidatorDoubleSignSlashPercentage      = "validator_double_sign_slash_percentage"       // how much a double signer is slashed (% of stake)
	ParamValidatorMaxCommittees                  = "validator_max_committees"                     // maximum number of committees a single validator may participate in
	ParamValidatorMaxCommitteeSize               = "validator_max_committee_size"                 // maximum number of members a committee may have
	ParamValidatorBlockReward                    = "validator_block_reward"                       // the amount of tokens minted each block
	ParamValidatorEarlyWithdrawalPenalty         = "validator_early_withdrawal_penalty"           // reduction percentage of non-compounded rewards
	ParamValidatorDelegateUnstakingBlocks        = "validator_delegate_unstaking_blocks"          // number of blocks a delegator must be 'unstaking' for
	ParamValidatorMinimumOrderSize               = "validator_minimum_order_size"                 // minimum sell tokens in a sell order
	ParamValidatorMinimumPercentForPaidCommittee = "validator_minimum_percent_for_paid_committee" // the minimum percentage of total stake needed to be a 'paid committee'
	ParamValidatorMaxSlashPerCommittee           = "validator_max_slash_per_committee"            // the maximum validator slash per committee per block
)

// Check() validates the Validator params
func (x *ValidatorParams) Check() lib.ErrorI {
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
	if x.ValidatorMaxCommittees > 100 {
		return ErrInvalidParam(ParamValidatorMaxCommittees)
	}
	if x.ValidatorMaxCommitteeSize == 0 {
		return ErrInvalidParam(ParamValidatorMaxCommitteeSize)
	}
	if x.ValidatorDelegateUnstakingBlocks < 2 {
		return ErrInvalidParam(ParamValidatorDelegateUnstakingBlocks)
	}
	if x.ValidatorEarlyWithdrawalPenalty > 100 {
		return ErrInvalidParam(ParamValidatorEarlyWithdrawalPenalty)
	}
	if x.ValidatorMinimumPercentForPaidCommittee == 0 || x.ValidatorMinimumPercentForPaidCommittee > 100 {
		return ErrInvalidParam(ParamValidatorMinimumPercentForPaidCommittee)
	}
	if x.ValidatorMaxSlashPerCommittee == 0 || x.ValidatorMaxSlashPerCommittee > 100 {
		return ErrInvalidParam(ParamValidatorMaxSlashPerCommittee)
	}
	return nil
}

// SetUint64() update a uint64 parameter in the structure
func (x *ValidatorParams) SetUint64(paramName string, value uint64) lib.ErrorI {
	switch paramName {
	case ParamValidatorUnstakingBlocks:
		x.ValidatorUnstakingBlocks = value
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
	case ParamValidatorMaxCommittees:
		x.ValidatorMaxCommittees = value
	case ParamValidatorMaxCommitteeSize:
		x.ValidatorMaxCommitteeSize = value
	case ParamValidatorBlockReward:
		x.ValidatorBlockReward = value
	case ParamValidatorEarlyWithdrawalPenalty:
		x.ValidatorEarlyWithdrawalPenalty = value
	case ParamValidatorDelegateUnstakingBlocks:
		x.ValidatorDelegateUnstakingBlocks = value
	case ParamValidatorMinimumOrderSize:
		x.ValidatorMinimumOrderSize = value
	case ParamValidatorMinimumPercentForPaidCommittee:
		x.ValidatorMinimumPercentForPaidCommittee = value
	case ParamValidatorMaxSlashPerCommittee:
		x.ValidatorMaxSlashPerCommittee = value
	default:
		return ErrUnknownParam()
	}
	return x.Check()
}

// SetString() update a string parameter in the structure
func (x *ValidatorParams) SetString(_ string, _ string) lib.ErrorI {
	return ErrUnknownParam()
}

// fee param space

var _ ParamSpace = &FeeParams{}

const (
	ParamMessageSendFee               = "message_send_fee"                // transaction fee for MessageSend
	ParamMessageStakeFee              = "message_stake_fee"               // transaction fee for MessageStake
	ParamMessageEditStakeFee          = "message_edit_stake_fee"          // transaction fee for MessageEditStake
	ParamMessageUnstakeFee            = "message_unstake_fee"             // transaction fee for MessageUnstake
	ParamMessagePauseFee              = "message_pause_fee"               // transaction fee for MessagePause
	ParamMessageUnpauseFee            = "message_unpause_fee"             // transaction fee for MessageUnpause
	ParamMessageChangeParameterFee    = "message_change_parameter_fee"    // transaction fee for MessageChangeParameter
	ParamMessageDAOTransferFee        = "message_dao_transfer_fee"        // transaction fee for MessageDAOTransfer
	ParamMessageCertificateResultsFee = "message_certificate_results_fee" // transaction fee for MessageCertificateResults
	ParamMessageSubsidyFee            = "message_subsidy_fee"             // transaction fee for MessageSubsidy
	ParamMessageCreateOrder           = "message_create_order"            // transaction fee for MessageCreateOrder
	ParamMessageEditOrder             = "message_edit_order"              // transaction fee for MessageEditOrder
	ParamMessageDeleteOrder           = "message_delete_order"            // transaction fee for MessageDeleteOrder
)

// Check() validates the Fee params
func (x *FeeParams) Check() lib.ErrorI {
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
	if x.MessageSubsidyFee == 0 {
		return ErrInvalidParam(ParamMessageSubsidyFee)
	}
	if x.MessageCreateOrderFee == 0 {
		return ErrInvalidParam(ParamMessageCreateOrder)
	}
	if x.MessageEditOrderFee == 0 {
		return ErrInvalidParam(ParamMessageEditOrder)
	}
	if x.MessageDeleteOrderFee == 0 {
		return ErrInvalidParam(ParamMessageDeleteOrder)
	}
	return nil
}

// SetString() update a string parameter in the structure
func (x *FeeParams) SetString(_ string, _ string) lib.ErrorI {
	return ErrUnknownParam()
}

// SetUint64() update a uint64 parameter in the structure
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
	case ParamMessageCertificateResultsFee:
		x.MessageCertificateResultsFee = value
	case ParamMessageSubsidyFee:
		x.MessageSubsidyFee = value
	default:
		return ErrUnknownParam()
	}
	return x.Check()
}

// governance param space

const (
	ParamDAORewardPercentage = "dao_reward_percentage" // percent of rewards the DAO fund receives
)

var _ ParamSpace = &GovernanceParams{}

// Check() validates the Governance params
func (x *GovernanceParams) Check() lib.ErrorI {
	if x.DaoRewardPercentage > 100 {
		return ErrInvalidParam(ParamDAORewardPercentage)
	}
	return nil
}

// SetUint64() update a uint64 parameter in the structure
func (x *GovernanceParams) SetUint64(paramName string, value uint64) lib.ErrorI {
	switch paramName {
	case ParamDAORewardPercentage:
		x.DaoRewardPercentage = value
	default:
		return ErrUnknownParam()
	}
	return x.Check()
}

// SetString() update a string parameter in the structure
func (x *GovernanceParams) SetString(_ string, _ string) lib.ErrorI {
	return ErrUnknownParam()
}

// prefixForParamSpace() converts the ParamSpace name into the ParamSpace prefix
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

// POLLING CODE BELOW

// Poll is a list of PollResults keyed by the hash of the ProposalJSON
type Poll map[string]PollResult

// PollResult is a structure that represents the current state of a 'Poll' for a 'Proposal'
type PollResult struct {
	ProposalJSON      json.RawMessage           `json:"proposalJSON"`      // the json of the proposal
	ApprovedPower     uint64                    `json:"approvedPower"`     // power (tokens) that voted 'yay'
	ApprovedPercent   uint64                    `json:"approvedPercent"`   // percent representation of power (tokens) that voted 'yay' out of total power
	RejectedPercent   uint64                    `json:"rejectedPercent"`   // percent representation of power (tokens) that voted 'nay' out of total power
	TotalVotedPercent uint64                    `json:"totalVotedPercent"` // percent representation of power (tokens) voted out of total power
	RejectedPower     uint64                    `json:"rejectedPower"`     // power (tokens)  that voted 'nay'
	TotalVotedPower   uint64                    `json:"totalVotedPower"`   // total power (tokens) that already voted
	TotalPower        uint64                    `json:"totalPower"`        // total power (tokens)  that could possibly vote
	ApproveVotes      []*lib.ConsensusValidator `json:"approveVotes"`      // voters who voted 'yay'
	RejectVotes       []*lib.ConsensusValidator `json:"rejectVotes"`       // voters who voted 'nay'
}

// PollValidators() iterates through all validators in a committee and checks how they voted
func PollValidators(vals *lib.ConsensusValidators, path string, logger lib.LoggerI) (poll Poll) {
	poll = make(Poll)
	validatorSet, e := lib.NewValidatorSet(vals)
	if e != nil {
		logger.Error(ErrPollValidator(e).Error())
		return
	}
	for _, v := range vals.ValidatorSet {
		p, u := make(GovProposals), ""
		u, e = lib.RemoveIPV4Port(v.NetAddress)
		if e != nil {
			logger.Error(ErrPollValidator(e).Error())
			continue
		}
		u, err := url.JoinPath(u, path)
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
				state.ApproveVotes = append(state.ApproveVotes, v)
				state.ApprovedPower += v.VotingPower
				state.TotalVotedPower += v.VotingPower
				state.ApprovedPercent = lib.Uint64PercentageDiv(state.ApprovedPower, state.TotalVotedPower)
			} else {
				state.RejectVotes = append(state.ApproveVotes, v)
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

// PROPOSAL CODE BELOW

// GovProposal is an interface that all proposals that may be polled for and voted on must conform to
type GovProposal interface {
	proto.Message
	GetStartHeight() uint64
	GetEndHeight() uint64
}

// GovProposalWithVote is a wrapper over a GovProposal but contains an approval / disapproval boolean
type GovProposalWithVote struct {
	Proposal GovProposal `json:"proposal"`
	Approve  bool        `json:"approve"`
}

// GovProposals is a list of GovProposalsWithVote keyed by hash of the underlying ProposalJSON
type GovProposals map[string]GovProposalWithVote

// NewProposalFromBytes() creates a GovProposal object from json bytes
func NewProposalFromBytes(b []byte) (GovProposal, lib.ErrorI) {
	cp, dt := new(MessageChangeParameter), new(MessageDAOTransfer)
	if err := lib.UnmarshalJSON(b, cp); err != nil {
		if err = lib.UnmarshalJSON(b, dt); err != nil {
			return nil, err
		}
		return dt, nil
	}
	return cp, nil
}

// NewFromFile() creates a new GovProposals object from a file
func (p GovProposals) NewFromFile(dataDirPath string) lib.ErrorI {
	bz, err := os.ReadFile(filepath.Join(dataDirPath, lib.ProposalsFilePath))
	if err != nil {
		return lib.ErrReadFile(err)
	}
	return lib.UnmarshalJSON(bz, &p)
}

// Add() adds a GovProposalWithVote to the list
func (p GovProposals) Add(proposal GovProposal, approve bool) lib.ErrorI {
	bz, err := lib.Marshal(proposal)
	if err != nil {
		return err
	}
	p[crypto.HashString(bz)] = GovProposalWithVote{proposal, approve}
	return nil
}

// Del() removes a GovProposalWithVote from the list
func (p GovProposals) Del(proposal GovProposal) {
	bz, _ := lib.Marshal(proposal)
	delete(p, crypto.HashString(bz))
}

// SaveToFile() persists the GovProposals object to a json file
func (p GovProposals) SaveToFile(dataDirPath string) lib.ErrorI {
	bz, err := lib.MarshalJSONIndent(p)
	if err != nil {
		return err
	}
	if e := os.WriteFile(filepath.Join(dataDirPath, lib.ProposalsFilePath), bz, os.ModePerm); e != nil {
		return lib.ErrWriteFile(e)
	}
	return nil
}

// UnmarshalJSON() implements the json.Unmarshaler interface for GovProposalWithVote
func (p *GovProposalWithVote) UnmarshalJSON(b []byte) (err error) {
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
