package state_machine

import (
	"github.com/ginchuco/ginchu/crypto"
	"github.com/ginchuco/ginchu/state_machine/types"
	lib "github.com/ginchuco/ginchu/types"
	"math/big"
)

func (s *StateMachine) GetAccount(address crypto.AddressI) (*types.Account, lib.ErrorI) {
	bz, err := s.Get(types.KeyForAccount(address))
	if err != nil {
		return nil, err
	}
	return s.unmarshalAccount(bz)
}

func (s *StateMachine) GetAccounts() ([]*types.Account, lib.ErrorI) {
	it, err := s.Iterator(types.AccountPrefix())
	if err != nil {
		return nil, err
	}
	defer it.Close()
	var result []*types.Account
	for ; it.Valid(); it.Next() {
		var acc *types.Account
		acc, err = s.unmarshalAccount(it.Value())
		if err != nil {
			return nil, err
		}
		result = append(result, acc)
	}
	return result, nil
}

func (s *StateMachine) GetAccountBalance(address crypto.AddressI) (*big.Int, lib.ErrorI) {
	account, err := s.GetAccount(address)
	if err != nil {
		return nil, err
	}
	return lib.StringToBigInt(account.Amount)
}

func (s *StateMachine) SetAccount(account *types.Account) lib.ErrorI {
	bz, err := s.marshalAccount(account)
	if err != nil {
		return err
	}
	address := crypto.NewAddressFromBytes(account.Address)
	if err = s.Set(types.KeyForAccount(address), bz); err != nil {
		return err
	}
	return nil
}

func (s *StateMachine) MintToAccount(address crypto.AddressI, amount *big.Int) lib.ErrorI {
	return s.AccountAdd(address, lib.BigIntToString(amount))
}

func (s *StateMachine) AccountAdd(address crypto.AddressI, amountToAdd string) lib.ErrorI {
	account, err := s.GetAccount(address)
	if err != nil {
		return err
	}
	account.Amount, err = lib.StringBigAdd(account.Amount, amountToAdd)
	if err != nil {
		return err
	}
	return s.SetAccount(account)
}

func (s *StateMachine) AccountSub(address crypto.AddressI, amountToSub string) lib.ErrorI {
	account, err := s.GetAccount(address)
	if err != nil {
		return err
	}
	account.Amount, err = lib.StringSub(account.Amount, amountToSub)
	if err != nil {
		return err
	}
	isLessThanZero, err := lib.StringBigLTE(account.Amount, big.NewInt(0))
	if err != nil {
		return err
	}
	if isLessThanZero {
		return types.ErrInsufficientFunds()
	}
	return s.SetAccount(account)
}

func (s *StateMachine) unmarshalAccount(bz []byte) (*types.Account, lib.ErrorI) {
	acc := new(types.Account)
	if err := types.Unmarshal(bz, acc); err != nil {
		return nil, err
	}
	return acc, nil
}

func (s *StateMachine) marshalAccount(account *types.Account) ([]byte, lib.ErrorI) {
	return types.Marshal(account)
}

// Pool logic below

func (s *StateMachine) GetPool(name string) (*types.Pool, lib.ErrorI) {
	bz, err := s.Get(types.KeyForPool([]byte(name)))
	if err != nil {
		return nil, err
	}
	return s.unmarshalPool(bz)
}

func (s *StateMachine) GetPoolBalance(name string) (*big.Int, lib.ErrorI) {
	pool, err := s.GetPool(name)
	if err != nil {
		return nil, err
	}
	return lib.StringToBigInt(pool.Amount)
}

func (s *StateMachine) SetPool(pool *types.Pool) lib.ErrorI {
	bz, err := s.marshalPool(pool)
	if err != nil {
		return err
	}
	if err = s.Set(types.KeyForPool(pool.Name), bz); err != nil {
		return err
	}
	return nil
}

func (s *StateMachine) MintToPool(name string, amount *big.Int) lib.ErrorI {
	return s.PoolAdd(name, lib.BigIntToString(amount))
}

func (s *StateMachine) PoolAdd(name, amountToAdd string) lib.ErrorI {
	pool, err := s.GetPool(name)
	if err != nil {
		return err
	}
	pool.Amount, err = lib.StringBigAdd(pool.Amount, amountToAdd)
	if err != nil {
		return err
	}
	return s.SetPool(pool)
}

func (s *StateMachine) PoolSub(name, amountToSub string) lib.ErrorI {
	pool, err := s.GetPool(name)
	if err != nil {
		return err
	}
	pool.Amount, err = lib.StringSub(pool.Amount, amountToSub)
	if err != nil {
		return err
	}
	isLessThanZero, err := lib.StringBigLTE(pool.Amount, big.NewInt(0))
	if err != nil {
		return err
	}
	if isLessThanZero {
		return types.ErrInsufficientFunds()
	}
	return s.SetPool(pool)
}

func (s *StateMachine) unmarshalPool(bz []byte) (*types.Pool, lib.ErrorI) {
	pool := new(types.Pool)
	if err := types.Unmarshal(bz, pool); err != nil {
		return nil, err
	}
	return pool, nil
}

func (s *StateMachine) marshalPool(pool *types.Pool) ([]byte, lib.ErrorI) {
	return types.Marshal(pool)
}
