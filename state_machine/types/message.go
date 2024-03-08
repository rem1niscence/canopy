package types

import (
	"github.com/ginchuco/ginchu/crypto"
	lib "github.com/ginchuco/ginchu/types"
	"math/big"
)

const (
	MessageSendName            = "send"
	MessageStakeName           = "stake"
	MessageUnstakeName         = "unstake"
	MessageEditStakeName       = "edit_stake"
	MessagePauseName           = "pause"
	MessageUnpauseName         = "unpause"
	MessageChangeParameterName = "change_parameter"
	MessageDoubleSignName      = "double_sign"
)

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

func (x *MessageSend) SetSigner(signer []byte)     { x.Signer = signer }
func (x *MessageSend) Bytes() ([]byte, lib.ErrorI) { return Marshal(x) }
func (x *MessageSend) Name() string                { return MessageSendName }
func (x *MessageSend) Recipient() string           { return crypto.NewAddressFromBytes(x.ToAddress).String() }

var _ lib.MessageI = &MessageStake{}

func (x *MessageStake) Check() lib.ErrorI {
	if err := checkOutputAddress(x.OutputAddress); err != nil {
		return err
	}
	if err := checkPubKey(x.PublicKey); err != nil {
		return err
	}
	return checkAmount(x.Amount)
}

func (x *MessageStake) SetSigner(signer []byte)     { x.Signer = signer }
func (x *MessageStake) Bytes() ([]byte, lib.ErrorI) { return Marshal(x) }
func (x *MessageStake) Name() string                { return MessageStakeName }
func (x *MessageStake) Recipient() string           { return "" }

var _ lib.MessageI = &MessageEditStake{}

func (x *MessageEditStake) Check() lib.ErrorI {
	if err := checkAddress(x.Address); err != nil {
		return err
	}
	if err := checkOutputAddress(x.OutputAddress); err != nil {
		return err
	}
	if err := checkAmount(x.Amount); err != nil {
		return err
	}
	return nil
}

func (x *MessageEditStake) SetSigner(signer []byte)     { x.Signer = signer }
func (x *MessageEditStake) Bytes() ([]byte, lib.ErrorI) { return Marshal(x) }
func (x *MessageEditStake) Name() string                { return MessageEditStakeName }
func (x *MessageEditStake) Recipient() string           { return "" }

var _ lib.MessageI = &MessageUnstake{}

func (x *MessageUnstake) Check() lib.ErrorI           { return checkAddress(x.Address) }
func (x *MessageUnstake) SetSigner(signer []byte)     { x.Signer = signer }
func (x *MessageUnstake) Bytes() ([]byte, lib.ErrorI) { return Marshal(x) }
func (x *MessageUnstake) Name() string                { return MessageUnstakeName }
func (x *MessageUnstake) Recipient() string           { return "" }

var _ lib.MessageI = &MessagePause{}

func (x *MessagePause) Check() lib.ErrorI           { return checkAddress(x.Address) }
func (x *MessagePause) SetSigner(signer []byte)     { x.Signer = signer }
func (x *MessagePause) Bytes() ([]byte, lib.ErrorI) { return Marshal(x) }
func (x *MessagePause) Name() string                { return MessagePauseName }
func (x *MessagePause) Recipient() string           { return "" }

var _ lib.MessageI = &MessageUnpause{}

func (x *MessageUnpause) Check() lib.ErrorI           { return checkAddress(x.Address) }
func (x *MessageUnpause) SetSigner(signer []byte)     { x.Signer = signer }
func (x *MessageUnpause) Bytes() ([]byte, lib.ErrorI) { return Marshal(x) }
func (x *MessageUnpause) Name() string                { return MessageUnpauseName }
func (x *MessageUnpause) Recipient() string           { return "" }

var _ lib.MessageI = &MessageChangeParameter{}

func (x *MessageChangeParameter) Check() lib.ErrorI {
	if err := checkAddress(x.Owner); err != nil {
		return err
	}
	if x.ParameterKey == "" {
		return ErrParamKeyEmpty()
	}
	if x.ParameterValue == nil {
		return ErrParamValueEmpty()
	}
	return nil
}

func (x *MessageChangeParameter) SetSigner(signer []byte)     { x.Signer = signer }
func (x *MessageChangeParameter) Bytes() ([]byte, lib.ErrorI) { return Marshal(x) }
func (x *MessageChangeParameter) Name() string                { return MessageChangeParameterName }
func (x *MessageChangeParameter) Recipient() string           { return "" }

var _ lib.MessageI = &MessageDoubleSign{}

func (x *MessageDoubleSign) Check() lib.ErrorI {
	if err := checkAddress(x.ReporterAddress); err != nil {
		return err
	}
	if err := checkVote(x.VoteA); err != nil {
		return err
	}
	if err := checkVote(x.VoteB); err != nil {
		return err
	}
	return nil
}

func (x *MessageDoubleSign) SetSigner(signer []byte)     { x.Signer = signer }
func (x *MessageDoubleSign) Bytes() ([]byte, lib.ErrorI) { return Marshal(x) }
func (x *MessageDoubleSign) Name() string                { return MessageDoubleSignName }
func (x *MessageDoubleSign) Recipient() string           { return "" }

func checkAmount(amount string) lib.ErrorI {
	am, err := lib.StringToBigInt(amount)
	if err != nil {
		return err
	}
	if lib.BigLTE(am, big.NewInt(0)) {
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
	if len(publicKey) != crypto.Ed25519PubKeySize {
		return ErrPublicKeySize()
	}
	return nil
}

func checkVote(vote *Vote) lib.ErrorI {
	if vote == nil {
		return ErrVoteEmpty()
	}
	if err := checkPubKey(vote.PublicKey); err != nil {
		return err
	}
	if vote.BlockHash == nil {
		return ErrHashEmpty()
	}
	if len(vote.BlockHash) != crypto.HashSize {
		return ErrHashSize()
	}
	return nil
}
