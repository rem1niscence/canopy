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
	case *types.MessageDoubleSign: // TODO likely remove and leave to QC in consensus layer
		return s.HandleMessageDoubleSign(x)
	//case *types.MessageInvalidBlock: TODO ^
	//	return s.HandleMessageInvalidBlock(x)
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
	publicKey := crypto.NewPublicKeyFromBytes(msg.PublicKey)
	address := publicKey.Address()
	// check if below minimum stake
	params, err := s.GetParamsVal()
	if err != nil {
		return err
	}
	lessThanMinimum, err := lib.StringsLess(msg.Amount, params.ValidatorMinStake.Value)
	if err != nil {
		return err
	}
	if lessThanMinimum {
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
	// set validator
	return s.SetValidator(&types.Validator{
		Address:      address.Bytes(),
		PublicKey:    publicKey.Bytes(),
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
	cmp, err := lib.StringsCmp(msg.Amount, val.StakedAmount)
	if err != nil {
		return err
	}
	amountToAdd := "0"
	switch cmp {
	case -1: // amount less than stake
		return types.ErrInvalidAmount()
	case 0: // amount equals stake
	case 1: // amount greater than stake
		if amountToAdd, err = lib.StringSub(msg.Amount, val.StakedAmount); err != nil {
			return err
		}
	}
	// subtract from sender
	if err = s.AccountSub(address, amountToAdd); err != nil {
		return err
	}
	// update validator stake amount
	newStakedAmount, err := lib.StringAdd(val.StakedAmount, amountToAdd)
	if err != nil {
		return err
	}
	// set validator
	return s.SetValidator(&types.Validator{
		Address:         val.Address,
		PublicKey:       val.PublicKey,
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
	unstakingBlocks := p.GetValidatorUnstakingBlocks().Value
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
	params, err := s.GetParamsVal()
	if err != nil {
		return err
	}
	maxPausedHeight := s.Height() + params.ValidatorMaxPauseBlocks.Value
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
	// set validator unpaused
	return s.SetValidatorUnpaused(address, validator)
}

func (s *StateMachine) HandleMessageChangeParameter(msg *types.MessageChangeParameter) lib.ErrorI {
	address := crypto.NewAddressFromBytes(msg.Owner)
	protoMsg, err := types.FromAny(msg.ParameterValue)
	if err != nil {
		return err
	}
	return s.UpdateParam(address.String(), msg.ParameterSpace, msg.ParameterKey, protoMsg)
}

func (s *StateMachine) HandleMessageDoubleSign(msg *types.MessageDoubleSign) lib.ErrorI {
	doubleSignerPK := crypto.NewPublicKeyFromBytes(msg.VoteA.PublicKey)
	doubleSignerAddr := doubleSignerPK.Address()
	reporterAddr := crypto.NewAddressFromBytes(msg.ReporterAddress)
	params, err := s.GetParamsVal()
	if err != nil {
		return err
	}
	doubleSigner, err := s.GetValidator(doubleSignerAddr)
	if err != nil {
		return err
	}
	reporter, err := s.GetValidator(reporterAddr)
	if err != nil {
		return err
	}
	reporterOutputAddr := crypto.NewAddressFromBytes(reporter.Output)
	if err = s.SlashValidator(doubleSigner, params.ValidatorDoubleSignSlashPercentage.Value); err != nil {
		return err
	}
	amount, err := lib.StringToBigInt(params.ValidatorDoubleSignReporterReward.Value)
	if err != nil {
		return err
	}
	return s.MintToAccount(reporterOutputAddr, amount)
}
