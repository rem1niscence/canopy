package types

import (
	"bytes"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
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
func (x *MessageSend) Bytes() ([]byte, lib.ErrorI) { return lib.Marshal(x) }
func (x *MessageSend) Name() string                { return MessageSendName }
func (x *MessageSend) Recipient() []byte           { return crypto.NewAddressFromBytes(x.ToAddress).Bytes() }

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
	if err := checkAmount(x.Amount); err != nil {
		return err
	}
	return nil
}

func (x *MessageEditStake) SetSigner(signer []byte)     { x.Signer = signer }
func (x *MessageEditStake) Bytes() ([]byte, lib.ErrorI) { return lib.Marshal(x) }
func (x *MessageEditStake) Name() string                { return MessageEditStakeName }
func (x *MessageEditStake) Recipient() []byte           { return nil }

var _ lib.MessageI = &MessageUnstake{}

func (x *MessageUnstake) Check() lib.ErrorI           { return checkAddress(x.Address) }
func (x *MessageUnstake) SetSigner(signer []byte)     { x.Signer = signer }
func (x *MessageUnstake) Bytes() ([]byte, lib.ErrorI) { return lib.Marshal(x) }
func (x *MessageUnstake) Name() string                { return MessageUnstakeName }
func (x *MessageUnstake) Recipient() []byte           { return nil }

var _ lib.MessageI = &MessagePause{}

func (x *MessagePause) Check() lib.ErrorI           { return checkAddress(x.Address) }
func (x *MessagePause) SetSigner(signer []byte)     { x.Signer = signer }
func (x *MessagePause) Bytes() ([]byte, lib.ErrorI) { return lib.Marshal(x) }
func (x *MessagePause) Name() string                { return MessagePauseName }
func (x *MessagePause) Recipient() []byte           { return nil }

var _ lib.MessageI = &MessageUnpause{}

func (x *MessageUnpause) Check() lib.ErrorI           { return checkAddress(x.Address) }
func (x *MessageUnpause) SetSigner(signer []byte)     { x.Signer = signer }
func (x *MessageUnpause) Bytes() ([]byte, lib.ErrorI) { return lib.Marshal(x) }
func (x *MessageUnpause) Name() string                { return MessageUnpauseName }
func (x *MessageUnpause) Recipient() []byte           { return nil }

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
func (x *MessageChangeParameter) Bytes() ([]byte, lib.ErrorI) { return lib.Marshal(x) }
func (x *MessageChangeParameter) Name() string                { return MessageChangeParameterName }
func (x *MessageChangeParameter) Recipient() []byte           { return nil }

var _ lib.MessageI = &MessageDoubleSign{}

func (x *MessageDoubleSign) Check() lib.ErrorI {
	if err := checkAddress(x.ReporterAddress); err != nil {
		return err
	}
	pk1, err := checkVote(x.VoteA)
	if err != nil {
		return err
	}
	pk2, err := checkVote(x.VoteB)
	if err != nil {
		return err
	}
	// compare votes
	if !pk1.Equals(pk2) {
		return ErrPublicKeysNotEqual()
	}
	if x.VoteA.Height != x.VoteB.Height {
		return ErrHeightsNotEqual()
	}
	if x.VoteA.Round != x.VoteB.Round {
		return ErrRoundsNotEqual()
	}
	if x.VoteA.Type != x.VoteB.Type {
		return ErrVoteTypesNotEqual()
	}
	if bytes.Equal(x.VoteA.BlockHash, x.VoteB.BlockHash) {
		return ErrIdenticalVotes()
	}
	return nil
}

func (x *MessageDoubleSign) SetSigner(signer []byte)     { x.Signer = signer }
func (x *MessageDoubleSign) Bytes() ([]byte, lib.ErrorI) { return lib.Marshal(x) }
func (x *MessageDoubleSign) Name() string                { return MessageDoubleSignName }
func (x *MessageDoubleSign) Recipient() []byte           { return nil }

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

func checkVote(vote *Vote) (crypto.PublicKeyI, lib.ErrorI) {
	if vote == nil {
		return nil, ErrVoteEmpty()
	}
	if err := checkPubKey(vote.PublicKey); err != nil {
		return nil, err
	}
	if vote.BlockHash == nil {
		return nil, ErrHashEmpty()
	}
	if len(vote.BlockHash) != crypto.HashSize {
		return nil, ErrHashSize()
	}
	pk := crypto.NewPublicKeyFromBytes(vote.PublicKey)
	voteCpy := &Vote{
		PublicKey: vote.PublicKey,
		Height:    vote.Height,
		Round:     vote.Round,
		Type:      vote.Type,
		BlockHash: vote.BlockHash,
		Signature: nil,
	}
	msg, err := lib.Marshal(voteCpy)
	if err != nil {
		return nil, err
	}
	if !pk.VerifyBytes(msg, vote.Signature) {
		return nil, ErrInvalidSignature()
	}
	return pk, nil
}
