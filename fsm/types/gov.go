package types

import (
	"encoding/json"
	"fmt"
	"github.com/alecthomas/units"
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
	"google.golang.org/protobuf/proto"
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

var (
	// the number of tokens in micro denomination that are initially (before halvenings) minted per block
	InitialTokensPerBlock = 50 * 1000000 // 50 CNPY
	// the number of blocks between each halvening (block reward is cut in half) event
	BlocksPerHalvening = 210000
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
			Retired:         0,
		},
		Validator: &ValidatorParams{
			ValidatorUnstakingBlocks:                    2,
			ValidatorMaxPauseBlocks:                     4380,
			ValidatorDoubleSignSlashPercentage:          10,
			ValidatorNonSignSlashPercentage:             1,
			ValidatorMaxNonSign:                         3,
			ValidatorNonSignWindow:                      5,
			ValidatorMaxCommittees:                      15,
			ValidatorMaxCommitteeSize:                   100,
			ValidatorEarlyWithdrawalPenalty:             20,
			ValidatorDelegateUnstakingBlocks:            2,
			ValidatorMinimumOrderSize:                   1000000000,
			ValidatorStakePercentForSubsidizedCommittee: 33,
			ValidatorMaxSlashPerCommittee:               15,
			ValidatorDelegateRewardPercentage:           10,
			ValidatorBuyDeadlineBlocks:                  15,
			ValidatorBuyOrderFeeMultiplier:              2,
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
		Governance: &GovernanceParams{DaoRewardPercentage: 5},
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
	ParamRetired         = "retired"          // if the chain is marking itself as 'retired' to the base-chain making it forever un-subsidized
)

var _ ParamSpace = &ConsensusParams{}

// Check() validates the consensus params
func (x *ConsensusParams) Check() lib.ErrorI {
	if x.BlockSize < lib.MaxBlockHeaderSize {
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
	case ParamRetired:
		x.Retired = value
	default:
		return ErrUnknownParam()
	}
	return x.Check()
}

// SetString() update a string parameter in the structure
func (x *ConsensusParams) SetString(paramName string, value string) lib.ErrorI {
	switch paramName {
	case ParamProtocolVersion:
		// get new protocol version
		newVersion, err := checkProtocolVersion(value)
		if err != nil {
			return err
		}
		// get old protocol version
		oldVersion, err := checkProtocolVersion(x.ProtocolVersion)
		if err != nil {
			return err
		}
		// ensure new version isn't less than old version
		if newVersion.Version <= oldVersion.Version || newVersion.Height <= oldVersion.Height {
			return ErrInvalidProtocolVersion()
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
	ParamValidatorUnstakingBlocks                    = "validator_unstaking_blocks"                       // number of blocks a committee member must be 'unstaking' for
	ParamValidatorMaxPauseBlocks                     = "validator_max_pause_blocks"                       // maximum blocks a validator may be paused for before force-unstaking
	ParamValidatorNonSignSlashPercentage             = "validator_non_sign_slash_percentage"              // how much a non-signer is slashed if exceeds threshold in window (% of stake)
	ParamValidatorMaxNonSign                         = "validator_max_non_sign"                           // how much a committee member can not sign before being slashed
	ParamValidatorNonSignWindow                      = "validator_non_sign_window"                        // how frequently the non-sign-count is reset
	ParamValidatorDoubleSignSlashPercentage          = "validator_double_sign_slash_percentage"           // how much a double signer is slashed (% of stake)
	ParamValidatorMaxCommittees                      = "validator_max_committees"                         // maximum number of committees a single validator may participate in
	ParamValidatorMaxCommitteeSize                   = "validator_max_committee_size"                     // maximum number of members a committee may have
	ParamValidatorEarlyWithdrawalPenalty             = "validator_early_withdrawal_penalty"               // reduction percentage of non-compounded rewards
	ParamValidatorDelegateUnstakingBlocks            = "validator_delegate_unstaking_blocks"              // number of blocks a delegator must be 'unstaking' for
	ParamValidatorMinimumOrderSize                   = "validator_minimum_order_size"                     // minimum sell tokens in a sell order
	ParamValidatorStakePercentForSubsidizedCommittee = "validator_stake_percent_for_subsidized_committee" // the minimum percentage of total stake needed to be a 'paid committee'
	ParamValidatorMaxSlashPerCommittee               = "validator_max_slash_per_committee"                // the maximum validator slash per committee per block
	ParamValidatorDelegateRewardPercentage           = "validator_delegate_reward_percentage"             // the percentage of the block reward that is awarded to the delegates
	ParamValidatorBuyDeadlineBlocks                  = "validator_buy_deadline_blocks"                    // the amount of blocks a 'buyer' has to complete an order they reserved
	ParamValidatorBuyOrderFeeMultiplier              = "validator_buy_order_fee_multiplier"               // the fee multiplier of the 'send' fee that is required to execute a buy order
)

// Check() validates the Validator params
func (x *ValidatorParams) Check() lib.ErrorI {
	if x.ValidatorUnstakingBlocks == 0 {
		return ErrInvalidParam(ParamValidatorUnstakingBlocks)
	}
	if x.ValidatorMaxPauseBlocks == 0 {
		return ErrInvalidParam(ParamValidatorMaxPauseBlocks)
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
	if x.ValidatorStakePercentForSubsidizedCommittee == 0 || x.ValidatorStakePercentForSubsidizedCommittee > 100 {
		return ErrInvalidParam(ParamValidatorStakePercentForSubsidizedCommittee)
	}
	if x.ValidatorMaxSlashPerCommittee == 0 || x.ValidatorMaxSlashPerCommittee > 100 {
		return ErrInvalidParam(ParamValidatorMaxSlashPerCommittee)
	}
	if x.ValidatorDelegateRewardPercentage == 0 || x.ValidatorDelegateRewardPercentage > 100 {
		return ErrInvalidParam(ParamValidatorDelegateRewardPercentage)
	}
	if x.ValidatorBuyDeadlineBlocks == 0 {
		return ErrInvalidParam(ParamValidatorBuyDeadlineBlocks)
	}
	if x.ValidatorBuyOrderFeeMultiplier == 0 {
		return ErrInvalidParam(ParamValidatorBuyOrderFeeMultiplier)
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
	case ParamValidatorEarlyWithdrawalPenalty:
		x.ValidatorEarlyWithdrawalPenalty = value
	case ParamValidatorDelegateUnstakingBlocks:
		x.ValidatorDelegateUnstakingBlocks = value
	case ParamValidatorMinimumOrderSize:
		x.ValidatorMinimumOrderSize = value
	case ParamValidatorStakePercentForSubsidizedCommittee:
		x.ValidatorStakePercentForSubsidizedCommittee = value
	case ParamValidatorMaxSlashPerCommittee:
		x.ValidatorMaxSlashPerCommittee = value
	case ParamValidatorDelegateRewardPercentage:
		x.ValidatorDelegateRewardPercentage = value
	case ParamValidatorBuyDeadlineBlocks:
		x.ValidatorBuyDeadlineBlocks = value
	case ParamValidatorBuyOrderFeeMultiplier:
		x.ValidatorBuyOrderFeeMultiplier = value
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

/*
	ChainId Polling Feature:
	- Canopy straw polling
	- Internal chain straw polling
	- External chain straw polling

	On-ChainId
	1. Memo field signal 'START POLL' {Hash, Opt:URL, Start, End}
	2. Memo field signal 'VOTE' {Hash, Y/N}

	Off-ChainId
	1. 'poll.json' maintains status of poll at each height
	2. 'poll.json' maintains historical poll stats for 1000 blocks
	3. Query the state of addresses & validators each height to update polling power
	4. '/v1/gov/poll/' shows what's in 'poll.json' + voting power
*/

const (
	minPollEmbedSize     = 80    // minPollEmbedSize is (below) the minimum length string a memo must be to be a poll
	prunePollAfterBlocks = 40320 // prunePollAfterBlocks is the amount of blocks a poll is maintained before being pruned
	maxPollLengthBlocks  = 10000 // maxPollLengthBlocks is the maximum length a poll may run in blocks
)

// ActivePolls is the in-memory representation of the polls.json file
// Contains a list of all active polls
type ActivePolls struct {
	Polls    map[string]map[string]bool `json:"ActivePolls"` // [poll_hash] -> [address hex] -> Vote
	PollMeta map[string]*StartPoll      `json:"PollMeta"`    // [poll_hash] -> StartPoll structure
}

// CheckForPollTransaction() populates the poll.json file from embeds if the embed exists in the memo field
func (p *ActivePolls) CheckForPollTransaction(sender crypto.AddressI, memo string, height uint64) lib.ErrorI {
	if len(memo) < minPollEmbedSize {
		return nil
	}
	// check for start poll embed
	if startPoll, err := checkMemoForStartPoll(height, memo); err == nil {
		p.NewPoll(startPoll)
		return nil
	}
	// check for vote poll embed
	if votePoll, err := checkMemoForVotePoll(memo); err == nil {
		p.VotePoll(sender, votePoll, height)
		return nil
	}
	// no embed
	return nil
}

// NewPoll() creates a new poll from the start poll embed
func (p *ActivePolls) NewPoll(startPoll *StartPoll) {
	if _, exists := p.Polls[startPoll.StartPoll]; exists {
		return
	}
	p.Polls[startPoll.StartPoll] = make(map[string]bool)
	p.PollMeta[startPoll.StartPoll] = startPoll
}

// VotePoll() upserts a vote to a specific poll
func (p *ActivePolls) VotePoll(sender crypto.AddressI, votePoll *VotePoll, height uint64) {
	poll, exists := p.Polls[votePoll.VotePoll]
	if !exists {
		return
	}
	if height > p.PollMeta[votePoll.VotePoll].EndHeight {
		return
	}
	poll[sender.String()] = votePoll.Approve
	p.Polls[votePoll.VotePoll] = poll
}

// Cleanup() adds polls to 'closed' section if past end height and removes any polls that are older than 4 weeks
func (p *ActivePolls) Cleanup(height uint64) {
	// close any polls that are past the end_height
	for hash, poll := range p.PollMeta {
		if int(height-poll.EndHeight) >= prunePollAfterBlocks {
			delete(p.PollMeta, hash)
			delete(p.Polls, hash) // defensive
		}
	}
}

// NewFromFile() creates a new polls object from a file
func (p *ActivePolls) NewFromFile(dataDirPath string) lib.ErrorI {
	return lib.NewJSONFromFile(p, dataDirPath, lib.PollsFilePath)
}

// SaveToFile() persists the polls object to a json file
func (p *ActivePolls) SaveToFile(dataDirPath string) lib.ErrorI {
	return lib.SaveJSONToFile(p, dataDirPath, lib.PollsFilePath)
}

// StartPoll represents the structure for initiating a new poll
// It is used to encode data into JSON format for storing in memo fields
type StartPoll struct {
	StartPoll string `json:"StartPoll"`
	Url       string `json:"URL,omitempty"`
	EndHeight uint64 `json:"EndHeight"`
}

// NewStartPollTransaction() isn't an actual transaction type - rather it's a protocol built on top of send transactions to allow simple straw polling on Canopy.
// This model is plugin specific and does not need to be followed for other chains.
func NewStartPollTransaction(from crypto.PrivateKeyI, pollJSON json.RawMessage, networkId, chainId, fee uint64) (lib.TransactionI, lib.ErrorI) {
	// extract the params from the pollJSON
	extract := struct {
		URL      string `json:"URL"`      // optional
		EndBlock uint64 `json:"endBlock"` // required
	}{}
	if err := lib.UnmarshalJSON(pollJSON, &extract); err != nil {
		return nil, err
	}
	// encode the structure to the memo
	memoBytes, err := lib.MarshalJSON(StartPoll{
		StartPoll: crypto.HashString(pollJSON),
		Url:       extract.URL,
		EndHeight: extract.EndBlock,
	})
	if err != nil {
		return nil, err
	}
	// return the transaction object
	return NewSendTransaction(from, from.PublicKey().Address(), 1, networkId, chainId, fee, string(memoBytes))
}

// VotePoll represents the structure of a voting action on a poll
// It is used to encode data into JSON format for storing in memo fields
type VotePoll struct {
	VotePoll string `json:"VotePoll"`
	Approve  bool   `json:"Approve"`
}

// NewVotePollTransaction() isn't an actual transaction type - rather it's a protocol built on top of send transactions to allow simple straw polling on Canopy.
// This model is plugin specific and does not need to be followed for other chains.
func NewVotePollTransaction(from crypto.PrivateKeyI, pollJSON json.RawMessage, approve bool, networkId, chainId, fee uint64) (lib.TransactionI, lib.ErrorI) {
	// encode the structure to the memo
	memoBytes, err := lib.MarshalJSON(VotePoll{
		VotePoll: crypto.HashString(pollJSON),
		Approve:  approve,
	})
	if err != nil {
		return nil, err
	}
	// return the transaction object
	return NewSendTransaction(from, from.PublicKey().Address(), 1, networkId, chainId, fee, string(memoBytes))
}

// validatePollHash() ensures a poll hash is valid for a poll transaction
func validatePollHash(pollHash string) lib.ErrorI {
	hash, err := lib.StringToBytes(pollHash)
	if err != nil {
		return err
	}
	if len(hash) != crypto.HashSize {
		return lib.ErrHashSize()
	}
	return nil
}

// checkMemoForStartPoll() checks a memo for a start poll embed
func checkMemoForStartPoll(height uint64, memo string) (start *StartPoll, err lib.ErrorI) {
	start = new(StartPoll)
	if err = lib.UnmarshalJSON([]byte(memo), start); err != nil {
		return
	}
	if err = validatePollHash(start.StartPoll); err != nil {
		return
	}
	heightDiff := start.EndHeight - height
	if heightDiff > maxPollLengthBlocks || heightDiff <= 0 {
		return nil, ErrInvalidStartPollHeight()
	}
	return
}

// checkMemoForVotePoll() checks a memo for a vote poll embed
func checkMemoForVotePoll(memo string) (vote *VotePoll, err lib.ErrorI) {
	vote = new(VotePoll)
	if err = lib.UnmarshalJSON([]byte(memo), vote); err != nil {
		return
	}
	if err = validatePollHash(vote.VotePoll); err != nil {
		return
	}
	return
}

// Poll is a list of PollResults keyed by the hash of the proposal
type Poll map[string]PollResult

// PollResult is a structure that represents the current state of a 'Poll' for a 'Proposal'
type PollResult struct {
	ProposalHash string    `json:"proposalHash"` // the hash of the proposal
	ProposalURL  string    `json:"proposalURL"`  // the url of the proposal
	Accounts     VoteStats `json:"accounts"`     // vote statistics for accounts
	Validators   VoteStats `json:"validators"`   // vote statistics for validators
}

// VoteStats: are demonstrative statistics about poll voting
type VoteStats struct {
	ApproveTokens     uint64 `json:"approveTokens"`    // power (tokens) that voted 'yay'
	RejectTokens      uint64 `json:"rejectTokens"`     // power (tokens)  that voted 'nay'
	TotalVotedTokens  uint64 `json:"totalVotedTokens"` // total power (tokens) that already voted
	TotalTokens       uint64 `json:"totalTokens"`      // total power (tokens)  that could possibly vote
	ApprovePercentage uint64 `json:"approvedPercent"`  // percent representation of power (tokens) that voted 'yay' out of total power
	RejectPercentage  uint64 `json:"rejectPercent"`    // percent representation of power (tokens) that voted 'nay' out of total power
	VotedPercentage   uint64 `json:"votedPercent"`     // percent representation of power (tokens) voted out of total power
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
	if err := lib.UnmarshalJSON(b, cp); err != nil || len(cp.Signer) == 0 {
		if err = lib.UnmarshalJSON(b, dt); err != nil {
			return nil, err
		}
		return dt, nil
	}
	return cp, nil
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

// NewFromFile() creates a new polls object from a file
func (p *GovProposals) NewFromFile(dataDirPath string) lib.ErrorI {
	return lib.NewJSONFromFile(p, dataDirPath, lib.ProposalsFilePath)
}

// SaveToFile() persists the polls object to a json file
func (p *GovProposals) SaveToFile(dataDirPath string) lib.ErrorI {
	return lib.SaveJSONToFile(p, dataDirPath, lib.ProposalsFilePath)
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
