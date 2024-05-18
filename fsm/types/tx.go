package types

import (
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
)

func NewSendTransaction(from crypto.PrivateKeyI, to crypto.AddressI, amount, seq, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return NewTransaction(from, &MessageSend{
		FromAddress: from.PublicKey().Address().Bytes(),
		ToAddress:   to.Bytes(),
		Amount:      amount,
	}, seq, fee)
}

func NewStakeTx(from crypto.PrivateKeyI, outputAddress crypto.AddressI, netAddress string, amount, seq, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return NewTransaction(from, &MessageStake{
		PublicKey:     from.PublicKey().Bytes(),
		Amount:        amount,
		NetAddress:    netAddress,
		OutputAddress: outputAddress.Bytes(),
	}, seq, fee)
}

func NewEditStakeTx(from crypto.PrivateKeyI, outputAddress crypto.AddressI, netAddress string, amount, seq, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return NewTransaction(from, &MessageEditStake{
		Address:       from.PublicKey().Address().Bytes(),
		Amount:        amount,
		NetAddress:    netAddress,
		OutputAddress: outputAddress.Bytes(),
	}, seq, fee)
}

func NewUnstakeTx(from crypto.PrivateKeyI, seq, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return NewTransaction(from, &MessageUnstake{Address: from.PublicKey().Address().Bytes()}, seq, fee)
}

func NewPauseTx(from crypto.PrivateKeyI, seq, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return NewTransaction(from, &MessagePause{Address: from.PublicKey().Address().Bytes()}, seq, fee)
}

func NewUnpauseTx(from crypto.PrivateKeyI, seq, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return NewTransaction(from, &MessageUnpause{Address: from.PublicKey().Address().Bytes()}, seq, fee)
}

func NewChangeParamTxUint64(from crypto.PrivateKeyI, space, key string, value, start, end, seq, fee uint64) (lib.TransactionI, lib.ErrorI) {
	a, err := lib.ToAny(&lib.UInt64Wrapper{Value: value})
	if err != nil {
		return nil, err
	}
	return NewTransaction(from, &MessageChangeParameter{
		ParameterSpace: space,
		ParameterKey:   key,
		ParameterValue: a,
		StartHeight:    start,
		EndHeight:      end,
		Signer:         from.PublicKey().Address().Bytes(),
	}, seq, fee)
}

func NewChangeParamTxString(from crypto.PrivateKeyI, space, key, value string, start, end, seq, fee uint64) (lib.TransactionI, lib.ErrorI) {
	a, err := lib.ToAny(&lib.StringWrapper{Value: value})
	if err != nil {
		return nil, err
	}
	return NewTransaction(from, &MessageChangeParameter{
		ParameterSpace: space,
		ParameterKey:   key,
		ParameterValue: a,
		StartHeight:    start,
		EndHeight:      end,
		Signer:         from.PublicKey().Address().Bytes(),
	}, seq, fee)
}

func NewDAOTransferTx(from crypto.PrivateKeyI, amount, start, end, seq, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return NewTransaction(from, &MessageDAOTransfer{
		Address:     from.PublicKey().Address().Bytes(),
		Amount:      amount,
		StartHeight: start,
		EndHeight:   end,
	}, seq, fee)
}

func NewTransaction(pk crypto.PrivateKeyI, msg lib.MessageI, seq, fee uint64) (lib.TransactionI, lib.ErrorI) {
	a, err := lib.ToAny(msg)
	if err != nil {
		return nil, err
	}
	tx := &lib.Transaction{
		Type:      msg.Name(),
		Msg:       a,
		Signature: nil,
		Sequence:  seq,
		Fee:       fee,
	}
	return tx, tx.Sign(pk)
}
