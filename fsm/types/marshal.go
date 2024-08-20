package types

import (
	"encoding/json"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"time"
)

const (
	PoolPageName           = "pools"
	AccountsPageName       = "accounts"
	ValidatorsPageName     = "validators"
	ConsValidatorsPageName = "consensus_validators"
)

func init() {
	lib.RegisteredPageables[PoolPageName] = new(PoolPage)
	lib.RegisteredPageables[AccountsPageName] = new(AccountPage)
	lib.RegisteredPageables[ValidatorsPageName] = new(ValidatorPage)
	lib.RegisteredPageables[ConsValidatorsPageName] = new(ConsValidatorPage)
}

type PoolPage []*Pool

func (p *PoolPage) Len() int          { return len(*p) }
func (p *PoolPage) New() lib.Pageable { return &PoolPage{} }

type AccountPage []*Account

func (p *AccountPage) Len() int          { return len(*p) }
func (p *AccountPage) New() lib.Pageable { return &AccountPage{} }

type ValidatorPage []*Validator

func (p *ValidatorPage) Len() int          { return len(*p) }
func (p *ValidatorPage) New() lib.Pageable { return &ValidatorPage{{}} }

type ConsValidatorPage []*lib.ConsensusValidator

func (p *ConsValidatorPage) Len() int          { return len(*p) }
func (p *ConsValidatorPage) New() lib.Pageable { return &ConsValidatorPage{{}} }

type NonSigners []*NonSigner

type genesisState struct {
	Time           string                   `json:"time,omitempty"`
	Pools          []*Pool                  `json:"pools,omitempty"`
	Accounts       []*Account               `protobuf:"bytes,3,rep,name=accounts,proto3" json:"accounts,omitempty"`
	ConsValidators *lib.ConsensusValidators `json:"consValidators"`
	Validators     []*Validator             `protobuf:"bytes,4,rep,name=validators,proto3" json:"validators,omitempty"`
	Params         *Params                  `protobuf:"bytes,5,opt,name=params,proto3" json:"params,omitempty"`
	Supply         *Supply                  `json:"supply"`
}

func (x *Account) ToString() string {
	bz, _ := lib.MarshalJSONIndent(x)
	return string(bz)
}

func (x *Validator) ToString() string {
	bz, _ := lib.MarshalJSONIndent(x)
	return string(bz)
}

func (x *Pool) ToString() string {
	bz, _ := lib.MarshalJSONIndent(x)
	return string(bz)
}

func (x *FeeParams) ToString() string {
	bz, _ := lib.MarshalJSONIndent(x)
	return string(bz)
}

func (x *ConsensusParams) ToString() string {
	bz, _ := lib.MarshalJSONIndent(x)
	return string(bz)
}

func (x *GovernanceParams) ToString() string {
	bz, _ := lib.MarshalJSONIndent(x)
	return string(bz)
}

func (x *ValidatorParams) ToString() string {
	bz, _ := lib.MarshalJSONIndent(x)
	return string(bz)
}

func (x *GenesisState) MarshalJSON() ([]byte, error) {
	return json.Marshal(genesisState{
		Time:           time.UnixMicro(int64(x.Time)).Format(time.DateTime),
		Pools:          x.Pools,
		Accounts:       x.Accounts,
		ConsValidators: x.ConsValidators,
		Validators:     x.Validators,
		Params:         x.Params,
		Supply:         x.Supply,
	})
}

func (x *GenesisState) UnmarshalJSON(bz []byte) (err error) {
	ptr := new(genesisState)
	if err = json.Unmarshal(bz, ptr); err != nil {
		return
	}
	t, e := time.Parse(time.DateTime, ptr.Time)
	if e != nil {
		return e
	}
	x.Time = uint64(t.UnixMicro())
	x.Params, x.Pools, x.Supply = ptr.Params, ptr.Pools, ptr.Supply
	x.Accounts, x.Validators, x.ConsValidators = ptr.Accounts, ptr.Validators, ptr.ConsValidators
	return
}

type account struct {
	Address lib.HexBytes `json:"address,omitempty"`
	Amount  uint64       `json:"amount,omitempty"`
}

func (x *Account) MarshalJSON() ([]byte, error) {
	return json.Marshal(account{x.Address, x.Amount})
}

func (x *Account) UnmarshalJSON(bz []byte) (err error) {
	a := new(account)
	if err = json.Unmarshal(bz, a); err != nil {
		return err
	}
	x.Address, x.Amount = a.Address, a.Amount
	return
}

type pool struct {
	ID     uint64 `json:"id"`
	Amount uint64 `json:"amount"`
}

func (x *Pool) MarshalJSON() ([]byte, error) {
	return json.Marshal(pool{x.Id, x.Amount})
}

func (x *Pool) UnmarshalJSON(bz []byte) (err error) {
	a := new(pool)
	if err = json.Unmarshal(bz, a); err != nil {
		return err
	}
	x.Id, x.Amount = a.ID, a.Amount
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

func (x *Validator) PassesFilter(f lib.ValidatorFilters) (ok bool) {
	switch {
	case f.Unstaking == lib.Yes:
		if x.UnstakingHeight == 0 {
			return
		}
	case f.Unstaking == lib.No:
		if x.UnstakingHeight != 0 {
			return
		}
	}
	switch {
	case f.Paused == lib.Yes:
		if x.MaxPausedHeight == 0 {
			return
		}
	case f.Paused == lib.No:
		if x.MaxPausedHeight != 0 {
			return
		}
	}
	switch {
	case f.Delegate == lib.Yes:
		if !x.Delegate {
			return
		}
	case f.Delegate == lib.No:
		if x.Delegate {
			return
		}
	}
	return true
}
