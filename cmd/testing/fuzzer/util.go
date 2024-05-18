package main

import (
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"google.golang.org/protobuf/proto"
	"math/rand"
	"sync"
)

type DependentState struct {
	sync.RWMutex
	height   uint64
	accounts map[string]*types.Account
}

func (d *DependentState) Reset(height uint64) {
	d.height = height
	d.accounts = make(map[string]*types.Account)
}

func (d *DependentState) GetAccount(address crypto.AddressI) (*types.Account, bool) {
	d.RLock()
	defer d.RUnlock()
	account, ok := d.accounts[address.String()]
	return account, ok
}

func (d *DependentState) SetAccount(account *types.Account) {
	d.Lock()
	defer d.Unlock()
	d.accounts[lib.BytesToString(account.Address)] = account
}

func (f *Fuzzer) getRandomAmountUpTo(limit uint64) uint64 {
	return uint64(rand.Intn(int(limit) + 1))
}

func (f *Fuzzer) getRandomKeyGroup() *crypto.KeyGroup {
	return crypto.NewKeyGroup(&f.config.PrivateKeys[rand.Intn(len(f.config.PrivateKeys))])
}

func newTransactionBadSignature(pk crypto.PrivateKeyI, msg proto.Message, seq, fee uint64) (lib.TransactionI, lib.ErrorI) {
	a, err := lib.ToAny(msg)
	if err != nil {
		return nil, err
	}
	tx := &lib.Transaction{
		Msg:      a,
		Sequence: seq,
		Fee:      fee,
	}
	bz, err := tx.GetSignBytes()
	if err != nil {
		return nil, err
	}
	switch rand.Intn(4) {
	case 0:
		tx.Signature = &lib.Signature{
			PublicKey: pk.PublicKey().Bytes(),
			Signature: pk.Sign([]byte("foo")),
		}
	case 1:
		tx.Signature = &lib.Signature{
			Signature: pk.Sign(bz),
		}
	case 2:
		tx.Signature = &lib.Signature{
			PublicKey: pk.PublicKey().Bytes(),
			Signature: nil,
		}
	case 3:
		tx.Signature = &lib.Signature{
			PublicKey: pk.PublicKey().Bytes(),
			Signature: []byte("foo"),
		}
	}
	return tx, tx.Sign(pk)
}
