package state_machine

import (
	"github.com/ginchuco/ginchu/types"
)

type StateMachine struct {
	store types.StoreI
}

func (s *StateMachine) Store() types.StoreI { return s.store }
