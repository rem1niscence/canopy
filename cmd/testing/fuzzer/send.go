package main

import (
	"fmt"
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"google.golang.org/protobuf/proto"
	"math"
	"math/rand"
)

func (f *Fuzzer) SendTransaction() lib.ErrorI {
	from, to := f.getRandomKeyGroup(), f.getRandomKeyGroup()
	account, fee := f.getAccount(from.Address), f.getFees().MessageSendFee
	if account.Amount < fee {
		return nil
	}
	account.Sequence++
	if i := rand.Intn(100); i >= f.config.PercentInvalidTransactions {
		return f.validSendTransaction(from, to, account, fee)
	} else {
		var tx lib.TransactionI
		var err lib.ErrorI
		var reason string
		switch rand.Intn(7) {
		case 0: // invalid signature
			tx, err = f.invalidSendSignature(from, to, account, fee)
			reason = BadSigReason
		case 1: // invalid sequence
			tx, err = f.invalidSendSequence(from, to, account, fee)
			reason = BadSeqReason
		case 2: // invalid fee
			tx, err = f.invalidSendFee(from, to, account, fee)
			reason = BadFeeReason
		case 3: // invalid message
			tx, err = f.invalidSendMsg(from, to, account, fee)
			reason = BadMessageReason
		case 4: // invalid from address
			tx, err = f.invalidSendSender(from, to, account, fee)
			reason = BadSenderReason
		case 5: // invalid to address
			tx, err = f.invalidSendRecipient(from, to, account, fee)
			reason = BadRecReason
		case 6: // invalid amount
			tx, err = f.invalidSendAmount(from, to, account, fee)
			reason = BadAmountReason
		}
		_, err = f.client.Transaction(tx)
		if err == nil {
			return ErrInvalidParams(SendMsgName, reason)
		}
		f.log.Warnf("Executed invalid %s transaction: %s: %s", SendMsgName, reason, err.Error())
	}
	return nil
}

func (f *Fuzzer) validSendTransaction(from, to *crypto.KeyGroup, account *types.Account, fee uint64) lib.ErrorI {
	amountToSend := f.getRandomAmountUpTo(account.Amount - fee)
	fmt.Printf("sending amount: %d\n", amountToSend+fee)
	account.Amount -= amountToSend + fee
	f.state.SetAccount(account)
	tx, err := types.NewSendTransaction(from.PrivateKey, to.Address, amountToSend, account.Sequence, fee)
	if err != nil {
		return err
	}
	hash, err := f.client.Transaction(tx)
	if err != nil {
		return err
	}
	f.log.Infof("Executed valid send transaction: %s", *hash)
	return nil
}

func (f *Fuzzer) invalidSendSignature(from, to *crypto.KeyGroup, account *types.Account, fee uint64) (tx lib.TransactionI, err lib.ErrorI) {
	return newTransactionBadSignature(from.PrivateKey, &types.MessageSend{
		FromAddress: from.Address.Bytes(),
		ToAddress:   to.Address.Bytes(),
		Amount:      f.getRandomAmountUpTo(account.Amount),
	}, account.Sequence, fee)
}

func (f *Fuzzer) invalidSendSequence(from, to *crypto.KeyGroup, account *types.Account, fee uint64) (tx lib.TransactionI, err lib.ErrorI) {
	var sequence uint64
	switch rand.Intn(3) {
	case 0:
		sequence = account.Sequence - 1
	default:
		sequence = 0
	}
	tx, err = types.NewSendTransaction(from.PrivateKey, to.Address, f.getRandomAmountUpTo(account.Amount), sequence, fee)
	return
}

func (f *Fuzzer) invalidSendFee(from, to *crypto.KeyGroup, account *types.Account, fee uint64) (tx lib.TransactionI, err lib.ErrorI) {
	switch rand.Intn(3) {
	case 0:
		fee -= 1
	case 1:
		fee = math.MaxUint64
	case 2:
		fee = 0
	}
	tx, err = types.NewSendTransaction(from.PrivateKey, to.Address, f.getRandomAmountUpTo(account.Amount), account.Sequence, math.MaxUint64)
	return
}

func (f *Fuzzer) invalidSendMsg(from, _ *crypto.KeyGroup, account *types.Account, fee uint64) (tx lib.TransactionI, err lib.ErrorI) {
	var msg proto.Message
	switch rand.Intn(2) {
	case 0:
		msg = nil
	case 1:
		msg = &lib.UInt64Wrapper{}
	}
	a, err := lib.ToAny(msg)
	if err != nil {
		return nil, err
	}
	tx = &lib.Transaction{
		Type:      types.MessageSendName,
		Msg:       a,
		Signature: nil,
		Sequence:  account.Sequence,
		Fee:       fee,
	}
	err = tx.(*lib.Transaction).Sign(from.PrivateKey)
	return
}

func (f *Fuzzer) invalidSendSender(from, to *crypto.KeyGroup, account *types.Account, fee uint64) (tx lib.TransactionI, err lib.ErrorI) {
	switch rand.Intn(4) {
	case 0:
		pk, _ := crypto.NewEd25519PrivateKey()
		tx, err = types.NewTransaction(from.PrivateKey, &types.MessageSend{
			FromAddress: pk.PublicKey().Address().Bytes(),
			ToAddress:   to.Address.Bytes(),
			Amount:      f.getRandomAmountUpTo(account.Amount),
		}, account.Sequence, fee)
	case 1:
		tx, err = types.NewTransaction(from.PrivateKey, &types.MessageSend{
			FromAddress: nil,
			ToAddress:   to.Address.Bytes(),
			Amount:      f.getRandomAmountUpTo(account.Amount),
		}, account.Sequence, fee)
	case 2:
		pk, _ := crypto.NewEd25519PrivateKey()
		tx, err = types.NewTransaction(pk, &types.MessageSend{
			FromAddress: from.Address.Bytes(),
			ToAddress:   to.Address.Bytes(),
			Amount:      f.getRandomAmountUpTo(account.Amount),
		}, account.Sequence, fee)
	}
	return
}

func (f *Fuzzer) invalidSendRecipient(from, to *crypto.KeyGroup, account *types.Account, fee uint64) (tx lib.TransactionI, err lib.ErrorI) {
	var toAddress []byte
	switch rand.Intn(3) {
	case 0:
		toAddress = to.PublicKey.Bytes()
	case 1:
		toAddress = nil
	case 2:
		toAddress = []byte("foo")
	}
	tx, err = types.NewTransaction(from.PrivateKey, &types.MessageSend{
		FromAddress: from.Address.Bytes(),
		ToAddress:   toAddress,
		Amount:      f.getRandomAmountUpTo(account.Amount),
	}, account.Sequence, fee)
	return
}

func (f *Fuzzer) invalidSendAmount(from, to *crypto.KeyGroup, account *types.Account, fee uint64) (tx lib.TransactionI, err lib.ErrorI) {
	tx, err = types.NewTransaction(from.PrivateKey, &types.MessageSend{
		FromAddress: from.Address.Bytes(),
		ToAddress:   to.Address.Bytes(),
		Amount:      math.MaxUint64,
	}, account.Sequence, fee)
	return
}
