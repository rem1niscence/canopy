package fsm

import (
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
)

func (s *StateMachine) HandleMessage(msg lib.MessageI) lib.ErrorI {
	switch x := msg.(type) {
	case *types.MessageSend:
		return s.HandleMessageSend(x)
	case *types.MessageStake:
		return s.HandleMessageStake(x)
	case *types.MessageEditStake:
		return s.HandleMessageEditStake(x)
	case *types.MessageUnstake:
		return s.HandleMessageUnstake(x)
	case *types.MessagePause:
		return s.HandleMessagePause(x)
	case *types.MessageUnpause:
		return s.HandleMessageUnpause(x)
	case *types.MessageChangeParameter:
		return s.HandleMessageChangeParameter(x)
	case *types.MessageDAOTransfer:
		return s.HandleMessageDAOTransfer(x)
	default:
		return types.ErrUnknownMessage(x)
	}
}

func (s *StateMachine) GetFeeForMessage(msg lib.MessageI) (fee uint64, err lib.ErrorI) {
	feeParams, err := s.GetParamsFee()
	if err != nil {
		return 0, err
	}
	switch x := msg.(type) {
	case *types.MessageSend:
		return feeParams.MessageSendFee, nil
	case *types.MessageStake:
		return feeParams.MessageStakeFee, nil
	case *types.MessageEditStake:
		return feeParams.MessageEditStakeFee, nil
	case *types.MessageUnstake:
		return feeParams.MessageUnstakeFee, nil
	case *types.MessageUnpause:
		return feeParams.MessageUnpauseFee, nil
	case *types.MessageChangeParameter:
		return feeParams.MessageChangeParameterFee, nil
	case *types.MessageDAOTransfer:
		return feeParams.MessageDaoTransferFee, nil
	default:
		return 0, types.ErrUnknownMessage(x)
	}
}

func (s *StateMachine) GetAuthorizedSignersFor(msg lib.MessageI) (signers [][]byte, err lib.ErrorI) {
	var validator *types.Validator
	switch x := msg.(type) {
	case *types.MessageSend:
		return [][]byte{x.FromAddress}, nil
	case *types.MessageChangeParameter:
		return [][]byte{x.Signer}, nil
	case *types.MessageDAOTransfer:
		return [][]byte{x.Address}, nil
	case *types.MessageStake:
		pubKey, e := crypto.NewPublicKeyFromBytes(x.PublicKey)
		if e != nil {
			return nil, types.ErrInvalidPublicKey(e)
		}
		validator, err = s.GetValidator(pubKey.Address())
		if err != nil {
			return nil, err
		}
	case *types.MessageEditStake:
		validator, err = s.GetValidator(crypto.NewAddressFromBytes(x.Address))
	case *types.MessageUnstake:
		validator, err = s.GetValidator(crypto.NewAddressFromBytes(x.Address))
	case *types.MessageUnpause:
		validator, err = s.GetValidator(crypto.NewAddressFromBytes(x.Address))
	default:
		return nil, types.ErrUnknownMessage(x)
	}
	if err != nil {
		return nil, err
	}
	if validator == nil {
		return nil, types.ErrValidatorNotExists()
	}
	return [][]byte{validator.Address, validator.Output}, nil
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
	publicKey, e := crypto.NewPublicKeyFromBytes(msg.PublicKey)
	if e != nil {
		return types.ErrInvalidPublicKey(e)
	}
	address := publicKey.Address()
	// check if below minimum stake
	params, err := s.GetParamsVal()
	if err != nil {
		return err
	}
	if msg.Amount < params.ValidatorMinStake {
		return types.ErrBelowMinimumStake()
	}
	// subtract from sender
	if err = s.AccountSub(address, msg.Amount); err != nil {
		return err
	}
	// check if validator exists
	exists, err := s.GetValidatorExists(address)
	if err != nil {
		return err
	}
	// fail if validator already exists
	if exists {
		return types.ErrValidatorExists()
	}
	// set validator sorted by stake
	if err = s.SetConsensusValidator(address, msg.Amount); err != nil {
		return err
	}
	if err = s.AddToStakedSupply(msg.Amount); err != nil {
		return err
	}
	// set validator
	return s.SetValidator(&types.Validator{
		Address:      address.Bytes(),
		PublicKey:    publicKey.Bytes(),
		NetAddress:   msg.NetAddress,
		StakedAmount: msg.Amount,
		Output:       msg.OutputAddress,
	})
}

