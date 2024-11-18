package fsm

import (
	"github.com/canopy-network/canopy/fsm/types"
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
	"google.golang.org/protobuf/types/known/anypb"
	"time"
)

// ApplyTransaction() processes the transaction within the state machine, returning the corresponding TxResult.
func (s *StateMachine) ApplyTransaction(index uint64, transaction []byte, txHash string) (*lib.TxResult, lib.ErrorI) {
	// validate the transaction and get the check result
	result, err := s.CheckTx(transaction)
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
func (s *StateMachine) CheckTx(transaction []byte) (result *CheckTxResult, err lib.ErrorI) {
	// convert the transaction bytes into an object
	tx := new(lib.Transaction)
	if err = lib.Unmarshal(transaction, tx); err != nil {
		return
	}
	// perform basic validations against the tx object
	if err = tx.CheckBasic(); err != nil {
		return
	}
	// validate the timestamp (prune friendly - replay protection)
	if err = s.CheckTimestamp(tx); err != nil {
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
	// validate the signature
	if !publicKey.VerifyBytes(signBytes, tx.Signature.Signature) {
		return nil, types.ErrInvalidSignature()
	}
	address := publicKey.Address()
	signers, er := s.GetAuthorizedSignersFor(msg)
	if er != nil {
		return nil, er
	}
	for _, signer := range signers {
		if address.Equals(crypto.NewAddressFromBytes(signer)) {
			// edit stake is a special case where the signer must be known by the handler
			if editStake, ok := msg.(*types.MessageEditStake); ok {
				editStake.Signer = signer
			}
			return address, nil
		}
	}
	return nil, types.ErrUnauthorizedTx()
}

// CheckTimestamp() validates the timestamp of the transaction
// Instead of using an increasing 'sequence number' Canopy uses timestamps to act as a prune-friendly, replay attack / hash collision prevention mechanism
//   - Canopy searches the transaction indexer for the transaction using its hash to prevent 'replay attacks'
//   - The timestamp protects against hash collisions as it injects 'micro-second level entropy'
//     into the hash of the transaction, ensuring no transactions will 'accidentally collide'
//   - The timestamp acceptance policy for transactions maintains an acceptable bound of time to support database pruning
func (s *StateMachine) CheckTimestamp(tx *lib.Transaction) lib.ErrorI {
	height := s.Height()
	if height < 2 {
		return nil
	}
	block, err := s.LoadBlock(height - 1)
	if err != nil {
		return err
	}
	// this gives us a safe mempool to block acceptance while providing a safe tx indexer prune time
	// NOTE: due to factors like 'clock drift' the maximum prune should be no less than 24 hours
	clockVarianceAcceptancePolicy := 6 * time.Hour
	txTime, lastBlockTime := time.UnixMicro(int64(tx.Time)), time.UnixMicro(int64(block.BlockHeader.Time))
	minimumTime, maximumTime := lastBlockTime.Add(-1*clockVarianceAcceptancePolicy), lastBlockTime.Add(clockVarianceAcceptancePolicy)
	if txTime.Before(minimumTime) || txTime.After(maximumTime) {
		return types.ErrInvalidTxTime()
	}
	return nil
}

// CheckMessage() performs basic validations on the msg payload
func (s *StateMachine) CheckMessage(msg *anypb.Any) (message lib.MessageI, err lib.ErrorI) {
	proto, err := lib.FromAny(msg)
	if err != nil {
		return nil, err
	}
	message, ok := proto.(lib.MessageI)
	if !ok {
		return nil, types.ErrInvalidTxMessage()
	}
	if err = message.Check(); err != nil {
		return nil, err
	}
	return message, nil
}

// CheckFee() validates the fee amount is sufficient to pay for a transaction
func (s *StateMachine) CheckFee(fee uint64, msg lib.MessageI) lib.ErrorI {
	stateLimitFee, err := s.GetFeeForMessage(msg)
	if err != nil {
		return err
	}
	if fee < stateLimitFee {
		return types.ErrTxFeeBelowStateLimit()
	}
	return nil
}
