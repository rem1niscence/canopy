// nolint:all
package main

import (
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
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
		if err != nil {
			return err
		}
		_, err = f.client.Transaction(tx)
		if err == nil {
			return ErrInvalidParams(SendMsgName, reason)
		}
		f.log.Warnf("Executed invalid %s transaction: %s: %s", SendMsgName, reason, err.Error())
	}
	return nil
}

func (f *Fuzzer) validSendTransaction(from, to *crypto.KeyGroup, account types.Account, fee uint64) lib.ErrorI {
	amountToSend := f.getRandomAmountUpTo(account.Amount - fee)
	account.Amount -= amountToSend + fee
	tx, err := types.NewSendTransaction(from.PrivateKey, to.Address, amountToSend, account.Sequence, fee)
	if err != nil {
		return err
	}
	hash, err := f.client.Transaction(tx)
	if err != nil {
		return err
	}
	f.log.Infof("Executed valid send transaction: %s", *hash)
	f.state.SetAccount(account)
	return nil
}

func (f *Fuzzer) invalidSendSignature(from, to *crypto.KeyGroup, account types.Account, fee uint64) (tx lib.TransactionI, err lib.ErrorI) {
	return newTransactionBadSignature(from.PrivateKey, &types.MessageSend{
		FromAddress: from.Address.Bytes(),
		ToAddress:   to.Address.Bytes(),
		Amount:      f.getRandomAmountUpTo(account.Amount - fee),
	}, account.Sequence, fee)
}

func (f *Fuzzer) invalidSendSequence(from, to *crypto.KeyGroup, account types.Account, fee uint64) (tx lib.TransactionI, err lib.ErrorI) {
	tx, err = types.NewSendTransaction(from.PrivateKey, to.Address, f.getRandomAmountUpTo(account.Amount-fee), f.getBadSequence(account), fee)
	return
}

func (f *Fuzzer) invalidSendFee(from, to *crypto.KeyGroup, account types.Account, fee uint64) (tx lib.TransactionI, err lib.ErrorI) {
	tx, err = types.NewSendTransaction(from.PrivateKey, to.Address, f.getRandomAmountUpTo(account.Amount-fee), account.Sequence, f.getBadFee(fee))
	return
}

func (f *Fuzzer) invalidSendMsg(from, _ *crypto.KeyGroup, account types.Account, fee uint64) (tx lib.TransactionI, err lib.ErrorI) {
	return f.getTxBadMessage(from, types.MessageSendName, account, fee)
}

func (f *Fuzzer) invalidSendSender(from, to *crypto.KeyGroup, account types.Account, fee uint64) (tx lib.TransactionI, err lib.ErrorI) {
	tx, err = types.NewTransaction(from.PrivateKey, &types.MessageSend{
		FromAddress: f.getBadAddress(from).Bytes(),
		ToAddress:   to.Address.Bytes(),
		Amount:      f.getRandomAmountUpTo(account.Amount - fee),
	}, account.Sequence, fee)
	return
}

func (f *Fuzzer) invalidSendRecipient(from, to *crypto.KeyGroup, account types.Account, fee uint64) (tx lib.TransactionI, err lib.ErrorI) {
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
		Amount:      f.getRandomAmountUpTo(account.Amount - fee),
	}, account.Sequence, fee)
	return
}

func (f *Fuzzer) invalidSendAmount(from, to *crypto.KeyGroup, account types.Account, fee uint64) (tx lib.TransactionI, err lib.ErrorI) {
	tx, err = types.NewTransaction(from.PrivateKey, &types.MessageSend{
		FromAddress: from.Address.Bytes(),
		ToAddress:   to.Address.Bytes(),
		Amount:      math.MaxUint64,
	}, account.Sequence, fee)
	return
}
