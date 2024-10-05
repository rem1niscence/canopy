package lib

import (
	"bytes"
	"encoding/json"
	"github.com/ginchuco/ginchu/lib/crypto"
	"time"
)

/*
	This file contains the Canopy implementation of a 'Block' which is used as 'Block bytes' in Committee for Canopy
	NOTE: other Committees will use other 'Block' implementations dictated by the respective plugins
*/

// BLOCK CODE BELOW

// Check() 'sanity checks' the Block structure
func (x *Block) Check(networkID, committeeID uint64) ErrorI {
	if x == nil {
		return ErrNilBlock()
	}
	return x.BlockHeader.Check(networkID, committeeID)
}

// Hash() computes, sets, and returns the BlockHash
func (x *Block) Hash() ([]byte, ErrorI) {
	return x.BlockHeader.SetHash()
}

// jsonBlock is the Block implementation of json.Marshaller and json.Unmarshaler
type jsonBlock struct {
	BlockHeader  *BlockHeader `json:"block_header,omitempty"`
	Transactions []HexBytes   `json:"transactions,omitempty"`
}

// MarshalJSON() implements the json.Marshaller interface
func (x Block) MarshalJSON() ([]byte, error) {
	var txs []HexBytes
	for _, tx := range x.Transactions {
		txs = append(txs, tx)
	}
	return json.Marshal(jsonBlock{
		BlockHeader:  x.BlockHeader,
		Transactions: txs,
	})
}

// UnmarshalJSON() implements the json.Unmarshaler interface
func (x *Block) UnmarshalJSON(b []byte) error {
	var j jsonBlock
	if err := json.Unmarshal(b, &j); err != nil {
		return err
	}
	var txs [][]byte
	for _, tx := range j.Transactions {
		txs = append(txs, tx)
	}
	x.BlockHeader, x.Transactions = j.BlockHeader, txs
	return nil
}

// BLOCK HEADER CODE BELOW

// Check() 'sanity checks' the block header
func (x *BlockHeader) Check(networkID, committeeID uint64) ErrorI {
	// rejects empty block header
	if x == nil {
		return ErrNilBlockHeader()
	}
	// check proposer address size
	if len(x.ProposerAddress) != crypto.AddressSize {
		return ErrInvalidBlockProposerAddress()
	}
	// check BlockHash size
	if len(x.Hash) != crypto.HashSize {
		return ErrNilBlockHash()
	}
	// check StateRoot hash size
	if len(x.StateRoot) != crypto.HashSize {
		return ErrNilStateRoot()
	}
	// check TransactionRoot hash size
	if len(x.TransactionRoot) != crypto.HashSize {
		return ErrNilTransactionRoot()
	}
	// check ValidatorRoot hash size
	if len(x.ValidatorRoot) != crypto.HashSize {
		return ErrNilValidatorRoot()
	}
	// check NextValidatorRoot hash size
	if len(x.NextValidatorRoot) != crypto.HashSize {
		return ErrNilNextValidatorRoot()
	}
	// check LastBlockHash hash size
	if len(x.LastBlockHash) != crypto.HashSize {
		return ErrNilLastBlockHash()
	}
	// check the LastQuorumCertificate
	// no block should be included in this, so set maxBlockSize to 0
	if err := x.LastQuorumCertificate.CheckBasic(); err != nil {
		return err
	}
	if x.LastQuorumCertificate.Header.NetworkId != networkID {
		return ErrWrongNetworkID()
	}
	if x.LastQuorumCertificate.Header.CommitteeId != committeeID {
		return ErrWrongCommitteeID()
	}
	// check for non-zero BlockTime
	if x.Time == 0 {
		return ErrNilBlockTime()
	}
	// check for non-zero NetworkID
	if x.NetworkId == 0 {
		return ErrNilNetworkID()
	}
	// check for BlockHash validity
	bz, err := Marshal(x)
	if err != nil {
		return err
	}
	if !bytes.Equal(x.Hash, crypto.Hash(bz)) {
		return ErrMismatchBlockHash()
	}
	return nil
}

// SetHash() computes and sets the BlockHash to BlockHeader.Hash
func (x *BlockHeader) SetHash() ([]byte, ErrorI) {
	x.Hash = nil
	bz, err := Marshal(x)
	if err != nil {
		return nil, err
	}
	x.Hash = crypto.Hash(bz)
	return x.Hash, nil
}

