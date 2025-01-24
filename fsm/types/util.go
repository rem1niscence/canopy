package types

import (
	"encoding/json"
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
	"slices"
	"time"
)

const (
	PoolPageName           = "pools"                // name for page of 'Pools'
	AccountsPageName       = "accounts"             // name for page of 'Accounts'
	ValidatorsPageName     = "validators"           // name for page of 'Validators'
	ConsValidatorsPageName = "consensus_validators" // name for page of 'Consensus Validators' (only essential val info needed for consensus)
)

func init() {
	// Register the pages for converting bytes of Page into the correct Page object
	lib.RegisteredPageables[PoolPageName] = new(PoolPage)
	lib.RegisteredPageables[AccountsPageName] = new(AccountPage)
	lib.RegisteredPageables[ValidatorsPageName] = new(ValidatorPage)
	lib.RegisteredPageables[ConsValidatorsPageName] = new(ConsValidatorPage)
}

type PoolPage []*Pool

// PoolPage satisfies the Page interface
func (p *PoolPage) New() lib.Pageable { return &PoolPage{} }

type AccountPage []*Account

// AccountPage satisfies the Page interface
func (p *AccountPage) New() lib.Pageable { return &AccountPage{} }

type ValidatorPage []*Validator

// ValidatorPage satisfies the Page interface
func (p *ValidatorPage) New() lib.Pageable { return &ValidatorPage{{}} }

type ConsValidatorPage []*lib.ConsensusValidator

// ConsValidatorPage satisfies the Page interface
func (p *ConsValidatorPage) New() lib.Pageable { return &ConsValidatorPage{{}} }

type NonSigners []*NonSigner

// genesisState is the json.Marshaller and json.Unmarshaler implementation for the GenesisState object
type genesisState struct {
	Time          string              `json:"time,omitempty"`
	Pools         []*Pool             `json:"pools,omitempty"`
	Accounts      []*Account          `protobuf:"bytes,3,rep,name=accounts,proto3" json:"accounts,omitempty"`
	NonSigners    NonSigners          `json:"nonSigners"`
	Validators    []*Validator        `protobuf:"bytes,4,rep,name=validators,proto3" json:"validators,omitempty"`
	Params        *Params             `protobuf:"bytes,5,opt,name=params,proto3" json:"params,omitempty"`
	Supply        *Supply             `json:"supply"`
	OrderBooks    *lib.OrderBooks     `protobuf:"bytes,7,opt,name=order_books,json=orderBooks,proto3" json:"order_books,omitempty"`
	DoubleSigners []*lib.DoubleSigner `protobuf:"bytes,6,rep,name=double_signers,json=doubleSigners,proto3" json:"double_signers,omitempty"` // only used for export
}

// MarshalJSON() is the json.Marshaller implementation for the GenesisState object
func (x *GenesisState) MarshalJSON() ([]byte, error) {
	var t string
	if x.Time != 0 {
		t = time.UnixMicro(int64(x.Time)).Format(time.DateTime)
	}
	return json.Marshal(genesisState{
		Time:          t,
		Pools:         x.Pools,
		Accounts:      x.Accounts,
		NonSigners:    x.NonSigners,
		Validators:    x.Validators,
		Params:        x.Params,
		Supply:        x.Supply,
		OrderBooks:    x.OrderBooks,
		DoubleSigners: x.DoubleSigners,
	})
}

// UnmarshalJSON() is the json.Unmarshaler implementation for the GenesisState object
func (x *GenesisState) UnmarshalJSON(bz []byte) (err error) {
	ptr := new(genesisState)
	if err = json.Unmarshal(bz, ptr); err != nil {
		return
	}
	if ptr.Time != "" {
		t, e := time.Parse(time.DateTime, ptr.Time)
		if e != nil {
			return e
		}
		x.Time = uint64(t.UnixMicro())
	}
	x.Params, x.Pools, x.Supply = ptr.Params, ptr.Pools, ptr.Supply
	x.Accounts, x.Validators, x.NonSigners = ptr.Accounts, ptr.Validators, ptr.NonSigners
	x.OrderBooks, x.DoubleSigners = ptr.OrderBooks, ptr.DoubleSigners
	return
}

// account is the json.Marshaller and json.Unmarshaler implementation for the Account object
type account struct {
	Address lib.HexBytes `json:"address,omitempty"`
	Amount  uint64       `json:"amount,omitempty"`
}

// MarshalJSON() is the json.Marshaller implementation for the Account object
func (x *Account) MarshalJSON() ([]byte, error) {
	return json.Marshal(account{x.Address, x.Amount})
}

// UnmarshalJSON() is the json.Unmarshaler implementation for the Account object
func (x *Account) UnmarshalJSON(bz []byte) (err error) {
	a := new(account)
	if err = json.Unmarshal(bz, a); err != nil {
		return err
	}
	x.Address, x.Amount = a.Address, a.Amount
	return
}

// pool is the json.Marshaller and json.Unmarshaler implementation for the Pool object
type pool struct {
	ID     uint64 `json:"id"`
	Amount uint64 `json:"amount"`
}

// MarshalJSON() is the json.Marshaller implementation for the Pool object
func (x *Pool) MarshalJSON() ([]byte, error) {
	return json.Marshal(pool{x.Id, x.Amount})
}

