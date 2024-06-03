package main

import (
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"math/rand"
)

func (f *Fuzzer) PauseTransaction() lib.ErrorI {
	from := f.getRandomKeyGroup()
	account, val := f.getAccount(from.Address), f.getValidator(from.Address)
	if val.Address == nil || val.UnstakingHeight != 0 {
		return nil
	}
	account.Sequence++
	if val.MaxPausedHeight == 0 {
		return nil
	}
	fee := f.getFees().MessagePauseFee
	if account.Amount < fee {
		return nil
	}
	return f.handlePauseTransaction(from, account, val, fee)
}

func (f *Fuzzer) UnpauseTransaction() lib.ErrorI {
	from := f.getRandomKeyGroup()
	account, val := f.getAccount(from.Address), f.getValidator(from.Address)
	if val.Address == nil || val.UnstakingHeight != 0 {
		return nil
	}
	account.Sequence++
	if val.MaxPausedHeight != 0 {
		return nil
	}
	fee := f.getFees().MessageUnpauseFee
	if account.Amount < fee {
		return nil
	}
	return f.handleUnpauseTransaction(from, account, val, fee)
}

func (f *Fuzzer) handlePauseTransaction(from *crypto.KeyGroup, account types.Account, val types.Validator, fee uint64) lib.ErrorI {
	if i := rand.Intn(100); i >= f.config.PercentInvalidTransactions {
		return f.validPauseTransaction(from, account, val, fee)
	} else {
		var tx lib.TransactionI
		var err lib.ErrorI
		var reason string
		switch rand.Intn(5) {
		case 0: // invalid signature
			tx, err = f.invalidPauseSignature(from, account, val, fee)
			reason = BadSigReason
		case 1: // invalid sequence
			tx, err = f.invalidPauseSequence(from, account, val, fee)
			reason = BadSeqReason
		case 2: // invalid fee
			tx, err = f.invalidPauseFee(from, account, val, fee)
			reason = BadFeeReason
		case 3: // invalid message
			tx, err = f.invalidPauseMsg(from, account, val, fee)
			reason = BadMessageReason
		case 4: // invalid from address
			tx, err = f.invalidPauseSender(from, account, val, fee)
			reason = BadSenderReason
		}
		if err != nil {
			return err
		}
		_, err = f.client.Transaction(tx)
		if err == nil {
			return ErrInvalidParams(PauseMsgName, reason)
		}
		f.log.Warnf("Executed invalid %s transaction: %s: %s", PauseMsgName, reason, err.Error())
	}
	return nil
}

func (f *Fuzzer) handleUnpauseTransaction(from *crypto.KeyGroup, account types.Account, val types.Validator, fee uint64) lib.ErrorI {
	if i := rand.Intn(100); i >= f.config.PercentInvalidTransactions {
		return f.validUnpauseTransaction(from, account, val, fee)
	} else {
		var tx lib.TransactionI
		var err lib.ErrorI
		var reason string
		switch rand.Intn(5) {
		case 0: // invalid signature
			tx, err = f.invalidUnpauseSignature(from, account, val, fee)
			reason = BadSigReason
		case 1: // invalid sequence
			tx, err = f.invalidUnpauseSequence(from, account, val, fee)
			reason = BadSeqReason
		case 2: // invalid fee
			tx, err = f.invalidUnpauseFee(from, account, val, fee)
			reason = BadFeeReason
		case 3: // invalid message
			tx, err = f.invalidUnpauseMsg(from, account, val, fee)
			reason = BadMessageReason
		case 4: // invalid from address
			tx, err = f.invalidUnpauseSender(from, account, val, fee)
			reason = BadSenderReason
		}
		if err != nil {
			return err
		}
		_, err = f.client.Transaction(tx)
		if err == nil {
			return ErrInvalidParams(UnpauseMsgName, reason)
		}
		f.log.Warnf("Executed invalid %s transaction: %s: %s", UnpauseMsgName, reason, err.Error())
	}
	return nil
}

func (f *Fuzzer) validPauseTransaction(from *crypto.KeyGroup, acc types.Account, val types.Validator, fee uint64) lib.ErrorI {
	val.MaxPausedHeight = 1
	tx, err := types.NewPauseTx(from.PrivateKey, acc.Sequence, fee)
	if err != nil {
		return err
	}
	hash, err := f.client.Transaction(tx)
	if err != nil {
		return err
	}
	f.log.Infof("Executed valid pause transaction: %s", *hash)
	f.state.SetAccount(acc)
	f.state.SetValidator(val)
	return nil
}

func (f *Fuzzer) validUnpauseTransaction(from *crypto.KeyGroup, acc types.Account, val types.Validator, fee uint64) lib.ErrorI {
	val.MaxPausedHeight = 0
	tx, err := types.NewUnpauseTx(from.PrivateKey, acc.Sequence, fee)
	if err != nil {
		return err
	}
	hash, err := f.client.Transaction(tx)
	if err != nil {
		return err
	}
	f.log.Infof("Executed valid unpause transaction: %s", *hash)
	f.state.SetAccount(acc)
	f.state.SetValidator(val)
	return nil
}

func (f *Fuzzer) invalidPauseSignature(from *crypto.KeyGroup, account types.Account, _ types.Validator, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return newTransactionBadSignature(from.PrivateKey, &types.MessagePause{
		Address: from.Address.Bytes(),
	}, account.Sequence, fee)
}

func (f *Fuzzer) invalidUnpauseSignature(from *crypto.KeyGroup, account types.Account, _ types.Validator, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return newTransactionBadSignature(from.PrivateKey, &types.MessageUnpause{
		Address: from.Address.Bytes(),
	}, account.Sequence, fee)
}

func (f *Fuzzer) invalidPauseSequence(from *crypto.KeyGroup, account types.Account, _ types.Validator, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return types.NewPauseTx(from.PrivateKey, f.getBadSequence(account), f.getBadFee(fee))
}

func (f *Fuzzer) invalidUnpauseSequence(from *crypto.KeyGroup, account types.Account, _ types.Validator, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return types.NewUnpauseTx(from.PrivateKey, f.getBadSequence(account), f.getBadFee(fee))
}

func (f *Fuzzer) invalidPauseFee(from *crypto.KeyGroup, account types.Account, _ types.Validator, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return types.NewPauseTx(from.PrivateKey, account.Sequence, f.getBadFee(fee))
}

func (f *Fuzzer) invalidUnpauseFee(from *crypto.KeyGroup, account types.Account, _ types.Validator, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return types.NewUnpauseTx(from.PrivateKey, account.Sequence, f.getBadFee(fee))
}

func (f *Fuzzer) invalidPauseMsg(from *crypto.KeyGroup, account types.Account, _ types.Validator, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return f.getTxBadMessage(from, types.MessagePauseName, account, fee)
}

func (f *Fuzzer) invalidUnpauseMsg(from *crypto.KeyGroup, account types.Account, _ types.Validator, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return f.getTxBadMessage(from, types.MessageUnpauseName, account, fee)
}

func (f *Fuzzer) invalidPauseSender(from *crypto.KeyGroup, account types.Account, _ types.Validator, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return types.NewTransaction(from.PrivateKey, &types.MessagePause{
		Address: f.getBadAddress(from).Bytes(),
	}, account.Sequence, fee)
}

func (f *Fuzzer) invalidUnpauseSender(from *crypto.KeyGroup, account types.Account, _ types.Validator, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return types.NewTransaction(from.PrivateKey, &types.MessageUnpause{
		Address: f.getBadAddress(from).Bytes(),
	}, account.Sequence, fee)
}