func (s *StateMachine) HandleMessageEditStake(msg *types.MessageEditStake) lib.ErrorI {
	address := crypto.NewAddressFromBytes(msg.Address)
	// get the validator
	val, err := s.GetValidator(address)
	if err != nil {
		return err
	}
	// check unstaking
	if val.UnstakingHeight != 0 {
		return types.ErrValidatorUnstaking()
	}
	var amountToAdd uint64
	switch {
	case msg.Amount < val.StakedAmount: // amount less than stake
		return types.ErrInvalidAmount()
	case msg.Amount == val.StakedAmount: // amount equals stake
	case msg.Amount > val.StakedAmount: // amount greater than stake
		amountToAdd = msg.Amount - val.StakedAmount
	}
	// subtract from sender
	if err = s.AccountSub(address, amountToAdd); err != nil {
		return err
	}
	if err = s.AddToStakedSupply(amountToAdd); err != nil {
		return err
	}
	// update validator stake amount
	newStakedAmount := val.StakedAmount + amountToAdd
	// updated sorted validator set
	if err = s.UpdateConsensusValidator(address, val.StakedAmount, newStakedAmount); err != nil {
		return err
	}
	// set validator
	return s.SetValidator(&types.Validator{
		Address:         val.Address,
		PublicKey:       val.PublicKey,
		NetAddress:      msg.NetAddress,
		StakedAmount:    newStakedAmount,
		MaxPausedHeight: val.MaxPausedHeight,
		UnstakingHeight: val.UnstakingHeight,
		Output:          msg.OutputAddress,
	})
}

func (s *StateMachine) HandleMessageUnstake(msg *types.MessageUnstake) lib.ErrorI {
	address := crypto.NewAddressFromBytes(msg.Address)
	// get validator
	validator, err := s.GetValidator(address)
	if err != nil {
		return err
	}
	// check if already unstaking
	if validator.UnstakingHeight != 0 {
		return types.ErrValidatorUnstaking()
	}
	// get params for unstaking blocks
	p, err := s.GetParamsVal()
	if err != nil {
		return err
	}
	unstakingBlocks := p.GetValidatorUnstakingBlocks()
	unstakingHeight := s.Height() + unstakingBlocks
	// set validator unstaking
	return s.SetValidatorUnstaking(address, validator, unstakingHeight)
}

func (s *StateMachine) HandleMessagePause(msg *types.MessagePause) lib.ErrorI {
	address := crypto.NewAddressFromBytes(msg.Address)
	// get validator
	validator, err := s.GetValidator(address)
	if err != nil {
		return err
	}
	// ensure not already paused
	if validator.MaxPausedHeight != 0 {
		return types.ErrValidatorPaused()
	}
	if validator.UnstakingHeight != 0 {
		return types.ErrValidatorUnstaking()
	}
	params, err := s.GetParamsVal()
	if err != nil {
		return err
	}
	maxPausedHeight := s.Height() + params.ValidatorMaxPauseBlocks
	// set validator paused
	return s.SetValidatorPaused(address, validator, maxPausedHeight)
}

func (s *StateMachine) HandleMessageUnpause(msg *types.MessageUnpause) lib.ErrorI {
	address := crypto.NewAddressFromBytes(msg.Address)
	// get validator
	validator, err := s.GetValidator(address)
	if err != nil {
		return err
	}
	// ensure already paused
	if validator.MaxPausedHeight == 0 {
		return types.ErrValidatorNotPaused()
	}
	if validator.UnstakingHeight != 0 {
		return types.ErrValidatorUnstaking()
	}
	// set validator unpaused
	return s.SetValidatorUnpaused(address, validator)
}

func (s *StateMachine) HandleMessageChangeParameter(msg *types.MessageChangeParameter) lib.ErrorI {
	if err := s.ApproveProposal(msg); err != nil {
		return types.ErrRejectProposal()
	}
	protoMsg, err := lib.FromAny(msg.ParameterValue)
	if err != nil {
		return err
	}
	return s.UpdateParam(msg.ParameterSpace, msg.ParameterKey, protoMsg)
}

func (s *StateMachine) HandleMessageDAOTransfer(msg *types.MessageDAOTransfer) lib.ErrorI {
	if err := s.ApproveProposal(msg); err != nil {
		return types.ErrRejectProposal()
	}
	if err := s.PoolSub(types.PoolID_DAO, msg.Amount); err != nil {
		return err
	}
	return s.AccountAdd(crypto.NewAddressFromBytes(msg.Address), msg.Amount)
}
