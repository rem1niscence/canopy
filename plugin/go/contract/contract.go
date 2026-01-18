package contract

import (
	"bytes"
	"encoding/binary"
	"log"
	"math/rand"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
)

/* This file contains the base contract implementation that overrides the basic 'transfer' functionality */

// PluginConfig: the configuration of the contract
var ContractConfig = &PluginConfig{
	Name:    "go_plugin_contract",
	Id:      1,
	Version: 1,
	SupportedTransactions: []string{"send", "reward", "faucet"},
	TransactionTypeUrls: []string{
		"type.googleapis.com/types.MessageSend",
		"type.googleapis.com/types.MessageReward",
		"type.googleapis.com/types.MessageFaucet",
	},
	EventTypeUrls: nil,
}

// init sets FileDescriptorProtos after ensuring .pb.go files are initialized
func init() {
	// Explicitly initialize the proto files first to ensure File_*_proto are set
	file_account_proto_init()
	file_event_proto_init()
	file_plugin_proto_init()
	file_tx_proto_init()

	var fds [][]byte
	// Include google/protobuf/any.proto first as it's a dependency of event.proto and tx.proto
	for _, file := range []protoreflect.FileDescriptor{
		anypb.File_google_protobuf_any_proto,
		File_account_proto, File_event_proto, File_plugin_proto, File_tx_proto,
	} {
		fd, _ := proto.Marshal(protodesc.ToFileDescriptorProto(file))
		fds = append(fds, fd)
	}
	ContractConfig.FileDescriptorProtos = fds
}

// Contract() defines the smart contract that implements the extended logic of the nested chain
type Contract struct {
	Config    Config
	FSMConfig *PluginFSMConfig // fsm configuration
	plugin    *Plugin          // plugin connection
	fsmId     uint64           // the id of the requesting fsm
}

// Genesis() implements logic to import a json file to create the state at height 0 and export the state at any height
func (c *Contract) Genesis(_ *PluginGenesisRequest) *PluginGenesisResponse {
	return &PluginGenesisResponse{} // TODO map out original token holders
}

// BeginBlock() is code that is executed at the start of `applying` the block
func (c *Contract) BeginBlock(_ *PluginBeginRequest) *PluginBeginResponse {
	return &PluginBeginResponse{}
}

// CheckTx() is code that is executed to statelessly validate a transaction
func (c *Contract) CheckTx(request *PluginCheckRequest) *PluginCheckResponse {
	// validate fee
	resp, err := c.plugin.StateRead(c, &PluginStateReadRequest{
		Keys: []*PluginKeyRead{
			{QueryId: rand.Uint64(), Key: KeyForFeeParams()},
		}})
	if err == nil {
		err = resp.Error
	}
	// handle error
	if err != nil {
		return &PluginCheckResponse{Error: err}
	}
	// convert bytes into fee parameters
	minFees := new(FeeParams)
	if err = Unmarshal(resp.Results[0].Entries[0].Value, minFees); err != nil {
		return &PluginCheckResponse{Error: err}
	}
	// check for the minimum fee
	if request.Tx.Fee < minFees.SendFee {
		return &PluginCheckResponse{Error: ErrTxFeeBelowStateLimit()}
	}
	// get the message
	msg, err := FromAny(request.Tx.Msg)
	if err != nil {
		return &PluginCheckResponse{Error: err}
	}
	// handle the message
	switch x := msg.(type) {
	case *MessageSend:
		return c.CheckMessageSend(x)
	case *MessageReward:
		return c.CheckMessageReward(x)
	case *MessageFaucet:
		return c.CheckMessageFaucet(x)
	default:
		return &PluginCheckResponse{Error: ErrInvalidMessageCast()}
	}
}

