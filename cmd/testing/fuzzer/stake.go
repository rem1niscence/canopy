// nolint:all
package main

import (
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"math/rand"
)

func (f *Fuzzer) StakeTransaction() lib.ErrorI {
	from := f.getRandomKeyGroup()
	account, val := f.getAccount(from.Address), f.getValidator(from.Address)
	if val.Address != nil {
		return nil
	}
	fee := f.getFees().MessageStakeFee
	if account.Amount < fee {
		return nil
	}
	return f.handleStakeTransaction(from, account, fee)
}

func (f *Fuzzer) EditStakeTransaction() lib.ErrorI {
	from := f.getRandomKeyGroup()
	account, val := f.getAccount(from.Address), f.getValidator(from.Address)
	if val.Address == nil || val.UnstakingHeight != 0 {
		return nil
	}
	fee := f.getFees().MessageEditStakeFee
	if account.Amount < fee {
		return nil
	}
	return f.handleEditStakeTransaction(from, account, val, fee)
}

func (f *Fuzzer) UnstakeTransaction() lib.ErrorI {
	from := f.getRandomKeyGroup()
	if f.config.PrivateKeys[0].String() == from.PrivateKey.String() { // never unstake the first validator
		return nil
	}
	account, val := f.getAccount(from.Address), f.getValidator(from.Address)
	if val.Address == nil || val.UnstakingHeight != 0 {
		return nil
	}
	return f.handleUnstakeTransaction(from, account, val)
}

func (f *Fuzzer) handleStakeTransaction(from *crypto.KeyGroup, account types.Account, fee uint64) lib.ErrorI {
	if _, err := f.getRandomStakeAmount(account, fee); err != nil {
		return nil // insufficient funds
	}
	if i := rand.Intn(100); i >= f.config.PercentInvalidTransactions {
		return f.validStakeTransaction(from, account, fee)
	} else {
		var tx lib.TransactionI
		var err lib.ErrorI
		var reason string
		switch rand.Intn(8) {
		case 0: // invalid signature
			tx, err = f.invalidStakeSignature(from, account, fee)
			reason = BadSigReason
		case 1: // invalid time
			tx, err = f.invalidStakeTime(from, account, fee)
			reason = BadSeqReason
		case 2: // invalid fee
			tx, err = f.invalidStakeFee(from, account, fee)
			reason = BadFeeReason
		case 3: // invalid message
			tx, err = f.invalidStakeMsg(from, account, fee)
			reason = BadMessageReason
		case 4: // invalid from address
			tx, err = f.invalidStakeAddress(from, account, fee)
			reason = BadSenderReason
		case 5: // invalid net address
			tx, err = f.invalidStakeNetAddress(from, account, fee)
			reason = BadNetAddrReason
		case 6: // invalid output address
			tx, err = f.invalidStakeOutput(from, account, fee)
			reason = BadOutputAddrReason
		case 7: // invalid stake amount
			tx, err = f.invalidStakeAmount(from, account, fee)
			reason = BadAmountReason
		}
		if err != nil {
			return err
		}
		_, err = f.client.Transaction(tx)
		if err == nil {
			return ErrInvalidParams(StakeMsgName, reason)
		}
		f.log.Warnf("Executed invalid %s transaction: %s: %s", StakeMsgName, reason, err.Error())
	}
	return nil
}

