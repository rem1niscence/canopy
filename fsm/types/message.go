package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"google.golang.org/protobuf/proto"
)

const (
	MessageSendName            = "send"
	MessageStakeName           = "stake"
	MessageUnstakeName         = "unstake"
	MessageEditStakeName       = "edit_stake"
	MessagePauseName           = "pause"
	MessageUnpauseName         = "unpause"
	MessageChangeParameterName = "change_parameter"
	MessageDAOTransferName     = "dao_transfer"
	MessageProposalName        = "proposal"
	MessageSubsidyName         = "subsidy"
)

func init() {
	lib.RegisteredMessages = make(map[string]lib.MessageI)
	lib.RegisteredMessages[MessageSendName] = new(MessageSend)
	lib.RegisteredMessages[MessageStakeName] = new(MessageStake)
	lib.RegisteredMessages[MessageEditStakeName] = new(MessageEditStake)
	lib.RegisteredMessages[MessageUnstakeName] = new(MessageUnstake)
	lib.RegisteredMessages[MessagePauseName] = new(MessagePause)
	lib.RegisteredMessages[MessageUnpauseName] = new(MessageUnpause)
	lib.RegisteredMessages[MessageChangeParameterName] = new(MessageChangeParameter)
	lib.RegisteredMessages[MessageDAOTransferName] = new(MessageDAOTransfer)
	lib.RegisteredMessages[MessageProposalName] = new(MessageProposal)
	lib.RegisteredMessages[MessageSubsidyName] = new(MessageSubsidy)
}

var _ lib.MessageI = &MessageSend{}

func (x *MessageSend) Check() lib.ErrorI {
	if err := checkAddress(x.FromAddress); err != nil {
		return err
	}
	if x.ToAddress == nil {
		return ErrRecipientAddressEmpty()
	}
	if len(x.ToAddress) != crypto.AddressSize {
		return ErrRecipientAddressSize()
	}
	return checkAmount(x.Amount)
}

func (x *MessageSend) Bytes() ([]byte, lib.ErrorI) { return lib.Marshal(x) }
func (x *MessageSend) Name() string                { return MessageSendName }
func (x *MessageSend) New() lib.MessageI           { return new(MessageSend) }
func (x *MessageSend) Recipient() []byte           { return crypto.NewAddressFromBytes(x.ToAddress).Bytes() }

// nolint:all
func (x MessageSend) MarshalJSON() ([]byte, error) {
	return json.Marshal(jsonMessageSend{
		FromAddress: x.FromAddress,
		ToAddress:   x.ToAddress,
		Amount:      x.Amount,
	})
}

func (x *MessageSend) UnmarshalJSON(b []byte) (err error) {
	var j jsonMessageSend
	if err = json.Unmarshal(b, &j); err != nil {
		return
	}
	*x = MessageSend{
		FromAddress: j.FromAddress,
		ToAddress:   j.ToAddress,
		Amount:      j.Amount,
	}
	return
}

type jsonMessageSend struct {
	FromAddress lib.HexBytes `json:"from_address,omitempty"`
	ToAddress   lib.HexBytes `json:"to_address,omitempty"`
	Amount      uint64       `json:"amount,omitempty"`
}

var _ lib.MessageI = &MessageStake{}

func (x *MessageStake) Check() lib.ErrorI {
	if err := checkOutputAddress(x.OutputAddress); err != nil {
		return err
	}
	if err := checkNetAddress(x.NetAddress); err != nil {
		return err
	}
	if err := checkPubKey(x.PublicKey); err != nil {
		return err
	}
	if err := checkCommittees(x.Committees); err != nil {
		return err
	}
	return checkAmount(x.Amount)
}

func (x *MessageStake) Bytes() ([]byte, lib.ErrorI) { return lib.Marshal(x) }
func (x *MessageStake) Name() string                { return MessageStakeName }
func (x *MessageStake) New() lib.MessageI           { return new(MessageStake) }
func (x *MessageStake) Recipient() []byte           { return nil }

