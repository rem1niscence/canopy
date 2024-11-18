package fsm

import (
	"github.com/canopy-network/canopy/fsm/types"
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
	"slices"
)

// HandleByzantine() handles the byzantine (faulty/malicious) participants from a QuorumCertificate
func (s *StateMachine) HandleByzantine(qc *lib.QuorumCertificate, vs *lib.ConsensusValidators, params *types.ValidatorParams) (nonSignerPercent int, err lib.ErrorI) {
	if s.height <= 2 {
		return // height 2 would use height 1 as begin_block which uses genesis as lastQC
	}
	slashRecipients := qc.Results.SlashRecipients
	// if the non-signers window has completed, reset the window and slash non-signers
	if s.Height()%params.ValidatorNonSignWindow == 0 {
		if err = s.SlashAndResetNonSigners(qc.Header.CommitteeId, params); err != nil {
			return 0, err
		}
	}
	// sanity check the slash recipient isn't nil
	if slashRecipients != nil {
		// slash proposers who missed or skipped their proposal turn after provably receiving a valid ElectionQC confirming they were the proposer
		if err = s.HandleBadProposers(qc.Header.CommitteeId, params, slashRecipients.BadProposers); err != nil {
			return 0, err
		}
		// set in state and slash double signers
		if err = s.HandleDoubleSigners(qc.Header.CommitteeId, params, slashRecipients.DoubleSigners); err != nil {
			return 0, err
		}
	}
	// get those who did not sign this particular QC but should have
	nonSigners, nonSignerPercent, err := qc.GetNonSigners(vs)
	if err != nil {
		return 0, err
	}
	// increment the non-signing count for the non-signers
	if err = s.IncrementNonSigners(nonSigners); err != nil {
		return 0, err
	}
	return
}

// SlashAndResetNonSigners() resets the non-signer tracking and slashes those who exceeded the ValidatorMaxNonSign threshold
func (s *StateMachine) SlashAndResetNonSigners(committeeId uint64, params *types.ValidatorParams) lib.ErrorI {
	var keys, badList [][]byte
	// this callback slashes the validator if exceeded the ValidatorMaxNonSign threshold
	// it is executed for every key found under 'non-signers'
	callback := func(k, v []byte) lib.ErrorI {
		// track non-signer keys to delete
		keys = append(keys, k)
		// for each non-signer, see if they exceeded the threshold
		// if so - add them to the bad list
		addr, err := types.AddressFromKey(k)
		if err != nil {
			return err
		}
		ptr := new(types.NonSigner)
		if err = lib.Unmarshal(v, ptr); err != nil {
			return err
		}
		if ptr.Counter > params.ValidatorMaxNonSign {
			badList = append(badList, addr.Bytes())
		}
		return nil
	}
	// execute the callback for each key under 'non-signer' prefix
	if err := s.IterateAndExecute(types.NonSignerPrefix(), callback); err != nil {
		return err
	}
	// pause all on the bad list
	s.SetValidatorsPaused(badList)
	// slash all on the bad list
	if err := s.SlashNonSigners(committeeId, params, badList); err != nil {
		return err
	}
	// delete all keys under 'non-signer' prefix as part of the reset
	_ = s.DeleteAll(keys)
	return nil
}

// GetNonSigners() returns all non-(QC)-signers save in the state
func (s *StateMachine) GetNonSigners() (results types.NonSigners, e lib.ErrorI) {
	it, e := s.Iterator(types.NonSignerPrefix())
	if e != nil {
		return nil, e
	}
	defer it.Close()
	for ; it.Valid(); it.Next() {
		addr, err := types.AddressFromKey(it.Key())
		if err != nil {
			return nil, err
		}
		ptr := new(types.NonSigner)
		if err = lib.Unmarshal(it.Value(), ptr); err != nil {
			return nil, err
		}
		results = append(results, &types.NonSigner{
			Address: addr.Bytes(),
			Counter: ptr.Counter,
		})
	}
	return results, nil
}

// GetDoubleSigners() returns all double signers save in the state
// IMPORTANT NOTE: this returns <address> -> <heights> NOT <pubic_key> -> <heights>
func (s *StateMachine) GetDoubleSigners() (results []*lib.DoubleSigner, e lib.ErrorI) {
	return s.Store().(lib.StoreI).GetDoubleSigners()
}

