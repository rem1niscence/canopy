package types

import (
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
	"time"
)

// NewSendTransaction() creates a SendTransaction object in the interface form of TransactionI
func NewSendTransaction(from crypto.PrivateKeyI, to crypto.AddressI, amount, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return NewTransaction(from, &MessageSend{
		FromAddress: from.PublicKey().Address().Bytes(),
		ToAddress:   to.Bytes(),
		Amount:      amount,
	}, fee)
}

// NewStakeTx() creates a StakeTransaction object in the interface form of TransactionI
func NewStakeTx(from crypto.PrivateKeyI, outputAddress crypto.AddressI, netAddress string, committees []uint64, amount, fee uint64, delegate, earlyWithdrawal bool) (lib.TransactionI, lib.ErrorI) {
	return NewTransaction(from, &MessageStake{
		PublicKey:     from.PublicKey().Bytes(),
		Amount:        amount,
		Committees:    committees,
		NetAddress:    netAddress,
		OutputAddress: outputAddress.Bytes(),
		Delegate:      delegate,
		Compound:      !earlyWithdrawal,
	}, fee)
}

// NewEditStakeTx() creates a EditStakeTransaction object in the interface form of TransactionI
func NewEditStakeTx(from crypto.PrivateKeyI, outputAddress crypto.AddressI, netAddress string, committees []uint64, amount, fee uint64, earlyWithdrawal bool) (lib.TransactionI, lib.ErrorI) {
	return NewTransaction(from, &MessageEditStake{
		Address:       from.PublicKey().Address().Bytes(),
		Amount:        amount,
		Committees:    committees,
		NetAddress:    netAddress,
		OutputAddress: outputAddress.Bytes(),
		Compound:      !earlyWithdrawal,
	}, fee)
}

// NewUnstakeTx() creates a UnstakeTransaction object in the interface form of TransactionI
func NewUnstakeTx(from crypto.PrivateKeyI, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return NewTransaction(from, &MessageUnstake{Address: from.PublicKey().Address().Bytes()}, fee)
}

// NewPauseTx() creates a PauseTransaction object in the interface form of TransactionI
func NewPauseTx(from crypto.PrivateKeyI, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return NewTransaction(from, &MessagePause{Address: from.PublicKey().Address().Bytes()}, fee)
}

// NewUnpauseTx() creates a UnpauseTransaction object in the interface form of TransactionI
func NewUnpauseTx(from crypto.PrivateKeyI, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return NewTransaction(from, &MessageUnpause{Address: from.PublicKey().Address().Bytes()}, fee)
}

// NewChangeParamTxUint64() creates a ChangeParamTransaction object (for uint64s) in the interface form of TransactionI
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

// NewChangeParamTxString() creates a ChangeParamTransaction object (for strings) in the interface form of TransactionI
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

// NewDAOTransferTx() creates a DAOTransferTransaction object in the interface form of TransactionI
func NewDAOTransferTx(from crypto.PrivateKeyI, amount, start, end, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return NewTransaction(from, &MessageDAOTransfer{
		Address:     from.PublicKey().Address().Bytes(),
		Amount:      amount,
		StartHeight: start,
		EndHeight:   end,
	}, fee)
}

// NewCertificateResultsTx() creates a CertificateResultsTransaction object in the interface form of TransactionI
func NewCertificateResultsTx(from crypto.PrivateKeyI, qc *lib.QuorumCertificate, fee uint64) (lib.TransactionI, lib.ErrorI) {
	qc.Block = nil // omit the block
	return NewTransaction(from, &MessageCertificateResults{
		Qc: qc,
	}, fee)
}

// NewSubsidyTx() creates a SubsidyTransaction object in the interface form of TransactionI
func NewSubsidyTx(from crypto.PrivateKeyI, amount, committeeId uint64, opCode string, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return NewTransaction(from, &MessageSubsidy{
		Address:     from.PublicKey().Address().Bytes(),
		CommitteeId: committeeId,
		Amount:      amount,
		Opcode:      opCode,
	}, fee)
}

// NewTransaction() creates a Transaction object from a message in the interface form of TransactionI
func NewTransaction(pk crypto.PrivateKeyI, msg lib.MessageI, fee uint64) (lib.TransactionI, lib.ErrorI) {
	a, err := lib.NewAny(msg)
	if err != nil {
		return nil, err
	}
	tx := &lib.Transaction{
		MessageType: msg.Name(),
		Msg:         a,
		Signature:   nil,
		Time:        uint64(time.Now().UnixMicro()), // stateless, prune friendly - replay / hash-collision protection
		Fee:         fee,
	}
	return tx, tx.Sign(pk)
}