// nolint:all
func (x MessageStake) MarshalJSON() ([]byte, error) {
	return json.Marshal(jsonMessageStake{
		PublicKey:     x.PublicKey,
		Amount:        x.Amount,
		Committees:    x.Committees,
		NetAddress:    x.NetAddress,
		OutputAddress: x.OutputAddress,
	})
}

func (x *MessageStake) UnmarshalJSON(b []byte) (err error) {
	var j jsonMessageStake
	if err = json.Unmarshal(b, &j); err != nil {
		return
	}
	*x = MessageStake{
		PublicKey:     j.PublicKey,
		Amount:        j.Amount,
		Committees:    x.Committees,
		NetAddress:    j.NetAddress,
		OutputAddress: j.OutputAddress,
	}
	return
}

type jsonMessageStake struct {
	PublicKey     lib.HexBytes `json:"public_key,omitempty"`
	Amount        uint64       `json:"amount,omitempty"`
	Committees    []*Committee `json:"committees,omitempty"`
	NetAddress    string       `json:"net_address,omitempty"`
	OutputAddress lib.HexBytes `json:"output_address,omitempty"`
}

var _ lib.MessageI = &MessageEditStake{}

func (x *MessageEditStake) Check() lib.ErrorI {
	if err := checkAddress(x.Address); err != nil {
		return err
	}
	if err := checkOutputAddress(x.OutputAddress); err != nil {
		return err
	}
	if err := checkNetAddress(x.NetAddress); err != nil {
		return err
	}
	if err := checkCommittees(x.Committees); err != nil {
		return err
	}
	return checkAmount(x.Amount)
}

func (x *MessageEditStake) Bytes() ([]byte, lib.ErrorI) { return lib.Marshal(x) }
func (x *MessageEditStake) Name() string                { return MessageEditStakeName }
func (x *MessageEditStake) New() lib.MessageI           { return new(MessageEditStake) }
func (x *MessageEditStake) Recipient() []byte           { return nil }

// nolint:all
func (x MessageEditStake) MarshalJSON() ([]byte, error) {
	return json.Marshal(jsonMessageEditStake{
		Address:       x.Address,
		Amount:        x.Amount,
		Committees:    x.Committees,
		NetAddress:    x.NetAddress,
		OutputAddress: x.OutputAddress,
	})
}

func (x *MessageEditStake) UnmarshalJSON(b []byte) (err error) {
	var j jsonMessageEditStake
	if err = json.Unmarshal(b, &j); err != nil {
		return
	}
	*x = MessageEditStake{
		Address:       j.Address,
		Amount:        j.Amount,
		Committees:    x.Committees,
		NetAddress:    j.NetAddress,
		OutputAddress: j.OutputAddress,
	}
	return
}

type jsonMessageEditStake struct {
	Address       lib.HexBytes `json:"address,omitempty"`
	Amount        uint64       `json:"amount,omitempty"`
	Committees    []*Committee `json:"committees,omitempty"`
	NetAddress    string       `json:"net_address,omitempty"`
	OutputAddress lib.HexBytes `json:"output_address,omitempty"`
}

var _ lib.MessageI = &MessageUnstake{}

func (x *MessageUnstake) Check() lib.ErrorI           { return checkAddress(x.Address) }
func (x *MessageUnstake) Bytes() ([]byte, lib.ErrorI) { return lib.Marshal(x) }
func (x *MessageUnstake) Name() string                { return MessageUnstakeName }
func (x *MessageUnstake) New() lib.MessageI           { return new(MessageUnstake) }
func (x *MessageUnstake) Recipient() []byte           { return nil }

// nolint:all
func (x MessageUnstake) MarshalJSON() ([]byte, error) {
	return json.Marshal(jsonHexAddressMsg{Address: x.Address})
}

func (x *MessageUnstake) UnmarshalJSON(b []byte) (err error) {
	var j jsonHexAddressMsg
	if err = json.Unmarshal(b, &j); err != nil {
		return
	}
	*x = MessageUnstake{Address: j.Address}
	return
}

type jsonHexAddressMsg struct {
	Address lib.HexBytes `json:"address,omitempty"`
}

var _ lib.MessageI = &MessagePause{}

