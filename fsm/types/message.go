package types

import (
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
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

func (x *MessageSend) Bytes() ([]byte, lib.ErrorI) { return lib.Marshal(x) }
func (x *MessageSend) Name() string                { return MessageSendName }
func (x *MessageSend) Recipient() []byte           { return crypto.NewAddressFromBytes(x.ToAddress).Bytes() }

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
	return checkAmount(x.Amount)
}

func (x *MessageStake) Bytes() ([]byte, lib.ErrorI) { return lib.Marshal(x) }
func (x *MessageStake) Name() string                { return MessageStakeName }
func (x *MessageStake) Recipient() []byte           { return nil }

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
	if err := checkAmount(x.Amount); err != nil {
		return err
	}
	return nil
}

func (x *MessageEditStake) Bytes() ([]byte, lib.ErrorI) { return lib.Marshal(x) }
func (x *MessageEditStake) Name() string                { return MessageEditStakeName }
func (x *MessageEditStake) Recipient() []byte           { return nil }

var _ lib.MessageI = &MessageUnstake{}

func (x *MessageUnstake) Check() lib.ErrorI           { return checkAddress(x.Address) }
func (x *MessageUnstake) Bytes() ([]byte, lib.ErrorI) { return lib.Marshal(x) }
func (x *MessageUnstake) Name() string                { return MessageUnstakeName }
func (x *MessageUnstake) Recipient() []byte           { return nil }

var _ lib.MessageI = &MessagePause{}

func (x *MessagePause) Check() lib.ErrorI           { return checkAddress(x.Address) }
func (x *MessagePause) Bytes() ([]byte, lib.ErrorI) { return lib.Marshal(x) }
func (x *MessagePause) Name() string                { return MessagePauseName }
func (x *MessagePause) Recipient() []byte           { return nil }

var _ lib.MessageI = &MessageUnpause{}

func (x *MessageUnpause) Check() lib.ErrorI           { return checkAddress(x.Address) }
func (x *MessageUnpause) Bytes() ([]byte, lib.ErrorI) { return lib.Marshal(x) }
func (x *MessageUnpause) Name() string                { return MessageUnpauseName }
func (x *MessageUnpause) Recipient() []byte           { return nil }

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
	return nil
}

func (x *MessageChangeParameter) Bytes() ([]byte, lib.ErrorI) { return lib.Marshal(x) }
func (x *MessageChangeParameter) Name() string                { return MessageChangeParameterName }
func (x *MessageChangeParameter) Recipient() []byte           { return nil }

var _ lib.MessageI = &MessageDAOTransfer{}

func (x *MessageDAOTransfer) Check() lib.ErrorI {
	if err := checkAddress(x.Address); err != nil {
		return err
	}
	return checkAmount(x.Amount)
}

func (x *MessageDAOTransfer) Bytes() ([]byte, lib.ErrorI) { return lib.Marshal(x) }
func (x *MessageDAOTransfer) Name() string                { return MessageDAOTransferName }
func (x *MessageDAOTransfer) Recipient() []byte           { return nil }

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
	if len(publicKey) != crypto.Ed25519PubKeySize {
		return ErrPublicKeySize()
	}
	return nil
}
