package types

import (
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"time"
)

func NewSendTransaction(from crypto.PrivateKeyI, to crypto.AddressI, amount, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return NewTransaction(from, &MessageSend{
		FromAddress: from.PublicKey().Address().Bytes(),
		ToAddress:   to.Bytes(),
		Amount:      amount,
	}, fee)
}

func NewStakeTx(from crypto.PrivateKeyI, outputAddress crypto.AddressI, netAddress string, committees []*Committee, amount, fee uint64, delegate bool) (lib.TransactionI, lib.ErrorI) {
	return NewTransaction(from, &MessageStake{
		PublicKey:     from.PublicKey().Bytes(),
		Amount:        amount,
		Committees:    committees,
		NetAddress:    netAddress,
		OutputAddress: outputAddress.Bytes(),
		Delegate:      delegate,
	}, fee)
}

func NewEditStakeTx(from crypto.PrivateKeyI, outputAddress crypto.AddressI, netAddress string, committees []*Committee, amount, fee uint64, delegate bool) (lib.TransactionI, lib.ErrorI) {
	return NewTransaction(from, &MessageEditStake{
		Address:       from.PublicKey().Address().Bytes(),
		Amount:        amount,
		Committees:    committees,
		NetAddress:    netAddress,
		OutputAddress: outputAddress.Bytes(),
		Delegate:      delegate,
	}, fee)
}

func NewUnstakeTx(from crypto.PrivateKeyI, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return NewTransaction(from, &MessageUnstake{Address: from.PublicKey().Address().Bytes()}, fee)
}

func NewPauseTx(from crypto.PrivateKeyI, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return NewTransaction(from, &MessagePause{Address: from.PublicKey().Address().Bytes()}, fee)
}

func NewUnpauseTx(from crypto.PrivateKeyI, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return NewTransaction(from, &MessageUnpause{Address: from.PublicKey().Address().Bytes()}, fee)
}

func NewChangeParamTxUint64(from crypto.PrivateKeyI, space, key string, value, start, end, fee uint64) (lib.TransactionI, lib.ErrorI) {
	a, err := lib.NewAny(&lib.UInt64Wrapper{Value: value})
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
	}, fee)
}

func NewChangeParamTxString(from crypto.PrivateKeyI, space, key, value string, start, end, fee uint64) (lib.TransactionI, lib.ErrorI) {
	a, err := lib.NewAny(&lib.StringWrapper{Value: value})
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
	}, fee)
}

func NewDAOTransferTx(from crypto.PrivateKeyI, amount, start, end, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return NewTransaction(from, &MessageDAOTransfer{
		Address:     from.PublicKey().Address().Bytes(),
		Amount:      amount,
		StartHeight: start,
		EndHeight:   end,
	}, fee)
}

func NewProposalTx(from crypto.PrivateKeyI, qc *lib.QuorumCertificate, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return NewTransaction(from, &MessageProposal{
		Qc: qc,
	}, fee)
}

func NewSubsidyTx(from crypto.PrivateKeyI, amount, committeeId uint64, opCode string, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return NewTransaction(from, &MessageSubsidy{
		Address:     from.PublicKey().Address().Bytes(),
		CommitteeId: committeeId,
		Amount:      amount,
		Opcode:      opCode,
	}, fee)
}

func NewTransaction(pk crypto.PrivateKeyI, msg lib.MessageI, fee uint64) (lib.TransactionI, lib.ErrorI) {
	a, err := lib.NewAny(msg)
	if err != nil {
		return nil, err
	}
	tx := &lib.Transaction{
		Type:      msg.Name(),
		Msg:       a,
		Signature: nil,
		Time:      uint64(time.Now().UnixMicro()),
		Fee:       fee,
	}
	return tx, tx.Sign(pk)
}
