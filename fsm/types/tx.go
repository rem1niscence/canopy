package types

import (
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
	"time"
)

// NewSendTransaction() creates a SendTransaction object in the interface form of TransactionI
func NewSendTransaction(from crypto.PrivateKeyI, to crypto.AddressI, amount, fee uint64, memo string) (lib.TransactionI, lib.ErrorI) {
	return NewTransaction(from, &MessageSend{
		FromAddress: from.PublicKey().Address().Bytes(),
		ToAddress:   to.Bytes(),
		Amount:      amount,
	}, fee, memo)
}

// NewStakeTx() creates a StakeTransaction object in the interface form of TransactionI
func NewStakeTx(signer crypto.PrivateKeyI, from lib.HexBytes, outputAddress crypto.AddressI, netAddress string, committees []uint64, amount, fee uint64, delegate, earlyWithdrawal bool, memo string) (lib.TransactionI, lib.ErrorI) {
	return NewTransaction(signer, &MessageStake{
		PublicKey:     from,
		Amount:        amount,
		Committees:    committees,
		NetAddress:    netAddress,
		OutputAddress: outputAddress.Bytes(),
		Delegate:      delegate,
		Compound:      !earlyWithdrawal,
	}, fee, memo)
}

// NewEditStakeTx() creates a EditStakeTransaction object in the interface form of TransactionI
func NewEditStakeTx(signer crypto.PrivateKeyI, from, outputAddress crypto.AddressI, netAddress string, committees []uint64, amount, fee uint64, earlyWithdrawal bool, memo string) (lib.TransactionI, lib.ErrorI) {
	return NewTransaction(signer, &MessageEditStake{
		Address:       from.Bytes(),
		Amount:        amount,
		Committees:    committees,
		NetAddress:    netAddress,
		OutputAddress: outputAddress.Bytes(),
		Compound:      !earlyWithdrawal,
	}, fee, memo)
}

// NewUnstakeTx() creates a UnstakeTransaction object in the interface form of TransactionI
func NewUnstakeTx(signer crypto.PrivateKeyI, from crypto.AddressI, fee uint64, memo string) (lib.TransactionI, lib.ErrorI) {
	return NewTransaction(signer, &MessageUnstake{Address: from.Bytes()}, fee, memo)
}

// NewPauseTx() creates a PauseTransaction object in the interface form of TransactionI
func NewPauseTx(signer crypto.PrivateKeyI, from crypto.AddressI, fee uint64, memo string) (lib.TransactionI, lib.ErrorI) {
	return NewTransaction(signer, &MessagePause{Address: from.Bytes()}, fee, memo)
}

// NewUnpauseTx() creates a UnpauseTransaction object in the interface form of TransactionI
func NewUnpauseTx(signer crypto.PrivateKeyI, from crypto.AddressI, fee uint64, memo string) (lib.TransactionI, lib.ErrorI) {
	return NewTransaction(signer, &MessageUnpause{Address: from.Bytes()}, fee, memo)
}

// NewChangeParamTxUint64() creates a ChangeParamTransaction object (for uint64s) in the interface form of TransactionI
func NewChangeParamTxUint64(from crypto.PrivateKeyI, space, key string, value, start, end, fee uint64, memo string) (lib.TransactionI, lib.ErrorI) {
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
	}, fee, memo)
}

// NewChangeParamTxString() creates a ChangeParamTransaction object (for strings) in the interface form of TransactionI
func NewChangeParamTxString(from crypto.PrivateKeyI, space, key, value string, start, end, fee uint64, memo string) (lib.TransactionI, lib.ErrorI) {
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
	}, fee, memo)
}

// NewDAOTransferTx() creates a DAOTransferTransaction object in the interface form of TransactionI
func NewDAOTransferTx(from crypto.PrivateKeyI, amount, start, end, fee uint64, memo string) (lib.TransactionI, lib.ErrorI) {
	return NewTransaction(from, &MessageDAOTransfer{
		Address:     from.PublicKey().Address().Bytes(),
		Amount:      amount,
		StartHeight: start,
		EndHeight:   end,
	}, fee, memo)
}

// NewCertificateResultsTx() creates a CertificateResultsTransaction object in the interface form of TransactionI
func NewCertificateResultsTx(from crypto.PrivateKeyI, qc *lib.QuorumCertificate, fee uint64, memo string) (lib.TransactionI, lib.ErrorI) {
	return NewTransaction(from, &MessageCertificateResults{Qc: qc}, fee, memo)
}

// NewSubsidyTx() creates a SubsidyTransaction object in the interface form of TransactionI
func NewSubsidyTx(from crypto.PrivateKeyI, amount, committeeId uint64, opCode string, fee uint64, memo string) (lib.TransactionI, lib.ErrorI) {
	return NewTransaction(from, &MessageSubsidy{
		Address:     from.PublicKey().Address().Bytes(),
		CommitteeId: committeeId,
		Amount:      amount,
		Opcode:      opCode,
	}, fee, memo)
}

// NewCreateOrderTx() creates a CreateOrderTransaction object in the interface form of TransactionI
func NewCreateOrderTx(from crypto.PrivateKeyI, sellAmount, requestAmount, committeeId uint64, receiveAddress []byte, fee uint64, memo string) (lib.TransactionI, lib.ErrorI) {
	return NewTransaction(from, &MessageCreateOrder{
		CommitteeId:          committeeId,
		AmountForSale:        sellAmount,
		RequestedAmount:      requestAmount,
		SellerReceiveAddress: receiveAddress,
		SellersSellAddress:   from.PublicKey().Address().Bytes(),
	}, fee, memo)
}

// NewEditOrderTx() creates an EditOrderTransaction object in the interface form of TransactionI
func NewEditOrderTx(from crypto.PrivateKeyI, orderId, sellAmount, requestAmount, committeeId uint64, receiveAddress []byte, fee uint64, memo string) (lib.TransactionI, lib.ErrorI) {
	return NewTransaction(from, &MessageEditOrder{
		OrderId:              orderId,
		CommitteeId:          committeeId,
		AmountForSale:        sellAmount,
		RequestedAmount:      requestAmount,
		SellerReceiveAddress: receiveAddress,
	}, fee, memo)
}

// NewDeleteOrderTx() creates an DeleteOrderTransaction object in the interface form of TransactionI
func NewDeleteOrderTx(from crypto.PrivateKeyI, orderId, committeeId uint64, fee uint64, memo string) (lib.TransactionI, lib.ErrorI) {
	return NewTransaction(from, &MessageDeleteOrder{
		OrderId:     orderId,
		CommitteeId: committeeId,
	}, fee, memo)
}

// NewBuyOrderTx() reserves a sell order using a send-tx and the memo field
func NewBuyOrderTx(from crypto.PrivateKeyI, order lib.BuyOrder, fee uint64) (lib.TransactionI, lib.ErrorI) {
	jsonBytes, err := lib.MarshalJSON(order)
	if err != nil {
		return nil, err
	}
	return NewSendTransaction(from, from.PublicKey().Address(), 1, fee, string(jsonBytes))
}

// NewTransaction() creates a Transaction object from a message in the interface form of TransactionI
func NewTransaction(pk crypto.PrivateKeyI, msg lib.MessageI, fee uint64, memo string) (lib.TransactionI, lib.ErrorI) {
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
		Memo:        memo,
	}
	return tx, tx.Sign(pk)
}