// IncrementNonSigners() upserts non-(QC)-signers by incrementing the non-signer count for address(es)
func (s *StateMachine) IncrementNonSigners(nonSigners [][]byte) lib.ErrorI {
	for _, ns := range nonSigners {
		pubKey, e := crypto.NewPublicKeyFromBytes(ns)
		if e != nil {
			return lib.ErrPubKeyFromBytes(e)
		}
		key := types.KeyForNonSigner(pubKey.Address().Bytes())
		ptr := new(types.NonSigner)
		bz, err := s.Get(key)
		if err != nil {
			return err
		}
		if err = lib.Unmarshal(bz, ptr); err != nil {
			return err
		}
		ptr.Counter++
		bz, err = lib.Marshal(ptr)
		if err != nil {
			return err
		}
		if err = s.Set(key, bz); err != nil {
			return err
		}
	}
	return nil
}

// HandleDoubleSigners() validates, sets, and slashes the list of doubleSigners
func (s *StateMachine) HandleDoubleSigners(committeeId uint64, params *types.ValidatorParams, doubleSigners []*lib.DoubleSigner) lib.ErrorI {
	store := s.Store().(lib.StoreI)
	var badList [][]byte
	for _, doubleSigner := range doubleSigners {
		// sanity check
		if doubleSigner == nil || doubleSigner.PubKey == nil {
			return lib.ErrInvalidEvidence()
		}
		if len(doubleSigner.Heights) < 1 {
			return lib.ErrInvalidDoubleSignHeights()
		}
		pubKey, e := crypto.NewPublicKeyFromBytes(doubleSigner.PubKey)
		if e != nil {
			return lib.ErrPubKeyFromBytes(e)
		}
		address := pubKey.Address().Bytes()
		// for each double sign height
		for _, height := range doubleSigner.Heights {
			isValidDS, err := store.IsValidDoubleSigner(address, height)
			if err != nil {
				return err
			}
			if !isValidDS {
				return lib.ErrInvalidDoubleSigner()
			}
			if err = store.IndexDoubleSigner(address, height); err != nil {
				return err
			}
			// add to bad list
			badList = append(badList, pubKey.Address().Bytes())
		}
	}
	return s.SlashDoubleSigners(committeeId, params, badList)
}

// IsValidDoubleSigner() checks if the double signer was already slashed for this height
// this prevents evidence re-use
func (s *StateMachine) IsValidDoubleSigner(height uint64, address []byte) bool {
	isValid, _ := s.Store().(lib.StoreI).IsValidDoubleSigner(address, height)
	return isValid
}

// HandleBadProposers() is a pass through call to SlashBadProposers but converts public keys into addresses
func (s *StateMachine) HandleBadProposers(committeeId uint64, params *types.ValidatorParams, badProposerPublicKeys [][]byte) lib.ErrorI {
	var badList [][]byte
	for _, badProposer := range badProposerPublicKeys {
		pubKey, err := crypto.NewPublicKeyFromBytes(badProposer)
		if err != nil {
			return lib.ErrPubKeyFromBytes(err)
		}
		badList = append(badList, pubKey.Address().Bytes())
	}
	return s.SlashBadProposers(committeeId, params, badList)
}

// SlashNonSigners() burns the staked tokens of non-(QC)-signers
func (s *StateMachine) SlashNonSigners(committeeId uint64, params *types.ValidatorParams, nonSigners [][]byte) lib.ErrorI {
	return s.SlashValidators(nonSigners, committeeId, params.ValidatorNonSignSlashPercentage, params)
}

// SlashNonSigners() burns the staked tokens of bad proposers
func (s *StateMachine) SlashBadProposers(committeeId uint64, params *types.ValidatorParams, badProposers [][]byte) lib.ErrorI {
	return s.SlashValidators(badProposers, committeeId, params.ValidatorBadProposalSlashPercentage, params)
}

// SlashNonSigners() burns the staked tokens of double signers
func (s *StateMachine) SlashDoubleSigners(committeeId uint64, params *types.ValidatorParams, doubleSigners [][]byte) lib.ErrorI {
	return s.SlashValidators(doubleSigners, committeeId, params.ValidatorDoubleSignSlashPercentage, params)
}

