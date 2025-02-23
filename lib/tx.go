package lib

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	sync "sync"
	"time"

	"github.com/canopy-network/canopy/lib/crypto"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

var (
	_ TransactionI = &Transaction{} // TransactionI interface enforcement of the Transaction struct
	_ SignatureI   = &Signature{}   // SignatureI interface enforcement of the Signature struct
	_ TxResultI    = &TxResult{}    // TxResultI interface enforcement of the TxResult struct
	_ Pageable     = new(TxResults) // Pageable interface enforcement of the TxResult struct
)

const (
	TxResultsPageName      = "tx-results-page"      // the name of a page of transactions
	PendingResultsPageName = "pending-results-page" //  the name of a page of mempool pending transactions
	FailedTxsPageName      = "failed-txs-page"      // the name of a page of failed transactions
)

// Messages must be pre-registered for Transaction JSON unmarshalling
var RegisteredMessages map[string]MessageI

func init() {
	RegisteredMessages = make(map[string]MessageI)
	RegisteredPageables[TxResultsPageName] = new(TxResults)      // preregister the page type for unmarshalling
	RegisteredPageables[PendingResultsPageName] = new(TxResults) // preregister the page type for unmarshalling
	RegisteredPageables[FailedTxsPageName] = new(FailedTxs)      // preregister the page type for unmarshalling
}

// TRANSACTION INTERFACES BELOW

// TxResultI is the model of a completed transaction object after execution
type TxResultI interface {
	proto.Message
	GetSender() []byte      // the sender of the transaction
	GetRecipient() []byte   // the receiver of the transaction (i.e. the recipient of a 'send' transaction; empty of not applicable)
	GetMessageType() string // the type of message the transaction contains
	GetHeight() uint64      // the block number the transaction is included in
	GetIndex() uint64       // the index of the transaction in the block
	GetTxHash() string      // the cryptographic hash of the transaction
	GetTx() TransactionI
}

// TransactionI is the model of a transaction object (a record of an action or event)
type TransactionI interface {
	proto.Message
	GetMsg() *anypb.Any             // message payload (send, stake, edit-stake, etc.)
	GetSig() SignatureI             // digital signature allowing public key verification
	GetTime() uint64                // a stateless - prune friendly, replay attack / hash collision defense (opposed to sequence)
	GetSignBytes() ([]byte, ErrorI) // the canonical form the bytes were signed in
	GetHash() ([]byte, ErrorI)      // the computed cryptographic hash of the transaction bytes
	GetMemo() string                // an optional 100 character descriptive string - these are often used for polling
}

// SignatureI is the model of a signature object (a signature and public key bytes pair)
type SignatureI interface {
	proto.Message
	GetPublicKey() []byte
	GetSignature() []byte
}

// MessageI is the model of a message object (send, stake, edit-stake, etc.)
type MessageI interface {
	proto.Message

	New() MessageI     // new instance of the message type
	Name() string      // name of the message
	Check() ErrorI     // stateless validation of the message
	Recipient() []byte // for transaction indexing by recipient
	json.Marshaler     // json encoding
	json.Unmarshaler   // json decoding
}

// TRANSACTION CODE BELOW

// CheckBasic() is a stateless validation function for a Transaction object
func (x *Transaction) CheckBasic() ErrorI {
	if x == nil {
		return ErrEmptyTransaction()
	}
	if x.Msg == nil {
		return ErrEmptyMessage()
	}
	if x.MessageType == "" {
		return ErrUnknownMessageName(x.MessageType)
	}
	if x.Signature == nil || x.Signature.Signature == nil || x.Signature.PublicKey == nil {
		return ErrEmptySignature()
	}
	if x.CreatedHeight == 0 {
		return ErrInvalidTxHeight()
	}
	if x.Time == 0 {
		return ErrInvalidTxTime()
	}
	if len(x.Memo) > 200 {
		return ErrInvalidMemo()
	}
	if x.NetworkId == 0 {
		return ErrNilNetworkID()
	}
	if x.ChainId == 0 {
		return ErrEmptyChainId()
	}
	return nil
}

// GetHash() returns the cryptographic hash of the Transaction
func (x *Transaction) GetHash() ([]byte, ErrorI) {
	bz, err := Marshal(x)
	if err != nil {
		return nil, err
	}
	return crypto.Hash(bz), nil
}

// GetSig() accessor for signature field (do not delete: needed to satisfy TransactionI)
func (x *Transaction) GetSig() SignatureI { return x.Signature }