// jsonBlockHeader is the BlockHeader implementation of json.Marshaller and json.Unmarshaler
type jsonBlockHeader struct {
	Height                uint64             `json:"height,omitempty"`
	Hash                  HexBytes           `json:"hash,omitempty"`
	NetworkId             uint32             `json:"network_id,omitempty"`
	Time                  string             `json:"time,omitempty"`
	NumTxs                uint64             `json:"num_txs,omitempty"`
	TotalTxs              uint64             `json:"total_txs,omitempty"`
	TotalVdfIterations    uint64             `json:"total_vdf_iterations,omitempty"`
	LastBlockHash         HexBytes           `json:"last_block_hash,omitempty"`
	StateRoot             HexBytes           `json:"state_root,omitempty"`
	TransactionRoot       HexBytes           `json:"transaction_root,omitempty"`
	ValidatorRoot         HexBytes           `json:"validator_root,omitempty"`
	NextValidatorRoot     HexBytes           `json:"next_validator_root,omitempty"`
	ProposerAddress       HexBytes           `json:"proposer_address,omitempty"`
	VDF                   *VDF               `json:"vdf,omitempty"`
	LastQuorumCertificate *QuorumCertificate `json:"last_quorum_certificate,omitempty"`
}

// MarshalJSON() implements the json.Marshaller interface
func (x BlockHeader) MarshalJSON() ([]byte, error) {
	return json.Marshal(jsonBlockHeader{
		Height:                x.Height,
		Hash:                  x.Hash,
		NetworkId:             x.NetworkId,
		Time:                  time.UnixMicro(int64(x.Time)).Format(time.DateTime),
		NumTxs:                x.NumTxs,
		TotalTxs:              x.TotalTxs,
		LastBlockHash:         x.LastBlockHash,
		StateRoot:             x.StateRoot,
		TransactionRoot:       x.TransactionRoot,
		ValidatorRoot:         x.ValidatorRoot,
		NextValidatorRoot:     x.NextValidatorRoot,
		ProposerAddress:       x.ProposerAddress,
		LastQuorumCertificate: x.LastQuorumCertificate,
	})
}

// UnmarshalJSON() implements the json.Unmarshaler interface
func (x *BlockHeader) UnmarshalJSON(b []byte) error {
	var j jsonBlockHeader
	if err := json.Unmarshal(b, &j); err != nil {
		return err
	}
	t, err := time.Parse(time.DateTime, j.Time)
	if err != nil {
		return err
	}
	*x = BlockHeader{
		Height:                j.Height,
		Hash:                  j.Hash,
		NetworkId:             j.NetworkId,
		Time:                  uint64(t.UnixMicro()),
		NumTxs:                j.NumTxs,
		TotalTxs:              j.TotalTxs,
		LastBlockHash:         j.LastBlockHash,
		StateRoot:             j.StateRoot,
		TransactionRoot:       j.TransactionRoot,
		ValidatorRoot:         j.ValidatorRoot,
		NextValidatorRoot:     j.NextValidatorRoot,
		ProposerAddress:       j.ProposerAddress,
		LastQuorumCertificate: j.LastQuorumCertificate,
	}
	return nil
}

// BLOCK RESULTS CODE BELOW

// BlockResults is a collection of Blocks containing their TransactionResults and Meta after commitment
type BlockResults []*BlockResult

// Len() Satisfies the pageable interface
func (b *BlockResults) Len() int { return len(*b) }

// New() Satisfies the pageable interface
func (b *BlockResults) New() Pageable { return &BlockResults{} }

// ToBlock() converts the BlockResult into a Block object
func (x *BlockResult) ToBlock() (*Block, ErrorI) {
	var txs [][]byte
	for _, txResult := range x.Transactions {
		txBz, err := Marshal(txResult.Transaction)
		if err != nil {
			return nil, err
		}
		txs = append(txs, txBz)
	}
	return &Block{
		BlockHeader:  x.BlockHeader,
		Transactions: txs,
	}, nil
}

const (
	BlockResultsPageName = "block-results-page" // BlockResults as a pageable name
)

func init() {
	RegisteredPageables[BlockResultsPageName] = new(BlockResults) // register BlockResults as a pageable
}

var _ Pageable = new(BlockResults) // Pageable interface enforcement