// UnmarshalJSON() is the json.Unmarshaler implementation for the Pool object
func (x *Pool) UnmarshalJSON(bz []byte) (err error) {
	a := new(pool)
	if err = json.Unmarshal(bz, a); err != nil {
		return err
	}
	x.Id, x.Amount = a.ID, a.Amount
	return
}

// validator is the json.Marshaller and json.Unmarshaler implementation for the Validator object
type validator struct {
	Address         *crypto.Address           `json:"address"`
	PublicKey       *crypto.BLS12381PublicKey `json:"public_key"`
	Committees      []uint64                  `json:"committees"`
	NetAddress      string                    `json:"net_address"`
	StakedAmount    uint64                    `json:"staked_amount"`
	MaxPausedHeight uint64                    `json:"max_paused_height"`
	UnstakingHeight uint64                    `json:"unstaking_height"`
	Output          *crypto.Address           `json:"output"`
	Delegate        bool                      `json:"delegate"`
	Compound        bool                      `json:"compound"`
}

// MarshalJSON() is the json.Marshaller implementation for the Validator object
func (x *Validator) MarshalJSON() ([]byte, error) {
	publicKey, err := crypto.BytesToBLS12381Public(x.PublicKey)
	if err != nil {
		return nil, err
	}
	return json.Marshal(validator{
		Address:         crypto.NewAddressFromBytes(x.Address).(*crypto.Address),
		PublicKey:       publicKey.(*crypto.BLS12381PublicKey),
		Committees:      x.Committees,
		NetAddress:      x.NetAddress,
		StakedAmount:    x.StakedAmount,
		MaxPausedHeight: x.MaxPausedHeight,
		UnstakingHeight: x.UnstakingHeight,
		Output:          crypto.NewAddressFromBytes(x.Output).(*crypto.Address),
		Delegate:        x.Delegate,
		Compound:        x.Compound,
	})
}

// UnmarshalJSON() is the json.Unmarshaler implementation for the Validator object
func (x *Validator) UnmarshalJSON(bz []byte) error {
	val := new(validator)
	if err := json.Unmarshal(bz, val); err != nil {
		return err
	}
	*x = Validator{
		Address:         val.Address.Bytes(),
		PublicKey:       val.PublicKey.Bytes(),
		NetAddress:      val.NetAddress,
		StakedAmount:    val.StakedAmount,
		Committees:      val.Committees,
		MaxPausedHeight: val.MaxPausedHeight,
		UnstakingHeight: val.UnstakingHeight,
		Output:          val.Output.Bytes(),
		Delegate:        val.Delegate,
		Compound:        val.Compound,
	}
	return nil
}

// PassesFilter() returns if the Validator object passes the filter (true) or is filtered out (false)
func (x *Validator) PassesFilter(f lib.ValidatorFilters) (ok bool) {
	switch {
	case f.Unstaking == lib.FilterOption_MustBe:
		if x.UnstakingHeight == 0 {
			return
		}
	case f.Unstaking == lib.FilterOption_Exclude:
		if x.UnstakingHeight != 0 {
			return
		}
	}
	switch {
	case f.Paused == lib.FilterOption_MustBe:
		if x.MaxPausedHeight == 0 {
			return
		}
	case f.Paused == lib.FilterOption_Exclude:
		if x.MaxPausedHeight != 0 {
			return
		}
	}
	switch {
	case f.Delegate == lib.FilterOption_MustBe:
		if !x.Delegate {
			return
		}
	case f.Delegate == lib.FilterOption_Exclude:
		if x.Delegate {
			return
		}
	}
	if f.Committee != 0 {
		if !slices.Contains(x.Committees, f.Committee) {
			return
		}
	}
	return true
}

// SlashTracker is a map of address -> committee -> slash percentage
// which is used to ensure no committee exceeds max slash within a single block
// NOTE: this slash tracker is naive and doesn't account for the consecutive reduction
// of a slash percentage impact i.e. two 10% slashes = 20%, but technically it's 19%
type SlashTracker map[string]map[uint64]uint64

func NewSlashTracker() *SlashTracker {
	slashTracker := make(SlashTracker)
	return &slashTracker
}

// AddSlash() adds a slash for an address at by a committee for a certain percent
func (s *SlashTracker) AddSlash(address []byte, committeeId, percent uint64) {
	// add the percent to the total
	(*s)[s.toKey(address)][committeeId] += percent
}

// GetTotalSlashPercent() returns the total percent for a slash
func (s *SlashTracker) GetTotalSlashPercent(address []byte, committeeId uint64) (percent uint64) {
	// return the total percent
	return (*s)[s.toKey(address)][committeeId]
}

// toKey() converts the address bytes to a string and ensures the map is initialized for that address
func (s *SlashTracker) toKey(address []byte) string {
	// convert the address to a string
	addr := lib.BytesToString(address)
	// if the address has not yet been slashed by any committee
	// create the corresponding committee map
	if _, ok := (*s)[addr]; !ok {
		(*s)[addr] = make(map[uint64]uint64)
	}
	return addr
}

type SupplyPoolType int

const (
	CommitteesWithDelegations SupplyPoolType = 0
	DelegationsOnly           SupplyPoolType = 1
)