// GetSignBytes() returns the canonical byte representation of the Transaction for signing and signature verification
func (x *Transaction) GetSignBytes() ([]byte, ErrorI) {
	return Marshal(&Transaction{
		MessageType:   x.MessageType,
		Msg:           x.Msg,
		Signature:     nil,
		Time:          x.Time,
		CreatedHeight: x.CreatedHeight,
		Fee:           x.Fee,
		Memo:          x.Memo,
		NetworkId:     x.NetworkId,
		ChainId:       x.ChainId,
	})
}

// Sign() executes a digital signature on the transaction
func (x *Transaction) Sign(pk crypto.PrivateKeyI) ErrorI {
	bz, err := x.GetSignBytes()
	if err != nil {
		return err
	}
	x.Signature = &Signature{
		PublicKey: pk.PublicKey().Bytes(),
		Signature: pk.Sign(bz),
	}
	return nil
}

type jsonTx struct {
	Type          string          `json:"type,omitempty"`
	Msg           json.RawMessage `json:"msg,omitempty"`
	Signature     *Signature      `json:"signature,omitempty"`
	Time          uint64          `json:"time,omitempty"`
	CreatedHeight uint64          `json:"createdHeight,omitempty"`
	Fee           uint64          `json:"fee,omitempty"`
	Memo          string          `json:"memo,omitempty"`
	NetworkId     uint64          `json:"networkID,omitempty"`
	ChainId       uint64          `json:"chainID,omitempty"`
}

// nolint:all
func (x Transaction) MarshalJSON() ([]byte, error) {
	a, err := FromAny(x.Msg)
	if err != nil {
		return nil, err
	}
	msg, ok := a.(MessageI)
	if !ok {
		return nil, fmt.Errorf("couldn't convert %T to type MessageI", a)
	}
	bz, err := MarshalJSON(msg)
	if err != nil {
		return nil, err
	}
	return json.Marshal(jsonTx{
		Type:          x.MessageType,
		Msg:           bz,
		Signature:     x.Signature,
		Time:          x.Time,
		CreatedHeight: x.CreatedHeight,
		Fee:           x.Fee,
		Memo:          x.Memo,
		NetworkId:     x.NetworkId,
		ChainId:       x.ChainId,
	})
}

func (x *Transaction) UnmarshalJSON(b []byte) error {
	var j jsonTx
	if err := json.Unmarshal(b, &j); err != nil {
		return err
	}
	var msg MessageI
	m, ok := RegisteredMessages[j.Type]
	if !ok {
		return ErrUnknownMessageName(j.Type)
	}
	msg = m.New()
	if err := json.Unmarshal(j.Msg, msg); err != nil {
		return err
	}
	a, err := NewAny(msg)
	if err != nil {
		return err
	}
	*x = Transaction{
		MessageType:   j.Type,
		Msg:           a,
		Signature:     j.Signature,
		CreatedHeight: j.CreatedHeight,
		Time:          j.Time,
		Fee:           j.Fee,
		Memo:          j.Memo,
		NetworkId:     j.NetworkId,
		ChainId:       j.ChainId,
	}
	return nil
}

// TRANSACTION RESULT CODE BELOW

type TxResults []*TxResult

func (t *TxResults) Len() int      { return len(*t) }
func (t *TxResults) New() Pageable { return &TxResults{} }

// GetTx() is an accessor for the Transaction field
func (x *TxResult) GetTx() TransactionI { return x.Transaction }

type jsonTxResult struct {
	Sender      HexBytes     `json:"sender,omitempty"`
	Recipient   HexBytes     `json:"recipient,omitempty"`
	MessageType string       `json:"message_type,omitempty"`
	Height      uint64       `json:"height,omitempty"`
	Index       uint64       `json:"index,omitempty"`
	Transaction *Transaction `json:"transaction,omitempty"`
	TxHash      string       `json:"tx_hash,omitempty"`
}

// TxResult satisfies the json.Marshaller and json.Unmarshaler interfaces

// nolint:all
func (x TxResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(jsonTxResult{
		Sender:      x.Sender,
		Recipient:   x.Recipient,
		MessageType: x.MessageType,
		Height:      x.Height,
		Index:       x.Index,
		Transaction: x.Transaction,
		TxHash:      x.TxHash,
	})
}

func (x *TxResult) UnmarshalJSON(b []byte) error {
	var j jsonTxResult
	if err := json.Unmarshal(b, &j); err != nil {
		return err
	}
	*x = TxResult{
		Sender:      j.Sender,
		Recipient:   j.Recipient,
		MessageType: j.MessageType,
		Height:      j.Height,
		Index:       j.Index,
		Transaction: j.Transaction,
		TxHash:      j.TxHash,
	}
	return nil
}

