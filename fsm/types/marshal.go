package types

import (
	"encoding/json"
	"github.com/ginchuco/ginchu/lib/crypto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"time"
)

type genesisState struct {
	Time       string       `json:"time,omitempty"`
	Pools      []*Pool      `json:"pools,omitempty"`
	Accounts   []*Account   `protobuf:"bytes,3,rep,name=accounts,proto3" json:"accounts,omitempty"`
	Validators []*Validator `protobuf:"bytes,4,rep,name=validators,proto3" json:"validators,omitempty"`
	Params     *Params      `protobuf:"bytes,5,opt,name=params,proto3" json:"params,omitempty"`
}

func (x *GenesisState) MarshalJSON() ([]byte, error) {
	t := x.Time.AsTime()
	return json.Marshal(genesisState{
		Time:       t.Format(time.DateTime),
		Pools:      x.Pools,
		Accounts:   x.Accounts,
		Validators: x.Validators,
		Params:     x.Params,
	})
}

func (x *GenesisState) UnmarshalJSON(bz []byte) (err error) {
	ptr := new(genesisState)
	if err = json.Unmarshal(bz, ptr); err != nil {
		return
	}
	t, err := time.Parse(time.DateTime, ptr.Time)
	if err != nil {
		return
	}
	x.Time, x.Params = timestamppb.New(t), ptr.Params
	x.Pools, x.Accounts, x.Validators = ptr.Pools, ptr.Accounts, ptr.Validators
	return
}

type account struct {
	Address  *crypto.Address `json:"address,omitempty"`
	Amount   uint64          `json:"amount,omitempty"`
	Sequence uint64          `json:"sequence,omitempty"`
}

func (x *Account) MarshalJSON() ([]byte, error) {
	return json.Marshal(account{crypto.NewAddressFromBytes(x.Address).(*crypto.Address), x.Amount, x.Sequence})
}

func (x *Account) UnmarshalJSON(bz []byte) (err error) {
	a := new(account)
	if err = json.Unmarshal(bz, a); err != nil {
		return err
	}
	x.Address, x.Amount, x.Sequence = a.Address.Bytes(), a.Amount, a.Sequence
	return
}

type pool struct {
	Name   string `json:"name"`
	Amount uint64 `json:"amount"`
}

func (x *Pool) MarshalJSON() ([]byte, error) {
	name := PoolID_name[int32(x.Id)]
	return json.Marshal(pool{name, x.Amount})
}

func (x *Pool) UnmarshalJSON(bz []byte) (err error) {
	a := new(pool)
	if err = json.Unmarshal(bz, a); err != nil {
		return err
	}
	x.Id, x.Amount = PoolID(PoolID_value[a.Name]), a.Amount
	return
}

type validator struct {
	Address         *crypto.Address           `json:"address,omitempty"`
	PublicKey       *crypto.BLS12381PublicKey `json:"public_key,omitempty"`
	NetAddress      string                    `json:"net_address,omitempty"`
	StakedAmount    uint64                    `json:"staked_amount,omitempty"`
	MaxPausedHeight uint64                    `json:"max_paused_height,omitempty"`
	UnstakingHeight uint64                    `json:"unstaking_height,omitempty"`
	Output          *crypto.Address           `json:"output,omitempty"`
}

func (x *Validator) MarshalJSON() ([]byte, error) {
	publicKey, err := crypto.NewBLSPublicKeyFromBytes(x.PublicKey)
	if err != nil {
		return nil, err
	}
	return json.Marshal(validator{
		Address:         crypto.NewAddressFromBytes(x.Address).(*crypto.Address),
		PublicKey:       publicKey.(*crypto.BLS12381PublicKey),
		NetAddress:      x.NetAddress,
		StakedAmount:    x.StakedAmount,
		MaxPausedHeight: x.MaxPausedHeight,
		UnstakingHeight: x.UnstakingHeight,
		Output:          crypto.NewAddressFromBytes(x.Output).(*crypto.Address),
	})
}

func (x *Validator) UnmarshalJSON(bz []byte) error {
	val := new(validator)
	if err := json.Unmarshal(bz, val); err != nil {
		return err
	}
	x.Address, x.PublicKey = val.Address.Bytes(), val.PublicKey.Bytes()
	x.StakedAmount, x.NetAddress, x.Output = val.StakedAmount, val.NetAddress, val.Output.Bytes()
	x.MaxPausedHeight, x.UnstakingHeight = val.MaxPausedHeight, val.UnstakingHeight
	return nil
}
