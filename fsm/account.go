package fsm

import (
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
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

func (s *StateMachine) GetAccountBalance(address crypto.AddressI) (uint64, lib.ErrorI) {
	account, err := s.GetAccount(address)
	if err != nil {
		return 0, err
	}
	return account.Amount, nil
}

func (s *StateMachine) GetAccountSequence(address crypto.AddressI) (uint64, lib.ErrorI) {
	account, err := s.GetAccount(address)
	if err != nil {
		return 0, err
	}
	return account.Sequence, nil
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

func (s *StateMachine) SetAccounts(accounts []*types.Account) lib.ErrorI {
	for _, acc := range accounts {
		if err := s.SetAccount(acc); err != nil {
			return err
		}
	}
	return nil
}

func (s *StateMachine) AccountDeductFees(address crypto.AddressI, fee uint64) lib.ErrorI {
	if err := s.AccountSub(address, fee); err != nil {
		return err
	}
	return s.PoolAdd(types.PoolName_FeeCollector, fee)
}

func (s *StateMachine) MintToAccount(address crypto.AddressI, amount uint64) lib.ErrorI {
	return s.AccountAdd(address, amount)
}

func (s *StateMachine) AccountSetSequence(address crypto.AddressI, sequence uint64) lib.ErrorI {
	acc, err := s.GetAccount(address)
	if err != nil {
		return err
	}
	if acc.Sequence >= sequence {
		return types.ErrInvalidTxSequence()
	}
	acc.Sequence = sequence
	return s.SetAccount(acc)
}

func (s *StateMachine) AccountAdd(address crypto.AddressI, amountToAdd uint64) lib.ErrorI {
	account, err := s.GetAccount(address)
	if err != nil {
		return err
	}
	account.Amount += amountToAdd
	return s.SetAccount(account)
}

func (s *StateMachine) AccountSub(address crypto.AddressI, amountToSub uint64) lib.ErrorI {
	account, err := s.GetAccount(address)
	if err != nil {
		return err
	}
	if account.Amount < amountToSub {
		return types.ErrInsufficientFunds()
	}
	account.Amount -= amountToSub
	return s.SetAccount(account)
}

func (s *StateMachine) unmarshalAccount(bz []byte) (*types.Account, lib.ErrorI) {
	acc := new(types.Account)
	if err := lib.Unmarshal(bz, acc); err != nil {
		return nil, err
	}
	return acc, nil
}

func (s *StateMachine) marshalAccount(account *types.Account) ([]byte, lib.ErrorI) {
	return lib.Marshal(account)
}

// Pool logic below

func (s *StateMachine) GetPool(name types.PoolName) (*types.Pool, lib.ErrorI) {
	bz, err := s.Get(types.KeyForPool(name))
	if err != nil {
		return nil, err
	}
	return s.unmarshalPool(bz)
}

func (s *StateMachine) GetPools() ([]*types.Pool, lib.ErrorI) {
	it, err := s.Iterator(types.PoolPrefix())
	if err != nil {
		return nil, err
	}
	defer it.Close()
	var result []*types.Pool
	for ; it.Valid(); it.Next() {
		var acc *types.Pool
		acc, err = s.unmarshalPool(it.Value())
		if err != nil {
			return nil, err
		}
		result = append(result, acc)
	}
	return result, nil
}

func (s *StateMachine) GetPoolBalance(name types.PoolName) (uint64, lib.ErrorI) {
	pool, err := s.GetPool(name)
	if err != nil {
		return 0, err
	}
	return pool.Amount, nil
}

func (s *StateMachine) SetPools(pools []*types.Pool) lib.ErrorI {
	for _, pool := range pools {
		if err := s.SetPool(pool); err != nil {
			return err
		}
	}
	return nil
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

func (s *StateMachine) MintToPool(name types.PoolName, amount uint64) lib.ErrorI {
	return s.PoolAdd(name, amount)
}

func (s *StateMachine) PoolAdd(name types.PoolName, amountToAdd uint64) lib.ErrorI {
	pool, err := s.GetPool(name)
	if err != nil {
		return err
	}
	pool.Amount += amountToAdd
	return s.SetPool(pool)
}

func (s *StateMachine) PoolSub(name types.PoolName, amountToSub uint64) lib.ErrorI {
	pool, err := s.GetPool(name)
	if err != nil {
		return err
	}
	if pool.Amount < amountToSub {
		return types.ErrInsufficientFunds()
	}
	pool.Amount -= amountToSub
	return s.SetPool(pool)
}

func (s *StateMachine) unmarshalPool(bz []byte) (*types.Pool, lib.ErrorI) {
	pool := new(types.Pool)
	if err := lib.Unmarshal(bz, pool); err != nil {
		return nil, err
	}
	return pool, nil
}

func (s *StateMachine) marshalPool(pool *types.Pool) ([]byte, lib.ErrorI) {
	return lib.Marshal(pool)
}