// SIGNATURE CODE BELOW

type Signable interface {
	proto.Message
	Sign(p crypto.PrivateKeyI) ErrorI
}

type SignByte interface{ SignBytes() []byte }

// Copy() returns a clone of the Signature object
func (x *Signature) Copy() *Signature {
	return &Signature{
		PublicKey: bytes.Clone(x.PublicKey),
		Signature: bytes.Clone(x.Signature),
	}
}

// Signature satisfies the json.Marshaller and json.Unmarshaler interfaces

// nolint:all
func (x Signature) MarshalJSON() ([]byte, error) {
	return json.Marshal(jsonSignature{x.PublicKey, x.Signature})
}

func (x *Signature) UnmarshalJSON(b []byte) (err error) {
	var j jsonSignature
	if err = json.Unmarshal(b, &j); err != nil {
		return err
	}
	x.PublicKey, x.Signature = j.PublicKey, j.Signature
	return
}

type jsonSignature struct {
	PublicKey HexBytes `json:"publicKey,omitempty"`
	Signature HexBytes `json:"signature,omitempty"`
}

// FAILED TX CACHE CODE BELOW

// FailedTx contains a failed transaction and its error
type FailedTx struct {
	Transaction *Transaction `json:"transaction,omitempty"`
	Hash        string       `json:"tx_hash,omitempty"`
	Address     string       `json:"address,omitempty"`
	Error       error        `json:"error,omitempty"`
}

type FailedTxs []*FailedTx

func (t *FailedTxs) Len() int      { return len(*t) }
func (t *FailedTxs) New() Pageable { return &FailedTxs{} }

type failedTx struct {
	tx        *FailedTx
	timestamp time.Time
}

// FailedTxCache is a cache of failed transactions that is used to inform
// the user of the failure
type FailedTxCache struct {
	// map tx hashes to errors
	cache map[string]failedTx
	// reject all transactions that are of these types
	disallowedMessageTypes []string
	m                      sync.Mutex
}

// NewFailedTxCache returns a new FailedTxCache
func NewFailedTxCache(disallowedMessageTypes ...string) *FailedTxCache {
	cache := &FailedTxCache{
		cache:                  map[string]failedTx{},
		m:                      sync.Mutex{},
		disallowedMessageTypes: disallowedMessageTypes,
	}
	go cache.clean()
	return cache
}

// Add adds a failed transaction with its error to the cache
func (f *FailedTxCache) Add(tx []byte, hash string, txErr error) bool {
	f.m.Lock()
	defer f.m.Unlock()

	rawTx := new(Transaction)
	if err := Unmarshal(tx, rawTx); err != nil {
		return false
	}

	if slices.Contains(f.disallowedMessageTypes, rawTx.MessageType) {
		return false
	}

	if rawTx.Signature == nil {
		return false
	}

	pubKey, err := crypto.NewPublicKeyFromBytes(rawTx.Signature.PublicKey)
	if err != nil {
		return false
	}

	f.cache[hash] = failedTx{
		tx: &FailedTx{
			Transaction: rawTx,
			Hash:        hash,
			Address:     pubKey.Address().String(),
			Error:       txErr,
		},
		timestamp: time.Now(),
	}
	return true
}

// Get returns the failed transaction associated with its hash
func (f *FailedTxCache) Get(txHash string) (*FailedTx, bool) {
	f.m.Lock()
	defer f.m.Unlock()

	failedTx, ok := f.cache[txHash]

	if !ok {
		return nil, ok
	}
	return failedTx.tx, ok
}

// GetAddr returns all the failed transactions in the cache for a given address
func (f *FailedTxCache) GetAddr(address string) []*FailedTx {
	f.m.Lock()
	defer f.m.Unlock()

	failedTxs := make([]*FailedTx, 0)
	for _, failedTx := range f.cache {
		if failedTx.tx.Address == address {
			failedTxs = append(failedTxs, failedTx.tx)
		}
	}

	return failedTxs
}

// Remove removes a transaction hash from the cache
func (f *FailedTxCache) Remove(txHashes ...string) {
	f.m.Lock()
	defer f.m.Unlock()
	for _, hash := range txHashes {
		delete(f.cache, hash)
	}
}

// clean periodically removes transactions from the cache that are older than 5 minutes
func (f *FailedTxCache) clean() {
	ticker := time.NewTicker(2 * time.Minute)
	for range ticker.C {
		f.m.Lock()
		for hash, tx := range f.cache {
			if time.Since(tx.timestamp) >= 5*time.Minute {
				delete(f.cache, hash)
			}
		}
		f.m.Unlock()
	}
}
