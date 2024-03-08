package state_machine

import (
	"github.com/ginchuco/ginchu/crypto"
	"github.com/ginchuco/ginchu/state_machine/types"
	lib "github.com/ginchuco/ginchu/types"
)

func (s *StateMachine) HandleMessage(msg lib.MessageI) (err lib.ErrorI) {
	switch x := msg.(type) {
	case *types.MessageSend:
		return s.HandleMessageSend(x)
	case *types.MessageStake:
		return s.HandleMessageStake(x)
	case *types.MessageEditStake:
		return s.HandleMessageEditStake(x)
	case *types.MessageUnstake:
		return s.HandleMessageUnstake(x)
	case *types.MessageUnpause:
		return s.HandleMessageUnpause(x)
	case *types.MessageChangeParameter:
		return s.HandleMessageChangeParameter(x)
	case *types.MessageDoubleSign:
		return s.HandleMessageDoubleSign(x)
	default:
		return types.ErrUnknownMessage(x)
	}
}

func (s *StateMachine) HandleMessageSend(msg *types.MessageSend) lib.ErrorI {
	// subtract from sender
	if err := s.AccountSub(crypto.NewAddressFromBytes(msg.FromAddress), msg.Amount); err != nil {
		return err
	}
	// add to recipient
	return s.AccountAdd(crypto.NewAddressFromBytes(msg.ToAddress), msg.Amount)
}

func (s *StateMachine) HandleMessageStake(msg *types.MessageStake) lib.ErrorI {
	//TODO implement me
	panic("implement me")
}

func (s *StateMachine) HandleMessageEditStake(msg *types.MessageEditStake) lib.ErrorI {
	//TODO implement me
	panic("implement me")
}

func (s *StateMachine) HandleMessageUnstake(msg *types.MessageUnstake) lib.ErrorI {
	//TODO implement me
	panic("implement me")
}

func (s *StateMachine) HandleMessagePause(msg *types.MessagePause) lib.ErrorI {
	//TODO implement me
	panic("implement me")
}

func (s *StateMachine) HandleMessageUnpause(msg *types.MessageUnpause) lib.ErrorI {
	//TODO implement me
	panic("implement me")
}

func (s *StateMachine) HandleMessageChangeParameter(msg *types.MessageChangeParameter) lib.ErrorI {
	//TODO implement me
	panic("implement me")
}

func (s *StateMachine) HandleMessageDoubleSign(msg *types.MessageDoubleSign) lib.ErrorI {
	//TODO implement me
	panic("implement me")
}