// ForceUnstakeValidator() automatically begins unstaking the validator
func (s *StateMachine) ForceUnstakeValidator(address crypto.AddressI) lib.ErrorI {
	// get validator
	validator, err := s.GetValidator(address)
	if err != nil {
		s.log.Warnf("validator %s is not found to be force unstaked", address.String())
		return nil
	}
	// check if already unstaking
	if validator.UnstakingHeight != 0 {
		s.log.Warnf("validator %s is already unstaking can't be forced to begin unstaking", address.String())
		return nil
	}
	// get params for unstaking blocks
	p, err := s.GetParamsVal()
	if err != nil {
		return err
	}
	// set validator unstaking
	return s.forceUnstakeValidator(address, validator, p)
}

// forceUnstakeValidator() automatically begins unstaking the validator (helper)
func (s *StateMachine) forceUnstakeValidator(address crypto.AddressI, val *types.Validator, p *types.ValidatorParams) lib.ErrorI {
	unstakingBlocks := p.GetValidatorUnstakingBlocks()
	unstakingHeight := s.Height() + unstakingBlocks
	// set validator unstaking
	return s.SetValidatorUnstaking(address, val, unstakingHeight)
}

// SlashValidators() burns a specified percentage of multiple validator's staked tokens
func (s *StateMachine) SlashValidators(addresses [][]byte, committeeId, percent uint64, p *types.ValidatorParams) lib.ErrorI {
	for _, addr := range addresses {
		validator, err := s.GetValidator(crypto.NewAddressFromBytes(addr))
		if err != nil {
			return err
		}
		if err = s.SlashValidator(validator, committeeId, percent, p); err != nil {
			return err
		}
	}
	return nil
}

// SlashValidator() burns a specified percentage of a validator's staked tokens
func (s *StateMachine) SlashValidator(validator *types.Validator, committeeId, percent uint64, p *types.ValidatorParams) (err lib.ErrorI) {
	// ensure no unauthorized slashes may occur
	if !slices.Contains(validator.Committees, committeeId) {
		return types.ErrInvalidCommitteeID()
	}
	// if a 'slash tracker' is used to limit the max slash per committee per block
	if s.slashTracker != nil {
		// get the slashed percent so far in this block by this committee
		slashTotal := s.slashTracker.GetTotalSlashPercent(validator.Address, committeeId)
		// check to see if it exceeds the max
		if slashTotal >= p.ValidatorMaxSlashPerCommittee {
			return nil // no slash nor no removal logic occurs because this block already hit the limit with a previous slash
		}
		// check to see if it 'now' exceeds the max
		if slashTotal+percent >= p.ValidatorMaxSlashPerCommittee {
			// only slash up to the maximum
			percent = p.ValidatorMaxSlashPerCommittee - slashTotal
			// get the number of committees for this validator
			numCommittees := len(validator.Committees)
			// defensive coding, this function basically requires 1 committee
			if numCommittees != 0 {
				for i, id := range validator.Committees {
					if id == committeeId {
						// remove the committee from the validator
						validator.Committees = append(validator.Committees[:i], validator.Committees[i+1:]...)
						break
					}
				}
			}
		}
		// update the slash tracker
		// NOTE: the slash tracker is automatically reset every block
		s.slashTracker.AddSlash(validator.Address, committeeId, percent)
	}
	// initialize address and new stake variable
	addr, stakeAfterSlash := crypto.NewAddressFromBytes(validator.Address), lib.Uint64ReducePercentage(validator.StakedAmount, percent)
	// calculate the slash amount
	slashAmount := validator.StakedAmount - stakeAfterSlash
	// subtract from total supply
	if err = s.SubFromTotalSupply(slashAmount); err != nil {
		return err
	}
	// subtract from staked supply
	if err = s.SubFromStakedSupply(slashAmount); err != nil {
		return err
	}
	// update the committees based on the new stake amount
	if err = s.UpdateCommittees(addr, validator, stakeAfterSlash, validator.Committees); err != nil {
		return err
	}
	// update the stake amount and set the validator
	validator.StakedAmount = stakeAfterSlash
	// update the validator
	return s.SetValidator(validator)
}

// LoadMinimumEvidenceHeight() loads the minimum height timestamp evidence must have to still be applicable
func (s *StateMachine) LoadMinimumEvidenceHeight() (uint64, lib.ErrorI) {
	valParams, err := s.GetParamsVal()
	if err != nil {
		return 0, err
	}
	height, unstakingBlocks := s.Height(), valParams.GetValidatorUnstakingBlocks()
	if height < unstakingBlocks {
		return 0, nil
	}
	return height - unstakingBlocks, nil
}