func (f *Fuzzer) handleEditStakeTransaction(from *crypto.KeyGroup, account types.Account, val types.Validator, fee uint64) lib.ErrorI {
	if val.UnstakingHeight != 0 {
		return nil
	}
	if i := rand.Intn(100); i >= f.config.PercentInvalidTransactions {
		return f.validEditStakeTx(from, val, account, fee)
	} else {
		var tx lib.TransactionI
		var err lib.ErrorI
		var reason string
		switch rand.Intn(8) {
		case 0: // invalid signature
			tx, err = f.invalidEditStakeSignature(from, val, account, fee)
			reason = BadSigReason
		case 1: // invalid time
			tx, err = f.invalidEditStakeTime(from, val, account, fee)
			reason = BadSeqReason
		case 2: // invalid fee
			tx, err = f.invalidEditStakeFee(from, val, account, fee)
			reason = BadFeeReason
		case 3: // invalid message
			tx, err = f.invalidEditStakeMsg(from, val, account, fee)
			reason = BadMessageReason
		case 4: // invalid from address
			tx, err = f.invalidEditStakeAddress(from, val, account, fee)
			reason = BadSenderReason
		case 5: // invalid net address
			tx, err = f.invalidEditStakeNetAddress(from, val, account, fee)
			reason = BadNetAddrReason
		case 6: // invalid output address
			tx, err = f.invalidEditStakeOutput(from, val, account, fee)
			reason = BadOutputAddrReason
		case 7: // invalid stake amount
			tx, err = f.invalidEditStakeAmount(from, val, account, fee)
			reason = BadAmountReason
		}
		if err != nil {
			return err
		}
		_, err = f.client.Transaction(tx)
		if err == nil {
			return ErrInvalidParams(EditStakeMsgName, reason)
		}
		f.log.Warnf("Executed invalid %s transaction: %s: %s", EditStakeMsgName, reason, err.Error())
	}
	return nil
}

func (f *Fuzzer) handleUnstakeTransaction(from *crypto.KeyGroup, account types.Account, val types.Validator) lib.ErrorI {
	fee := f.getFees().MessageUnstakeFee
	if i := rand.Intn(100); i >= f.config.PercentInvalidTransactions {
		return f.validUnstakeTransaction(from, account, val, fee)
	} else {
		var tx lib.TransactionI
		var err lib.ErrorI
		var reason string
		switch rand.Intn(5) {
		case 0: // invalid signature
			tx, err = f.invalidUnstakeSignature(from, account, val, fee)
			reason = BadSigReason
		case 1: // invalid sequence
			tx, err = f.invalidUnstakeTime(from, account, val, fee)
			reason = BadSeqReason
		case 2: // invalid fee
			tx, err = f.invalidUnstakeFee(from, account, val, fee)
			reason = BadFeeReason
		case 3: // invalid message
			tx, err = f.invalidUnstakeMsg(from, account, val, fee)
			reason = BadMessageReason
		case 4: // invalid from address
			tx, err = f.invalidUnstakeSender(from, account, val, fee)
			reason = BadSenderReason
		}
		if err != nil {
			return err
		}
		_, err = f.client.Transaction(tx)
		if err == nil {
			return ErrInvalidParams(UnstakeMsgName, reason)
		}
		f.log.Warnf("Executed invalid %s transaction: %s: %s", UnstakeMsgName, reason, err.Error())
	}
	return nil
}

func (f *Fuzzer) validStakeTransaction(from *crypto.KeyGroup, acc types.Account, fee uint64) lib.ErrorI {
	amount, err := f.getRandomStakeAmount(acc, fee)
	if err != nil {
		return err
	}
	acc.Amount -= amount + fee
	output := f.getRandomOutputAddr(from)
	val := types.Validator{
		Address:      from.Address.Bytes(),
		PublicKey:    from.PublicKey.Bytes(),
		NetAddress:   f.getRandomNetAddr(),
		StakedAmount: amount,
		Output:       output.Bytes(),
	}
	tx, err := types.NewStakeTx(from.PrivateKey, output, val.NetAddress, val.StakedAmount, f.getTxTime(), fee)
	if err != nil {
		return err
	}
	hash, err := f.client.Transaction(tx)
	if err != nil {
		return err
	}
	f.log.Infof("Executed valid stake transaction: %s", *hash)
	f.state.SetValidator(val)
	f.state.SetAccount(acc)
	return nil
}

func (f *Fuzzer) validEditStakeTx(from *crypto.KeyGroup, val types.Validator, acc types.Account, fee uint64) lib.ErrorI {
	amount := f.getRandomAmountUpTo(acc.Amount - fee)
	amountToStake := val.StakedAmount + amount
	acc.Amount -= amount + fee
	output := from.Address
	switch rand.Intn(2) {
	case 0:
		pub, _ := crypto.NewED25519PublicKey()
		output = pub.Address()
	}
	tx, err := types.NewEditStakeTx(from.PrivateKey, output, f.getRandomNetAddr(), amountToStake, f.getTxTime(), fee)
	if err != nil {
		return err
	}
	hash, err := f.client.Transaction(tx)
	if err != nil {
		return err
	}
	f.log.Infof("Executed valid edit_stake transaction: %s", *hash)
	f.state.SetValidator(val)
	f.state.SetAccount(acc)
	return nil
}

