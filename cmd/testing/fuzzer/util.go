// nolint:all
package main

import (
	"fmt"
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"github.com/tjarratt/babble"
	"google.golang.org/protobuf/proto"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"
)

type DependentState struct {
	sync.RWMutex
	height   uint64
	accounts map[string]*types.Account
	vals     map[string]*types.Validator
}

func (d *DependentState) Reset(height uint64) {
	d.height = height
	d.accounts = make(map[string]*types.Account)
	d.vals = make(map[string]*types.Validator)
}

func (d *DependentState) GetAccount(address crypto.AddressI) (*types.Account, bool) {
	d.RLock()
	defer d.RUnlock()
	account, ok := d.accounts[address.String()]
	return account, ok
}

func (d *DependentState) SetAccount(account types.Account) {
	d.Lock()
	defer d.Unlock()
	d.accounts[lib.BytesToString(account.Address)] = &account
}

func (d *DependentState) GetValidator(address crypto.AddressI) (*types.Validator, bool) {
	d.RLock()
	defer d.RUnlock()
	val, ok := d.vals[address.String()]
	return val, ok
}

func (d *DependentState) SetValidator(val types.Validator) {
	d.Lock()
	defer d.Unlock()
	d.vals[lib.BytesToString(val.Address)] = &val
}

func (f *Fuzzer) getAccount(address crypto.AddressI) (acc types.Account) {
	if cached, ok := f.state.GetAccount(address); ok {
		return *cached
	}
	account, err := f.client.Account(0, address.String())
	if err != nil {
		return types.Account{}
	}
	if account == nil {
		return types.Account{}
	}
	return *account
}

func (f *Fuzzer) getValidator(address crypto.AddressI) (val types.Validator) {
	if cached, ok := f.state.GetValidator(address); ok {
		return *cached
	}
	validator, err := f.client.Validator(0, address.String())
	if err != nil {
		return types.Validator{}
	}
	if validator == nil {
		return types.Validator{}
	}
	return *validator
}

func (f *Fuzzer) getFees() (p *types.FeeParams) {
	p, err := f.client.FeeParams(0)
	if err != nil {
		f.log.Error(err.Error())
		time.Sleep(1 * time.Second)
		f.getFees()
	}
	return
}

func (f *Fuzzer) getValParams() (p *types.ValidatorParams) {
	p, err := f.client.ValParams(0)
	if err != nil {
		f.log.Error(err.Error())
		time.Sleep(1 * time.Second)
		f.getValParams()
	}
	return
}

func (f *Fuzzer) getRandomAmountUpTo(limit uint64) uint64 {
	return uint64(rand.Intn(int(limit) + 1))
}

func (f *Fuzzer) getRandomStakeAmount(acc types.Account, fee uint64) (uint64, lib.ErrorI) {
	minStake := uint64(1)
	if acc.Amount < minStake {
		return 0, types.ErrInsufficientFunds()
	}
	return uint64(rand.Intn(int(acc.Amount-fee-minStake) + int(minStake))), nil
}

func (f *Fuzzer) getBadStakeAmount() uint64 {
	return 0
}

func (f *Fuzzer) getRandomKeyGroup() *crypto.KeyGroup {
	return crypto.NewKeyGroup(f.config.PrivateKeys[rand.Intn(len(f.config.PrivateKeys))])
}

func (f *Fuzzer) getRandomNetAddr() string {
	return fmt.Sprintf("http://%s.com", babble.NewBabbler().Babble())
}

func (f *Fuzzer) getBadNetAddr() string {
	switch rand.Intn(2) {
	case 0:
		return strings.Repeat("a", 51)
	default:
		return ""
	}
}

func (f *Fuzzer) getRandomOutputAddr(from *crypto.KeyGroup) crypto.AddressI {
	output := from.Address
	switch rand.Intn(2) {
	case 0:
		pub, _ := crypto.NewED25519PublicKey()
		output = pub.Address()
	}
	return output
}

func (f *Fuzzer) getBadFee(fee uint64) uint64 {
	switch rand.Intn(3) {
	case 0:
		fee -= 1
	case 1:
		fee = math.MaxUint64
	case 2:
		fee = 0
	}
	return fee
}

func (f *Fuzzer) getTxBadMessage(from *crypto.KeyGroup, msgType string, account types.Account, fee uint64) (tx lib.TransactionI, err lib.ErrorI) {
	var msg proto.Message
	switch rand.Intn(2) {
	case 0:
		msg = nil
	case 1:
		msg = &lib.UInt64Wrapper{}
	}
	a, err := lib.NewAny(msg)
	if err != nil {
		return nil, err
	}
	tx = &lib.Transaction{
		Type: msgType,
		Msg:  a,
		Time: f.getTxTime(),
		Fee:  fee,
	}
	err = tx.(*lib.Transaction).Sign(from.PrivateKey)
	return
}

func (f *Fuzzer) getBadAddress(from *crypto.KeyGroup) crypto.AddressI {
	switch rand.Intn(3) {
	case 0:
		pk, _ := crypto.NewEd25519PrivateKey()
		return pk.PublicKey().Address()
	case 1:
		return nil
	case 2:
		return crypto.NewAddress(from.PublicKey.Bytes())
	}
	return nil
}

func (f *Fuzzer) getBadOutputAddress(from *crypto.KeyGroup) crypto.AddressI {
	switch rand.Intn(2) {
	case 0:
		pk, _ := crypto.NewEd25519PrivateKey()
		return pk.PublicKey().Address()
	case 1:
		return crypto.NewAddress(from.PublicKey.Bytes())
	}
	return nil
}

func newTransactionBadSignature(pk crypto.PrivateKeyI, msg proto.Message, time, fee uint64) (lib.TransactionI, lib.ErrorI) {
	a, err := lib.NewAny(msg)
	if err != nil {
		return nil, err
	}
	tx := &lib.Transaction{
		Msg:  a,
		Time: time,
		Fee:  fee,
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