func (x *MessagePause) Check() lib.ErrorI           { return checkAddress(x.Address) }
func (x *MessagePause) Bytes() ([]byte, lib.ErrorI) { return lib.Marshal(x) }
func (x *MessagePause) Name() string                { return MessagePauseName }
func (x *MessagePause) New() lib.MessageI           { return new(MessagePause) }
func (x *MessagePause) Recipient() []byte           { return nil }

// nolint:all
func (x MessagePause) MarshalJSON() ([]byte, error) {
	return json.Marshal(jsonHexAddressMsg{Address: x.Address})
}

func (x *MessagePause) UnmarshalJSON(b []byte) (err error) {
	var j jsonHexAddressMsg
	if err = json.Unmarshal(b, &j); err != nil {
		return
	}
	*x = MessagePause{Address: j.Address}
	return
}

var _ lib.MessageI = &MessageUnpause{}

func (x *MessageUnpause) Check() lib.ErrorI           { return checkAddress(x.Address) }
func (x *MessageUnpause) Bytes() ([]byte, lib.ErrorI) { return lib.Marshal(x) }
func (x *MessageUnpause) Name() string                { return MessageUnpauseName }
func (x *MessageUnpause) New() lib.MessageI           { return new(MessageUnpause) }
func (x *MessageUnpause) Recipient() []byte           { return nil }

// nolint:all
func (x MessageUnpause) MarshalJSON() ([]byte, error) {
	return json.Marshal(jsonHexAddressMsg{Address: x.Address})
}

func (x *MessageUnpause) UnmarshalJSON(b []byte) (err error) {
	var j jsonHexAddressMsg
	if err = json.Unmarshal(b, &j); err != nil {
		return
	}
	*x = MessageUnpause{Address: j.Address}
	return
}

var _ lib.MessageI = &MessageChangeParameter{}

func (x *MessageChangeParameter) Check() lib.ErrorI {
	if err := checkAddress(x.Signer); err != nil {
		return err
	}
	if x.ParameterKey == "" {
		return ErrParamKeyEmpty()
	}
	if x.ParameterValue == nil {
		return ErrParamValueEmpty()
	}
	if err := checkStartEndHeight(x); err != nil {
		return err
	}
	return nil
}

// nolint:all
func (x MessageChangeParameter) MarshalJSON() ([]byte, error) {
	a, err := lib.FromAny(x.ParameterValue)
	if err != nil {
		return nil, err
	}
	var parameterValue any
	switch p := a.(type) {
	case *lib.StringWrapper:
		parameterValue = p.Value
	case *lib.UInt64Wrapper:
		parameterValue = p.Value
	default:
		return nil, fmt.Errorf("unknown parameter type %T", p)
	}
	return json.Marshal(jsonMessageChangeParameter{
		ParameterSpace: x.ParameterSpace,
		ParameterKey:   x.ParameterKey,
		ParameterValue: parameterValue,
		StartHeight:    x.StartHeight,
		EndHeight:      x.EndHeight,
		Signer:         x.Signer,
	})
}

func (x *MessageChangeParameter) UnmarshalJSON(b []byte) (err error) {
	var j jsonMessageChangeParameter
	if err = json.Unmarshal(b, &j); err != nil {
		return
	}
	var parameterValue proto.Message
	switch p := j.ParameterValue.(type) {
	case string:
		parameterValue = &lib.StringWrapper{Value: p}
	case uint64:
		parameterValue = &lib.UInt64Wrapper{Value: p}
	case float64:
		parameterValue = &lib.UInt64Wrapper{Value: uint64(p)}
	default:
		return fmt.Errorf("unknown parameter type %T", p)
	}
	a, err := lib.NewAny(parameterValue)
	if err != nil {
		return
	}
	*x = MessageChangeParameter{
		ParameterSpace: j.ParameterSpace,
		ParameterKey:   j.ParameterKey,
		ParameterValue: a,
		StartHeight:    j.StartHeight,
		EndHeight:      j.EndHeight,
		Signer:         j.Signer,
	}
	return
}