// CheckMessageFaucet statelessly validates a 'faucet' message
func (c *Contract) CheckMessageFaucet(msg *MessageFaucet) *PluginCheckResponse {
	log.Printf("CheckMessageFaucet called: signer=%x recipient=%x amount=%d",
		msg.SignerAddress, msg.RecipientAddress, msg.Amount)
	// Validate signer address (must be 20 bytes)
	if len(msg.SignerAddress) != 20 {
		log.Printf("CheckMessageFaucet: invalid signer address length %d", len(msg.SignerAddress))
		return &PluginCheckResponse{Error: ErrInvalidAddress()}
	}
	// Validate recipient address
	if len(msg.RecipientAddress) != 20 {
		log.Printf("CheckMessageFaucet: invalid recipient address length %d", len(msg.RecipientAddress))
		return &PluginCheckResponse{Error: ErrInvalidAddress()}
	}
	// Validate amount
	if msg.Amount == 0 {
		log.Printf("CheckMessageFaucet: invalid amount 0")
		return &PluginCheckResponse{Error: ErrInvalidAmount()}
	}
	log.Printf("CheckMessageFaucet: returning authorizedSigners=%x", msg.SignerAddress)
	// Return authorized signers (signer must sign this tx)
	return &PluginCheckResponse{
		Recipient:         msg.RecipientAddress,
		AuthorizedSigners: [][]byte{msg.SignerAddress},
	}
}

// CheckMessageReward statelessly validates a 'reward' message
func (c *Contract) CheckMessageReward(msg *MessageReward) *PluginCheckResponse {
	log.Printf("CheckMessageReward called: admin=%x recipient=%x amount=%d",
		msg.AdminAddress, msg.RecipientAddress, msg.Amount)
	// Validate admin address (must be 20 bytes)
	if len(msg.AdminAddress) != 20 {
		log.Printf("CheckMessageReward: invalid admin address length %d", len(msg.AdminAddress))
		return &PluginCheckResponse{Error: ErrInvalidAddress()}
	}
	// Validate recipient address
	if len(msg.RecipientAddress) != 20 {
		log.Printf("CheckMessageReward: invalid recipient address length %d", len(msg.RecipientAddress))
		return &PluginCheckResponse{Error: ErrInvalidAddress()}
	}
	// Validate amount
	if msg.Amount == 0 {
		log.Printf("CheckMessageReward: invalid amount 0")
		return &PluginCheckResponse{Error: ErrInvalidAmount()}
	}
	log.Printf("CheckMessageReward: returning authorizedSigners=%x", msg.AdminAddress)
	// Return authorized signers (admin must sign this tx)
	return &PluginCheckResponse{
		Recipient:         msg.RecipientAddress,
		AuthorizedSigners: [][]byte{msg.AdminAddress},
	}
}

// DeliverTx() is code that is executed to apply a transaction
func (c *Contract) DeliverTx(request *PluginDeliverRequest) *PluginDeliverResponse {
	// get the message
	msg, err := FromAny(request.Tx.Msg)
	if err != nil {
		return &PluginDeliverResponse{Error: err}
	}
	// handle the message
	switch x := msg.(type) {
	case *MessageSend:
		return c.DeliverMessageSend(x, request.Tx.Fee)
	case *MessageReward:
		return c.DeliverMessageReward(x, request.Tx.Fee)
	case *MessageFaucet:
		return c.DeliverMessageFaucet(x)
	default:
		return &PluginDeliverResponse{Error: ErrInvalidMessageCast()}
	}
}