func (f *Fuzzer) validUnstakeTransaction(from *crypto.KeyGroup, acc types.Account, val types.Validator, fee uint64) lib.ErrorI {
	val.UnstakingHeight = 1
	tx, err := types.NewUnstakeTx(from.PrivateKey, f.getTxTime(), fee)
	if err != nil {
		return err
	}
	hash, err := f.client.Transaction(tx)
	if err != nil {
		return err
	}
	f.log.Infof("Executed valid unstake transaction: %s", *hash)
	f.state.SetAccount(acc)
	f.state.SetValidator(val)
	return nil
}

func (f *Fuzzer) invalidStakeSignature(from *crypto.KeyGroup, account types.Account, fee uint64) (lib.TransactionI, lib.ErrorI) {
	amount, err := f.getRandomStakeAmount(account, fee)
	if err != nil {
		return nil, err
	}
	return newTransactionBadSignature(from.PrivateKey, &types.MessageStake{
		PublicKey:     from.PublicKey.Bytes(),
		Amount:        amount,
		NetAddress:    f.getRandomNetAddr(),
		OutputAddress: f.getRandomOutputAddr(from).Bytes(),
	}, f.getTxTime(), fee)
}

func (f *Fuzzer) invalidEditStakeSignature(from *crypto.KeyGroup, val types.Validator, account types.Account, fee uint64) (tx lib.TransactionI, err lib.ErrorI) {
	return newTransactionBadSignature(from.PrivateKey, &types.MessageEditStake{
		Address:       from.Address.Bytes(),
		Amount:        val.StakedAmount,
		NetAddress:    val.NetAddress,
		OutputAddress: val.Output,
	}, f.getTxTime(), fee)
}

func (f *Fuzzer) invalidUnstakeSignature(from *crypto.KeyGroup, account types.Account, _ types.Validator, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return newTransactionBadSignature(from.PrivateKey, &types.MessageUnstake{
		Address: from.Address.Bytes(),
	}, f.getTxTime(), fee)
}

func (f *Fuzzer) invalidStakeTime(from *crypto.KeyGroup, account types.Account, fee uint64) (lib.TransactionI, lib.ErrorI) {
	amount, err := f.getRandomStakeAmount(account, fee)
	if err != nil {
		return nil, err
	}
	return types.NewStakeTx(from.PrivateKey, f.getRandomOutputAddr(from), f.getRandomNetAddr(), amount, f.getInvalidTxTime(), f.getBadFee(fee))
}

func (f *Fuzzer) invalidEditStakeTime(from *crypto.KeyGroup, val types.Validator, account types.Account, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return types.NewEditStakeTx(from.PrivateKey, crypto.NewAddress(val.Output), val.NetAddress, val.StakedAmount, f.getInvalidTxTime(), f.getBadFee(fee))
}

func (f *Fuzzer) invalidUnstakeTime(from *crypto.KeyGroup, account types.Account, _ types.Validator, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return types.NewUnstakeTx(from.PrivateKey, f.getInvalidTxTime(), f.getBadFee(fee))
}

func (f *Fuzzer) invalidStakeFee(from *crypto.KeyGroup, account types.Account, fee uint64) (lib.TransactionI, lib.ErrorI) {
	amount, err := f.getRandomStakeAmount(account, fee)
	if err != nil {
		return nil, err
	}
	return types.NewStakeTx(from.PrivateKey, f.getRandomOutputAddr(from), f.getRandomNetAddr(), amount, f.getTxTime(), fee)
}

func (f *Fuzzer) invalidEditStakeFee(from *crypto.KeyGroup, val types.Validator, account types.Account, fee uint64) (lib.TransactionI, lib.ErrorI) {
	output := crypto.Address(val.Output)
	return types.NewEditStakeTx(from.PrivateKey, &output, val.NetAddress, val.StakedAmount, f.getTxTime(), f.getBadFee(fee))
}

func (f *Fuzzer) invalidUnstakeFee(from *crypto.KeyGroup, account types.Account, _ types.Validator, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return types.NewUnstakeTx(from.PrivateKey, f.getTxTime(), f.getBadFee(fee))
}