type jsonMessageChangeParameter struct {
	ParameterSpace string       `json:"parameter_space,omitempty"`
	ParameterKey   string       `json:"parameter_key,omitempty"`
	ParameterValue any          `json:"parameter_value,omitempty"`
	StartHeight    uint64       `json:"start_height,omitempty"`
	EndHeight      uint64       `json:"end_height,omitempty"`
	Signer         lib.HexBytes `json:"signer,omitempty"`
}

func (x *MessageChangeParameter) Bytes() ([]byte, lib.ErrorI) { return lib.Marshal(x) }
func (x *MessageChangeParameter) Name() string                { return MessageChangeParameterName }
func (x *MessageChangeParameter) New() lib.MessageI           { return new(MessageChangeParameter) }
func (x *MessageChangeParameter) Recipient() []byte           { return nil }

var _ lib.MessageI = &MessageDAOTransfer{}

func (x *MessageDAOTransfer) Check() lib.ErrorI {
	if err := checkAddress(x.Address); err != nil {
		return err
	}
	if err := checkStartEndHeight(x); err != nil {
		return err
	}
	return checkAmount(x.Amount)
}

func (x *MessageDAOTransfer) Bytes() ([]byte, lib.ErrorI) { return lib.Marshal(x) }
func (x *MessageDAOTransfer) Name() string                { return MessageDAOTransferName }
func (x *MessageDAOTransfer) New() lib.MessageI           { return new(MessageDAOTransfer) }
func (x *MessageDAOTransfer) Recipient() []byte           { return nil }

// nolint:all
func (x MessageDAOTransfer) MarshalJSON() ([]byte, error) {
	return json.Marshal(jsonMessageDaoTransfer{
		Address:     x.Address,
		Amount:      x.Amount,
		StartHeight: x.StartHeight,
		EndHeight:   x.EndHeight,
	})
}

func (x *MessageDAOTransfer) UnmarshalJSON(b []byte) (err error) {
	var j jsonMessageDaoTransfer
	if err = json.Unmarshal(b, &j); err != nil {
		return
	}
	*x = MessageDAOTransfer{
		Address:     j.Address,
		Amount:      j.Amount,
		StartHeight: j.StartHeight,
		EndHeight:   j.EndHeight,
	}
	return
}

type jsonMessageDaoTransfer struct {
	Address     lib.HexBytes `json:"address,omitempty"`
	Amount      uint64       `json:"amount,omitempty"`
	StartHeight uint64       `json:"start_height,omitempty"`
	EndHeight   uint64       `json:"end_height,omitempty"`
}

var _ lib.MessageI = &MessageProposal{}

func (x *MessageProposal) Check() lib.ErrorI {
	if x == nil {
		return ErrInvalidProposal()
	}
	if err := x.Qc.CheckBasic(); err != nil {
		return err
	}
	proposal := x.Qc.Proposal
	if proposal == nil || proposal.Block != nil {
		return ErrInvalidProposal()
	}
	if len(x.Qc.ProposalHash) != crypto.HashSize || !bytes.Equal(x.Qc.ProposalHash, x.Qc.Proposal.Hash()) {
		return ErrInvalidProposalHash()
	}
	blockHashLen := len(proposal.BlockHash)
	if blockHashLen != 0 && blockHashLen != crypto.HashSize { // hash size may be zero for external chains
		return ErrInvalidProposal()
	}
	for _, reserved := range ReservedIDs {
		if proposal.Meta.CommitteeId == reserved {
			return ErrInvalidCommitteeID()
		}
	}
	if err := lib.CheckPaymentPercents(proposal.RewardRecipients.PaymentPercents); err != nil {
		return err
	}
	if proposal.RewardRecipients.NumberOfSamples != 0 {
		return ErrInvalidNumOfSamples()
	}
	return nil
}

func (x *MessageProposal) Bytes() ([]byte, lib.ErrorI) { return lib.Marshal(x) }
func (x *MessageProposal) Name() string                { return MessageProposalName }
func (x *MessageProposal) New() lib.MessageI           { return new(MessageProposal) }
func (x *MessageProposal) Recipient() []byte           { return nil }

// nolint:all
func (x MessageProposal) MarshalJSON() ([]byte, error) {
	return json.Marshal(jsonMessageProposal{
		Qc: x.Qc,
	})
}