// DeliverMessageFaucet handles a 'faucet' message (mints tokens to recipient - no fee, no balance check)
func (c *Contract) DeliverMessageFaucet(msg *MessageFaucet) *PluginDeliverResponse {
	log.Printf("DeliverMessageFaucet called: signer=%x recipient=%x amount=%d",
		msg.SignerAddress, msg.RecipientAddress, msg.Amount)
	recipientKey := KeyForAccount(msg.RecipientAddress)
	recipientQueryId := rand.Uint64()

	// Read current recipient state
	response, err := c.plugin.StateRead(c, &PluginStateReadRequest{
		Keys: []*PluginKeyRead{
			{QueryId: recipientQueryId, Key: recipientKey},
		},
	})
	if err != nil {
		return &PluginDeliverResponse{Error: err}
	}
	if response.Error != nil {
		return &PluginDeliverResponse{Error: response.Error}
	}

	// Get recipient bytes
	var recipientBytes []byte
	for _, resp := range response.Results {
		if resp.QueryId == recipientQueryId && len(resp.Entries) > 0 {
			recipientBytes = resp.Entries[0].Value
		}
	}

	// Unmarshal recipient account (or create new if doesn't exist)
	recipient := new(Account)
	if len(recipientBytes) > 0 {
		if err = Unmarshal(recipientBytes, recipient); err != nil {
			return &PluginDeliverResponse{Error: err}
		}
	}

	// Mint tokens to recipient
	recipient.Amount += msg.Amount

	// Marshal updated state
	recipientBytes, err = Marshal(recipient)
	if err != nil {
		return &PluginDeliverResponse{Error: err}
	}

	// Write state changes
	resp, err := c.plugin.StateWrite(c, &PluginStateWriteRequest{
		Sets: []*PluginSetOp{
			{Key: recipientKey, Value: recipientBytes},
		},
	})
	if err == nil {
		err = resp.Error
	}
	return &PluginDeliverResponse{Error: err}
}

// DeliverMessageReward handles a 'reward' message (mints tokens to recipient)
func (c *Contract) DeliverMessageReward(msg *MessageReward, fee uint64) *PluginDeliverResponse {
	var (
		adminKey, recipientKey, feePoolKey         []byte
		adminBytes, recipientBytes, feePoolBytes   []byte
		adminQueryId, recipientQueryId, feeQueryId = rand.Uint64(), rand.Uint64(), rand.Uint64()
		admin, recipient, feePool                  = new(Account), new(Account), new(Pool)
	)

	// Calculate state keys
	adminKey = KeyForAccount(msg.AdminAddress)
	recipientKey = KeyForAccount(msg.RecipientAddress)
	feePoolKey = KeyForFeePool(c.Config.ChainId)

	// Read current state
	response, err := c.plugin.StateRead(c, &PluginStateReadRequest{
		Keys: []*PluginKeyRead{
			{QueryId: feeQueryId, Key: feePoolKey},
			{QueryId: adminQueryId, Key: adminKey},
			{QueryId: recipientQueryId, Key: recipientKey},
		},
	})
	if err != nil {
		return &PluginDeliverResponse{Error: err}
	}
	if response.Error != nil {
		return &PluginDeliverResponse{Error: response.Error}
	}

	// Parse results by QueryId
	for _, resp := range response.Results {
		switch resp.QueryId {
		case adminQueryId:
			adminBytes = resp.Entries[0].Value
		case recipientQueryId:
			recipientBytes = resp.Entries[0].Value
		case feeQueryId:
			feePoolBytes = resp.Entries[0].Value
		}
	}

	// Unmarshal accounts
	if err = Unmarshal(adminBytes, admin); err != nil {
		return &PluginDeliverResponse{Error: err}
	}
	if err = Unmarshal(recipientBytes, recipient); err != nil {
		return &PluginDeliverResponse{Error: err}
	}
	if err = Unmarshal(feePoolBytes, feePool); err != nil {
		return &PluginDeliverResponse{Error: err}
	}

	// Admin must have enough to pay the fee
	if admin.Amount < fee {
		return &PluginDeliverResponse{Error: ErrInsufficientFunds()}
	}

	// Apply state changes
	admin.Amount -= fee            // Admin pays fee
	recipient.Amount += msg.Amount // Mint tokens to recipient
	feePool.Amount += fee

	// Marshal updated state
	adminBytes, err = Marshal(admin)
	if err != nil {
		return &PluginDeliverResponse{Error: err}
	}
	recipientBytes, err = Marshal(recipient)
	if err != nil {
		return &PluginDeliverResponse{Error: err}
	}
	feePoolBytes, err = Marshal(feePool)
	if err != nil {
		return &PluginDeliverResponse{Error: err}
	}

	// Write state changes
	var resp *PluginStateWriteResponse
	if admin.Amount == 0 {
		// Delete drained admin account
		resp, err = c.plugin.StateWrite(c, &PluginStateWriteRequest{
			Sets: []*PluginSetOp{
				{Key: feePoolKey, Value: feePoolBytes},
				{Key: recipientKey, Value: recipientBytes},
			},
			Deletes: []*PluginDeleteOp{{Key: adminKey}},
		})
	} else {
		resp, err = c.plugin.StateWrite(c, &PluginStateWriteRequest{
			Sets: []*PluginSetOp{
				{Key: feePoolKey, Value: feePoolBytes},
				{Key: adminKey, Value: adminBytes},
				{Key: recipientKey, Value: recipientBytes},
			},
		})
	}
	if err == nil {
		err = resp.Error
	}
	return &PluginDeliverResponse{Error: err}
}

