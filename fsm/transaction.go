package fsm

import (
	"github.com/canopy-network/canopy/fsm/types"
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
	"google.golang.org/protobuf/types/known/anypb"
)

/* This file contains transaction handling logic - for the payload handling check message.go */

// ApplyTransaction() processes the transaction within the state machine, returning the corresponding TxResult.
func (s *StateMachine) ApplyTransaction(index uint64, transaction []byte, txHash string) (*lib.TxResult, lib.ErrorI) {
	// validate the transaction and get the check result
	result, err := s.CheckTx(transaction, txHash)
	if err != nil {
		return nil, err
	}
	// deduct fees for the transaction
	if err = s.AccountDeductFees(result.sender, result.tx.Fee); err != nil {
		return nil, err
	}
	// handle the message (payload)
	if err = s.HandleMessage(result.msg); err != nil {
		return nil, err
	}
	// return the tx result
	return &lib.TxResult{
		Sender:      result.sender.Bytes(),
		Recipient:   result.msg.Recipient(),
		MessageType: result.msg.Name(),
		Height:      s.Height(),
		Index:       index,
		Transaction: result.tx,
		TxHash:      txHash,
	}, nil
}

// CheckTx() validates the transaction object
func (s *StateMachine) CheckTx(transaction []byte, txHash string) (result *CheckTxResult, err lib.ErrorI) {
	// create a new transaction object reference to ensure a non-nil transaction
	tx := new(lib.Transaction)
	// populate the object ref with the bytes of the transaction
	if err = lib.Unmarshal(transaction, tx); err != nil {
		return
	}
	// perform basic validations against the tx object
	if err = tx.CheckBasic(); err != nil {
		return
	}
	// validate the timestamp (prune friendly - replay protection)
	if err = s.CheckReplay(tx, txHash); err != nil {
		return
	}
	// perform basic validations against the message payload
	msg, err := s.CheckMessage(tx.Msg)
	if err != nil {
		return
	}
	// validate the signature of the transaction
	sender, err := s.CheckSignature(msg, tx)
	if err != nil {
		return
	}
	// validate the fee associated with the transaction
	if err = s.CheckFee(tx.Fee, msg); err != nil {
		return
	}
	// return the result
	return &CheckTxResult{
		tx:     tx,
		msg:    msg,
		sender: sender,
	}, nil
}

// CheckTxResult is the result object from CheckTx()
type CheckTxResult struct {
	tx     *lib.Transaction // the transaction object
	msg    lib.MessageI     // the payload message in the transaction
	sender crypto.AddressI  // the sender address of the transaction
}

// CheckSignature() validates the signer and the digital signature associated with the transaction object
func (s *StateMachine) CheckSignature(msg lib.MessageI, tx *lib.Transaction) (crypto.AddressI, lib.ErrorI) {
	// validate the actual signature bytes
	if tx.Signature == nil || len(tx.Signature.Signature) == 0 {
		return nil, types.ErrEmptySignature()
	}
	// get the canonical byte representation of the transaction
	signBytes, err := tx.GetSignBytes()
	if err != nil {
		return nil, types.ErrTxSignBytes(err)
	}
	// convert signature bytes to public key object
	publicKey, e := crypto.NewPublicKeyFromBytes(tx.Signature.PublicKey)
	if e != nil {
		return nil, types.ErrInvalidPublicKey(e)
	}
	// validate the actual signature
	if !publicKey.VerifyBytes(signBytes, tx.Signature.Signature) {
		return nil, types.ErrInvalidSignature()
	}
	// calculate the corresponding address from the public key
	address := publicKey.Address()
	// check the authorized signers for the message
	authorizedSigners, er := s.GetAuthorizedSignersFor(msg)
	if er != nil {
		return nil, er
	}
	// for each authorized signer
	for _, authorized := range authorizedSigners {
		// if the address that signed the transaction matches one of the authorized signers
		if address.Equals(crypto.NewAddressFromBytes(authorized)) {
			// populate the signer field for stake
			if stake, ok := msg.(*types.MessageStake); ok {
				stake.Signer = authorized
			}
			// populate the signer field for edit-stake
			if editStake, ok := msg.(*types.MessageEditStake); ok {
				editStake.Signer = authorized
			}
			// return the signer address
			return address, nil
		}
	}
	// if no authorized signer matched the signer address, it's unauthorized
	return nil, types.ErrUnauthorizedTx()
}