func (f *Fuzzer) invalidStakeMsg(from *crypto.KeyGroup, account types.Account, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return f.getTxBadMessage(from, types.MessageStakeName, account, fee)
}

func (f *Fuzzer) invalidEditStakeMsg(from *crypto.KeyGroup, _ types.Validator, account types.Account, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return f.getTxBadMessage(from, types.MessageEditStakeName, account, fee)
}

func (f *Fuzzer) invalidUnstakeMsg(from *crypto.KeyGroup, account types.Account, _ types.Validator, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return f.getTxBadMessage(from, types.MessageUnstakeName, account, fee)
}

func (f *Fuzzer) invalidStakeAddress(from *crypto.KeyGroup, account types.Account, fee uint64) (lib.TransactionI, lib.ErrorI) {
	amount, err := f.getRandomStakeAmount(account, fee)
	if err != nil {
		return nil, err
	}
	return types.NewTransaction(from.PrivateKey, &types.MessageEditStake{
		Address:       f.getBadAddress(from).Bytes(),
		Amount:        amount,
		NetAddress:    f.getRandomNetAddr(),
		OutputAddress: f.getRandomOutputAddr(from).Bytes(),
	}, f.getTxTime(), fee)
}

func (f *Fuzzer) invalidEditStakeAddress(from *crypto.KeyGroup, val types.Validator, account types.Account, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return types.NewTransaction(from.PrivateKey, &types.MessageEditStake{
		Address:       f.getBadAddress(from).Bytes(),
		Amount:        val.StakedAmount,
		NetAddress:    val.NetAddress,
		OutputAddress: val.Output,
	}, f.getTxTime(), fee)
}

func (f *Fuzzer) invalidUnstakeSender(from *crypto.KeyGroup, account types.Account, _ types.Validator, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return types.NewTransaction(from.PrivateKey, &types.MessageUnstake{
		Address: f.getBadAddress(from).Bytes(),
	}, f.getTxTime(), fee)
}

func (f *Fuzzer) invalidStakeNetAddress(from *crypto.KeyGroup, account types.Account, fee uint64) (lib.TransactionI, lib.ErrorI) {
	amount, err := f.getRandomStakeAmount(account, fee)
	if err != nil {
		return nil, err
	}
	return types.NewStakeTx(from.PrivateKey, f.getRandomOutputAddr(from), f.getBadNetAddr(), amount, f.getTxTime(), fee)
}

func (f *Fuzzer) invalidEditStakeNetAddress(from *crypto.KeyGroup, val types.Validator, account types.Account, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return types.NewEditStakeTx(from.PrivateKey, crypto.NewAddress(val.Output), f.getBadNetAddr(), val.StakedAmount, f.getTxTime(), fee)
}

func (f *Fuzzer) invalidStakeOutput(from *crypto.KeyGroup, account types.Account, fee uint64) (lib.TransactionI, lib.ErrorI) {
	amount, err := f.getRandomStakeAmount(account, fee)
	if err != nil {
		return nil, err
	}
	return types.NewStakeTx(from.PrivateKey, f.getBadOutputAddress(from), f.getRandomNetAddr(), amount, f.getTxTime(), fee)
}

func (f *Fuzzer) invalidEditStakeOutput(from *crypto.KeyGroup, val types.Validator, account types.Account, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return types.NewEditStakeTx(from.PrivateKey, f.getBadOutputAddress(from), f.getRandomNetAddr(), val.StakedAmount, f.getTxTime(), fee)
}

func (f *Fuzzer) invalidStakeAmount(from *crypto.KeyGroup, account types.Account, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return types.NewStakeTx(from.PrivateKey, f.getRandomOutputAddr(from), f.getRandomNetAddr(), f.getBadStakeAmount(), f.getTxTime(), fee)
}

func (f *Fuzzer) invalidEditStakeAmount(from *crypto.KeyGroup, val types.Validator, account types.Account, fee uint64) (lib.TransactionI, lib.ErrorI) {
	return types.NewEditStakeTx(from.PrivateKey, crypto.NewAddress(val.Output), f.getRandomNetAddr(), val.StakedAmount, f.getTxTime(), fee)
}