// EndBlock() is code that is executed at the end of 'applying' a block
func (c *Contract) EndBlock(_ *PluginEndRequest) *PluginEndResponse {
	return &PluginEndResponse{}
}

// CheckMessageSend() statelessly validates a 'send' message
func (c *Contract) CheckMessageSend(msg *MessageSend) *PluginCheckResponse {
	// check sender address
	if len(msg.FromAddress) != 20 {
		return &PluginCheckResponse{Error: ErrInvalidAddress()}
	}
	// check recipient address
	if len(msg.ToAddress) != 20 {
		return &PluginCheckResponse{Error: ErrInvalidAddress()}
	}
	// check amount
	if msg.Amount == 0 {
		return &PluginCheckResponse{Error: ErrInvalidAmount()}
	}
	// return the authorized signers
	return &PluginCheckResponse{Recipient: msg.ToAddress, AuthorizedSigners: [][]byte{msg.FromAddress}}
}

// DeliverMessageSend() handles a 'send' message
func (c *Contract) DeliverMessageSend(msg *MessageSend, fee uint64) *PluginDeliverResponse {
	log.Printf("DeliverMessageSend called: from=%x to=%x amount=%d fee=%d", msg.FromAddress, msg.ToAddress, msg.Amount, fee)
	var (
		fromKey, toKey, feePoolKey         []byte
		fromBytes, toBytes, feePoolBytes   []byte
		fromQueryId, toQueryId, feeQueryId = rand.Uint64(), rand.Uint64(), rand.Uint64()
		from, to, feePool                  = new(Account), new(Account), new(Pool)
	)
	// calculate the from key and to key
	fromKey, toKey, feePoolKey = KeyForAccount(msg.FromAddress), KeyForAccount(msg.ToAddress), KeyForFeePool(c.Config.ChainId)
	log.Printf("Keys: fromKey=%x toKey=%x feePoolKey=%x", fromKey, toKey, feePoolKey)
	// get the from and to account
	response, err := c.plugin.StateRead(c, &PluginStateReadRequest{
		Keys: []*PluginKeyRead{
			{QueryId: feeQueryId, Key: feePoolKey},
			{QueryId: fromQueryId, Key: fromKey},
			{QueryId: toQueryId, Key: toKey},
		}})
	// check for internal error
	if err != nil {
		log.Printf("StateRead error: %v", err)
		return &PluginDeliverResponse{Error: err}
	}
	// ensure no error fsm error
	if response.Error != nil {
		log.Printf("StateRead FSM error: %v", response.Error)
		return &PluginDeliverResponse{Error: response.Error}
	}
	log.Printf("StateRead returned %d results", len(response.Results))
	// get the from bytes and to bytes
	for _, resp := range response.Results {
		log.Printf("Result QueryId=%d Entries=%d", resp.QueryId, len(resp.Entries))
		if len(resp.Entries) == 0 {
			log.Printf("WARNING: No entries for QueryId=%d", resp.QueryId)
			continue
		}
		switch resp.QueryId {
		case fromQueryId:
			fromBytes = resp.Entries[0].Value
			log.Printf("fromBytes len=%d", len(fromBytes))
		case toQueryId:
			toBytes = resp.Entries[0].Value
			log.Printf("toBytes len=%d", len(toBytes))
		case feeQueryId:
			feePoolBytes = resp.Entries[0].Value
			log.Printf("feePoolBytes len=%d", len(feePoolBytes))
		}
	}
	// add fee to 'amount to deduct'
	amountToDeduct := msg.Amount + fee
	// convert the bytes to account structures
	if err = Unmarshal(fromBytes, from); err != nil {
		return &PluginDeliverResponse{Error: err}
	}
	if err = Unmarshal(toBytes, to); err != nil {
		return &PluginDeliverResponse{Error: err}
	}
	if err = Unmarshal(feePoolBytes, feePool); err != nil {
		return &PluginDeliverResponse{Error: err}
	}
	log.Printf("from.Amount=%d to.Amount=%d feePool.Amount=%d", from.Amount, to.Amount, feePool.Amount)
	// if the account amount is less than the amount to subtract; return insufficient funds
	if from.Amount < amountToDeduct {
		log.Printf("ERROR: Insufficient funds: from.Amount=%d amountToDeduct=%d", from.Amount, amountToDeduct)
		return &PluginDeliverResponse{Error: ErrInsufficientFunds()}
	}
	// for self-transfer, use same account data
	if bytes.Equal(fromKey, toKey) {
		to = from
	}
	// subtract from sender
	from.Amount -= amountToDeduct
	// add the fee to the 'fee pool'
	feePool.Amount += fee
	// add to recipient
	to.Amount += msg.Amount
	log.Printf("AFTER: from.Amount=%d to.Amount=%d feePool.Amount=%d", from.Amount, to.Amount, feePool.Amount)
	// convert the accounts to bytes
	fromBytes, err = Marshal(from)
	if err != nil {
		return &PluginDeliverResponse{Error: err}
	}
	toBytes, err = Marshal(to)
	if err != nil {
		return &PluginDeliverResponse{Error: err}
	}
	feePoolBytes, err = Marshal(feePool)
	if err != nil {
		return &PluginDeliverResponse{Error: err}
	}
	// execute writes to the database
	var resp *PluginStateWriteResponse
	// if the from account is drained - delete the from account
	if from.Amount == 0 {
		resp, err = c.plugin.StateWrite(c, &PluginStateWriteRequest{
			Sets: []*PluginSetOp{
				{Key: feePoolKey, Value: feePoolBytes},
				{Key: toKey, Value: toBytes},
			},
			Deletes: []*PluginDeleteOp{{Key: fromKey}},
		})
	} else {
		resp, err = c.plugin.StateWrite(c, &PluginStateWriteRequest{
			Sets: []*PluginSetOp{
				{Key: feePoolKey, Value: feePoolBytes},
				{Key: toKey, Value: toBytes},
				{Key: fromKey, Value: fromBytes},
			},
		})
	}
	if err != nil {
		log.Printf("StateWrite internal error: %v", err)
		return &PluginDeliverResponse{Error: err}
	}
	if resp.Error != nil {
		log.Printf("StateWrite FSM error: %v", resp.Error)
		return &PluginDeliverResponse{Error: resp.Error}
	}
	log.Printf("StateWrite SUCCESS!")
	return &PluginDeliverResponse{}
}

var (
	accountPrefix = []byte{1} // store key prefix for accounts
	poolPrefix    = []byte{2} // store key prefix for pools
	paramsPrefix  = []byte{7} // store key prefix for governance parameters
)

// KeyForAccount() returns the state database key for an account
func KeyForAccount(addr []byte) []byte {
	return JoinLenPrefix(accountPrefix, addr)
}

// KeyForFeeParams() returns the state database key for governance controlled 'fee parameters'
func KeyForFeeParams() []byte {
	return JoinLenPrefix(paramsPrefix, []byte("/f/"))
}

// KeyForFeeParams() returns the state database key for governance controlled 'fee parameters'
func KeyForFeePool(chainId uint64) []byte {
	return JoinLenPrefix(poolPrefix, formatUint64(chainId))
}

func formatUint64(u uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, u)
	return b
}
