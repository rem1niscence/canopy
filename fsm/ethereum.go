package fsm

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"math/big"
	"time"
)

/* This file implements an Ethereum based wrapper over send transactions in order to allow popular tooling to interact with Canopy */

// Flow
// 1. An Ethereum wallet creates the transaction
// 2. Canopy RPC wraps the RLP to a SendTransaction and gossips it like a normal transaction
// 3. If RLP detected during tx processing, verify the signature and generate a new SendTx from the RLP to verify equality

// CanopyPseudoContractAddress: a fake contract address that allows tools to utilize canopy as if it's an ERC20
const CanopyPseudoContractAddress = `0x837F636655dFa9B32663f30424F92aBb1Af40308`
const RLPIndicator = "RLP"

// RLPToSendTransaction() converts an RLP encoded transaction into a Canopy send transaction
func RLPToSendTransaction(txBytes []byte, networkId uint64) (transaction *lib.Transaction, e lib.ErrorI) {
	// protect against spam
	if len(txBytes) >= 400 {
		return nil, ErrInvalidRLPTx(fmt.Errorf("max transaction size"))
	}
	// decode transaction to ethereum object
	var tx ethTypes.Transaction
	if err := tx.UnmarshalBinary(txBytes); err != nil {
		return nil, ErrInvalidRLPTx(err)
	}
	// get the signer type (supports: Legacy, EIP-155, EIP-1559, EIP-2930, EIP-4844, EIP-7702)
	signer := ethTypes.LatestSignerForChainID(big.NewInt(1))
	// recover sender (from address)
	from, err := signer.Sender(&tx)
	if err != nil {
		return nil, ErrInvalidRLPTx(err)
	}
	// recover the public key from the rlp transaction and validate the signature
	publicKey, err := crypto.RecoverPublicKey(signer, tx)
	if err != nil {
		return nil, ErrInvalidPublicKey(err)
	}
	// generate the transaction boject
	transaction = &lib.Transaction{
		MessageType: MessageSendName,
		Signature: &lib.Signature{
			PublicKey: publicKey.Bytes(),
			Signature: txBytes, // store the raw transaction here
		},
		CreatedHeight: tx.Nonce(),
		Time:          pseudoEthereumTimestamp(tx.Gas()), // can be used as a pseudo-nonce if needed
		Fee:           DownscaleTo6Decimals(new(big.Int).Mul(big.NewInt(int64(tx.Gas())), tx.GasPrice())),
		NetworkId:     networkId,
		Memo:          RLPIndicator,
		ChainId:       tx.ChainId().Uint64(),
	}
	// generate a variable to hold message send
	msg := &MessageSend{
		FromAddress: from.Bytes(),
		ToAddress:   tx.To().Bytes(),
		Amount:      DownscaleTo6Decimals(tx.Value()),
	}
	// check if is an ERC20 send
	if tx.To().Hex() == CanopyPseudoContractAddress {
		msg, e = ParseERC20EthereumTransfer(from.Bytes(), hex.EncodeToString(tx.Data()))
		if e != nil {
			return
		}
	}
	// convert the message to an `any`
	transaction.Msg, e = lib.NewAny(msg)
	return
}

// ParseERC20EthereumTransfer() parses the 'data' field of an Ethereum transaction to use Canopy like it's an ERC20
func ParseERC20EthereumTransfer(fromAddress []byte, input string) (*MessageSend, lib.ErrorI) {
	// check input length (8 selector + 64 address + 64 amount = 136 hex chars + 2 for "0x")
	if len(input) < 8+64+64 {
		return nil, ErrInvalidERC20Tx(fmt.Errorf("input too short"))
	}
	// check the method selector directly from the hex string
	if input[:8] != "a9059cbb" {
		return nil, ErrInvalidERC20Tx(fmt.Errorf("not a transfer(address,uint256) call"))
	}
	// decode hex string to bytes (excluding "0x")
	data, err := lib.StringToBytes(input[:])
	if err != nil {
		return nil, ErrInvalidERC20Tx(err)
	}
	// amount: full 32-byte uint256
	amount := new(big.Int).SetBytes(data[36:68])
	// return the message send type
	return &MessageSend{
		ToAddress:   data[16:36],
		FromAddress: fromAddress,
		Amount:      amount.Uint64(), // take amount as is - because decimals is specified at the 'contract level'
	}, nil
}

// VerifyRLPBytes() implements special 'signature verification logic' that allows a MessageSend to be authenticated using a signed RLP transaction
func (s *StateMachine) VerifyRLPBytes(tx *lib.Transaction) lib.ErrorI {
	// create a compare transaction from the signature field
	compare, err := RLPToSendTransaction(tx.Signature.Signature, s.Config.NetworkID)
	if err != nil {
		return err
	}
	// get the transaction hash (includes the raw RLP) for the compare tx
	compareHash, err := compare.GetHash()
	if err != nil {
		return err
	}
	// get the transaction hash (includes the raw RLP) for the raw tx
	originalHash, err := tx.GetHash()
	if err != nil {
		return err
	}
	// check the equality of the two transactions
	if !bytes.Equal(compareHash, originalHash) {
		return ErrInvalidSignature()
	}
	// exit without error
	return nil
}

// pseudoEthereumTimestamp() creates a fake timestamp to ensure collision resistance using the gas limit variable
func pseudoEthereumTimestamp(gasLimit uint64) uint64 {
	return uint64(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(gasLimit) * time.Second).Unix())
}

var scaleFactor = big.NewInt(1_000_000_000_000) // 10^12

// UpscaleTo18Decimals converts a 6-decimal unit (Canopy native) to 18-decimal (Ethereum RPC)
func UpscaleTo18Decimals(amount uint64) *big.Int {
	return new(big.Int).Mul(big.NewInt(int64(amount)), scaleFactor)
}

// DownscaleTo6Decimals converts from 18-decimal unit (Ethereum RPC) to 6-decimal (Canopy native)
func DownscaleTo6Decimals(amount *big.Int) uint64 {
	return new(big.Int).Div(amount, scaleFactor).Uint64()
}