// CheckReplay() validates the timestamp of the transaction
// Instead of using an increasing 'sequence number' Canopy uses timestamp + created block to act as a prune-friendly, replay attack / hash collision prevention mechanism
//   - Canopy searches the transaction indexer for the transaction using its hash to prevent 'replay attacks'
//   - The timestamp protects against hash collisions as it injects 'micro-second level entropy'
//     into the hash of the transaction, ensuring no transactions will 'accidentally collide'
//   - The created block acceptance policy for transactions maintains an acceptable bound of 'time' to support database pruning
func (s *StateMachine) CheckReplay(tx *lib.Transaction, txHash string) lib.ErrorI {
	// ensure the right network
	if uint64(s.NetworkID) != tx.NetworkId {
		return lib.ErrWrongNetworkID()
	}
	// ensure the right chain
	if s.Config.ChainId != tx.ChainId {
		return lib.ErrWrongChainId()
	}
	// if below height 2, skip this check as GetBlockByHeight will load a block that has a lastQC that doesn't exist
	if s.Height() < 2 {
		return nil
	}
	// ensure the store can 'read the indexer'
	store, ok := s.store.(lib.RIndexerI)
	// if it can't then exit
	if !ok {
		return types.ErrWrongStoreType()
	}
	// convert the transaction hash string into bytes
	hashBz, err := lib.StringToBytes(txHash)
	if err != nil {
		return err
	}
	// ensure the tx doesn't already exist in the indexer
	// same block replays are protected at a higher level
	txResult, err := store.GetTxByHash(hashBz)
	if err != nil {
		return err
	}
	// if the tx transaction result isn't nil, and it has a hash
	if txResult != nil && txResult.TxHash == txHash {
		return lib.ErrDuplicateTx(txHash)
	}
	// define some safe +/- tx indexer prune height
	const blockAcceptancePolicy = 120
	// this gives the protocol a theoretically safe tx indexer prune height
	maxHeight, minHeight := s.Height()+blockAcceptancePolicy, uint64(0)
	// if height is after the blockAcceptancePolicy blocks
	if s.Height() > blockAcceptancePolicy {
		// update the minimum height
		minHeight = s.Height() - blockAcceptancePolicy
	}
	// ensure the tx 'created height' is not above or below the acceptable bounds
	if tx.CreatedHeight > maxHeight || tx.CreatedHeight < minHeight {
		return lib.ErrInvalidTxHeight()
	}
	// exit
	return nil
}

// CheckMessage() performs basic validations on the msg payload
func (s *StateMachine) CheckMessage(msg *anypb.Any) (message lib.MessageI, err lib.ErrorI) {
	// ensure the message isn't nil
	if msg == nil {
		return nil, lib.ErrEmptyMessage()
	}
	// extract the message from an protobuf any
	proto, err := lib.FromAny(msg)
	if err != nil {
		return nil, err
	}
	// cast the proto message to a Message interface that may be interpreted
	message, ok := proto.(lib.MessageI)
	// if cast fails, throw an error
	if !ok {
		return nil, types.ErrInvalidTxMessage()
	}
	// do stateless checks on the message
	if err = message.Check(); err != nil {
		return nil, err
	}
	// return the message as the interface
	return message, nil
}

// CheckFee() validates the fee amount is sufficient to pay for a transaction
func (s *StateMachine) CheckFee(fee uint64, msg lib.MessageI) (err lib.ErrorI) {
	// get the fee for the message name
	stateLimitFee, err := s.GetFeeForMessageName(msg.Name())
	if err != nil {
		return err
	}
	// if the fee is below the limit
	if fee < stateLimitFee {
		return types.ErrTxFeeBelowStateLimit()
	}
	// exit
	return
}