func (x *MessageProposal) UnmarshalJSON(b []byte) (err error) {
	var j jsonMessageProposal
	if err = json.Unmarshal(b, &j); err != nil {
		return
	}
	*x = MessageProposal{
		Qc: j.Qc,
	}
	return
}

type jsonMessageProposal struct {
	Qc *lib.QuorumCertificate `json:"qc,omitempty"`
}

var _ lib.MessageI = &MessageSubsidy{}

func (x *MessageSubsidy) Check() lib.ErrorI {
	if x == nil {
		return ErrInvalidSubisdy()
	}
	if err := checkAddress(x.Address); err != nil {
		return err
	}
	if len(x.Opcode) > 100 {
		return ErrInvalidOpcode()
	}
	return nil
}

func (x *MessageSubsidy) Bytes() ([]byte, lib.ErrorI) { return lib.Marshal(x) }
func (x *MessageSubsidy) Name() string                { return MessageSubsidyName }
func (x *MessageSubsidy) New() lib.MessageI           { return new(MessageSubsidy) }
func (x *MessageSubsidy) Recipient() []byte           { return nil }

// nolint:all
func (x MessageSubsidy) MarshalJSON() ([]byte, error) {
	return json.Marshal(jsonMessageSubsidy{
		Address:     x.Address,
		CommitteeId: x.CommitteeId,
		Amount:      x.Amount,
		Opcode:      x.Opcode,
	})
}

func (x *MessageSubsidy) UnmarshalJSON(b []byte) (err error) {
	var j jsonMessageSubsidy
	if err = json.Unmarshal(b, &j); err != nil {
		return
	}
	*x = MessageSubsidy{
		Address:     x.Address,
		CommitteeId: j.CommitteeId,
		Amount:      j.Amount,
		Opcode:      j.Opcode,
	}
	return
}

type jsonMessageSubsidy struct {
	Address     lib.HexBytes `json:"address,omitempty"`
	CommitteeId uint64       `json:"committee_id,omitempty"`
	Amount      uint64       `json:"amount,omitempty"`
	Opcode      string       `json:"opcode,omitempty"`
}

func checkAmount(amount uint64) lib.ErrorI {
	if amount == 0 {
		return ErrInvalidAmount()
	}
	return nil
}

func checkAddress(address []byte) lib.ErrorI {
	if address == nil {
		return ErrAddressEmpty()
	}
	if len(address) != crypto.AddressSize {
		return ErrAddressSize()
	}
	return nil
}

func checkNetAddress(netAddress string) lib.ErrorI {
	netAddressLen := len(netAddress)
	if netAddressLen < 1 || netAddressLen > 50 {
		return ErrInvalidNetAddressLen()
	}
	return nil
}

func checkOutputAddress(output []byte) lib.ErrorI {
	if output == nil {
		return ErrOutputAddressEmpty()
	}
	if len(output) != crypto.AddressSize {
		return ErrOutputAddressSize()
	}
	return nil
}

func checkPubKey(publicKey []byte) lib.ErrorI {
	if publicKey == nil {
		return ErrPublicKeyEmpty()
	}
	if len(publicKey) != crypto.BLS12381PubKeySize {
		return ErrPublicKeySize()
	}
	return nil
}

func checkCommittees(committees []*Committee) lib.ErrorI {
	numCommittees := len(committees)
	if numCommittees > 100 || numCommittees == 0 {
		return ErrInvalidNumCommittees()
	}
	total := uint64(0)
	for _, committee := range committees {
		if committee == nil {
			return ErrInvalidCommittee()
		}
		for _, reserved := range ReservedIDs {
			if committee.Id == reserved {
				return ErrInvalidCommitteeID()
			}
		}
		total += committee.StakePercent
		if committee.StakePercent == 0 || total > 100 {
			return ErrInvalidCommitteeStakeDistribution()
		}
	}
	return nil
}

func checkStartEndHeight(proposal GovProposal) lib.ErrorI {
	blockRange := proposal.GetEndHeight() - proposal.GetStartHeight()
	if 100 > blockRange || blockRange <= 0 {
		return ErrInvalidBlockRange()
	}
	return nil
}
