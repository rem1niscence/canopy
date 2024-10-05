package types

import (
	"bytes"
	"encoding/json"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
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
func (p *PoolPage) Len() int          { return len(*p) }
func (p *PoolPage) New() lib.Pageable { return &PoolPage{} }

type AccountPage []*Account

// AccountPage satisfies the Page interface
func (p *AccountPage) Len() int          { return len(*p) }
func (p *AccountPage) New() lib.Pageable { return &AccountPage{} }

type ValidatorPage []*Validator

// ValidatorPage satisfies the Page interface
func (p *ValidatorPage) Len() int          { return len(*p) }
func (p *ValidatorPage) New() lib.Pageable { return &ValidatorPage{{}} }

type ConsValidatorPage []*lib.ConsensusValidator

// ConsValidatorPage satisfies the Page interface
func (p *ConsValidatorPage) Len() int          { return len(*p) }
func (p *ConsValidatorPage) New() lib.Pageable { return &ConsValidatorPage{{}} }

type NonSigners []*NonSigner

// genesisState is the json.Marshaller and json.Unmarshaler implementation for the GenesisState object
type genesisState struct {
	Time       string       `json:"time,omitempty"`
	Pools      []*Pool      `json:"pools,omitempty"`
	Accounts   []*Account   `protobuf:"bytes,3,rep,name=accounts,proto3" json:"accounts,omitempty"`
	NonSigners NonSigners   `json:"nonSigners"`
	Validators []*Validator `protobuf:"bytes,4,rep,name=validators,proto3" json:"validators,omitempty"`
	Params     *Params      `protobuf:"bytes,5,opt,name=params,proto3" json:"params,omitempty"`
	Supply     *Supply      `json:"supply"`
}

// MarshalJSON() is the json.Marshaller implementation for the GenesisState object
func (x *GenesisState) MarshalJSON() ([]byte, error) {
	return json.Marshal(genesisState{
		Time:       time.UnixMicro(int64(x.Time)).Format(time.DateTime),
		Pools:      x.Pools,
		Accounts:   x.Accounts,
		NonSigners: x.NonSigners,
		Validators: x.Validators,
		Params:     x.Params,
		Supply:     x.Supply,
	})
}

// UnmarshalJSON() is the json.Unmarshaler implementation for the GenesisState object
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
	x.Accounts, x.Validators, x.NonSigners = ptr.Accounts, ptr.Validators, ptr.NonSigners
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
	Address         *crypto.Address           `json:"address,omitempty"`
	PublicKey       *crypto.BLS12381PublicKey `json:"public_key,omitempty"`
	NetAddress      string                    `json:"net_address,omitempty"`
	StakedAmount    uint64                    `json:"staked_amount,omitempty"`
	MaxPausedHeight uint64                    `json:"max_paused_height,omitempty"`
	UnstakingHeight uint64                    `json:"unstaking_height,omitempty"`
	Output          *crypto.Address           `json:"output,omitempty"`
}

// MarshalJSON() is the json.Marshaller implementation for the Validator object
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

// UnmarshalJSON() is the json.Unmarshaler implementation for the Validator object
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

// PassesFilter() returns if the Validator object passes the filter (true) or is filtered out (false)
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

// Combine() merges the Reward Recipients' Payment Percents of the current Proposal with those of another Proposal
// such that the Payment Percentages may be equally weighted when performing reward distribution calculations
// NOTE: percents will exceed 100% over multiple samples, but are normalized using the NumberOfSamples field
func (x *CommitteeData) Combine(f *CommitteeData) lib.ErrorI {
	if f == nil {
		return nil
	}
	for _, ep := range f.PaymentPercents {
		x.addPercents(ep.Address, ep.Percent)
	}
	// new Proposal purposefully overwrites the Block and Meta of the current Proposal
	// this is to ensure both Proposals have the latest Block and Meta information
	// in the case where the caller uses a pattern where there may be a stale Block/Meta
	*x = CommitteeData{
		PaymentPercents: x.PaymentPercents,
		NumberOfSamples: x.NumberOfSamples + 1,
		CommitteeId:     f.CommitteeId,
		CommitteeHeight: f.CommitteeHeight,
		ChainHeight:     f.ChainHeight,
	}
	return nil
}

// addPercents() is a helper function that adds reward distribution percents on behalf of an address
func (x *CommitteeData) addPercents(address []byte, percent uint64) {
	// check to see if the address already has samples
	for i, ep := range x.PaymentPercents {
		if bytes.Equal(address, ep.Address) {
			x.PaymentPercents[i].Percent += ep.Percent
			return
		}
	}
	// if not, append a sample to PaymentPercents
	x.PaymentPercents = append(x.PaymentPercents, &lib.PaymentPercents{
		Address: address,
		Percent: percent,
	})
}

// AddOrder() adds a sell order to the OrderBook
func (x *OrderBook) AddOrder(order *SellOrder) (id uint64) {
	// if there's an empty slot, fill it with the sell order
	for i, slot := range x.Orders {
		if slot == nil {
			id = uint64(i)
			order.Id = id
			x.Orders[i] = order
			return
		}
	}
	// if there's no empty slots, add the sell order to the
	id = uint64(len(x.Orders))
	order.Id = id
	x.Orders = append(x.Orders, order)
	return
}

// BuyOrder() adds a recipient address and deadline height to the order to 'claim' the order and prevent others from 'claiming it'
func (x *OrderBook) BuyOrder(orderId int, buyerAddress []byte, buyerChainDeadlineHeight uint64) lib.ErrorI {
	order, err := x.GetOrder(orderId)
	if err != nil {
		return err
	}
	if order.BuyerReceiveAddress != nil {
		return ErrOrderAlreadyAccepted()
	}
	order.BuyerReceiveAddress, order.BuyerChainDeadline = buyerAddress, buyerChainDeadlineHeight
	x.Orders[orderId] = order
	return nil
}

// ResetOrder() removes a recipient address and the deadline height from the order to 'un-claim' the order
func (x *OrderBook) ResetOrder(orderId int) lib.ErrorI {
	order, err := x.GetOrder(orderId)
	if err != nil {
		return err
	}
	order.BuyerReceiveAddress, order.BuyerChainDeadline = nil, 0
	x.Orders[orderId] = order
	return nil
}

// UpdateOrder() updates a sell order to the OrderBook, passing a nil `order` is effectively a delete operation
func (x *OrderBook) UpdateOrder(orderId int, order *SellOrder) (err lib.ErrorI) {
	numOfOrderSlots := len(x.Orders)
	if orderId >= numOfOrderSlots {
		return ErrOrderNotFound(orderId)
	}
	// if deleting from the end, shrink the slice
	if order == nil && orderId == numOfOrderSlots-1 {
		x.Orders = x.Orders[:numOfOrderSlots-1]
		// continue shrinking the slice if nil entries are at the end
		for i := numOfOrderSlots - 2; i >= 0; i-- {
			if x.Orders[i] != nil {
				break
			}
			x.Orders = x.Orders[:i]
		}
		return
	}
	// if not deleting from the end of the slice,
	// simply replace the order
	x.Orders[orderId] = order
	return
}

// GetOrder() retrieves a sell order from the OrderBook
func (x *OrderBook) GetOrder(orderId int) (order *SellOrder, err lib.ErrorI) {
	numOfOrderSlots := len(x.Orders)
	if orderId >= numOfOrderSlots || x.Orders[orderId] == nil {
		return nil, ErrOrderNotFound(orderId)
	}
	order = x.Orders[orderId]
	return
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
