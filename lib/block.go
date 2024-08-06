package lib

import (
	"bytes"
	"encoding/json"
	"github.com/ginchuco/ginchu/lib/crypto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"time"
)

const (
	BlockResultsPageName = "block-results-page"
)

func init() {
	RegisteredPageables[BlockResultsPageName] = new(BlockResults)
}

var _ Pageable = new(BlockResults)

type BlockResults []*BlockResult

func (b *BlockResults) Len() int { return len(*b) }

func (b *BlockResults) New() Pageable {
	return &BlockResults{}
}

func (x *Block) Check() ErrorI {
	if x == nil {
		return ErrNilBlock()
	}
	return x.BlockHeader.Check()
}

func (x *Block) Hash() ([]byte, ErrorI) {
	return x.BlockHeader.SetHash()
}

func (x *BlockHeader) SetHash() ([]byte, ErrorI) {
	x.Hash = nil
	bz, err := Marshal(x)
	if err != nil {
		return nil, err
	}
	x.Hash = crypto.Hash(bz)
	return x.Hash, nil
}

func (x *BlockHeader) Check() ErrorI {
	if x == nil {
		return ErrNilBlockHeader()
	}
	if x.ProposerAddress == nil {
		return ErrNilBlockProposer()
	}
	if len(x.Hash) != crypto.HashSize {
		return ErrNilBlockHash()
	}
	if len(x.StateRoot) != crypto.HashSize {
		return ErrNilStateRoot()
	}
	if len(x.TransactionRoot) != crypto.HashSize {
		return ErrNilTransactionRoot()
	}
	if len(x.ValidatorRoot) != crypto.HashSize {
		return ErrNilValidatorRoot()
	}
	if len(x.NextValidatorRoot) != crypto.HashSize {
		return ErrNilNextValidatorRoot()
	}
	if len(x.LastBlockHash) != crypto.HashSize {
		return ErrNilLastBlockHash()
	}
	if x.LastQuorumCertificate == nil {
		return ErrNilQuorumCertificate()
	}
	if x.Time.Seconds == 0 {
		return ErrNilBlockTime()
	}
	if x.NetworkId == 0 {
		return ErrNilNetworkID()
	}
	return nil
}

type jsonBlockHeader struct {
	Height                uint64             `json:"height,omitempty"`
	Hash                  HexBytes           `json:"hash,omitempty"`
	NetworkId             uint32             `json:"network_id,omitempty"`
	Time                  string             `json:"time,omitempty"`
	NumTxs                uint64             `json:"num_txs,omitempty"`
	TotalTxs              uint64             `json:"total_txs,omitempty"`
	LastBlockHash         HexBytes           `json:"last_block_hash,omitempty"`
	StateRoot             HexBytes           `json:"state_root,omitempty"`
	TransactionRoot       HexBytes           `json:"transaction_root,omitempty"`
	ValidatorRoot         HexBytes           `json:"validator_root,omitempty"`
	NextValidatorRoot     HexBytes           `json:"next_validator_root,omitempty"`
	ProposerAddress       HexBytes           `json:"proposer_address,omitempty"`
	DoubleSigners         []*DoubleSigners   `json:"double_signers,omitempty"`
	BadProposers          []HexBytes         `json:"bad_proposers,omitempty"`
	LastQuorumCertificate *QuorumCertificate `json:"last_quorum_certificate,omitempty"`
}

// nolint:all
func (x BlockHeader) MarshalJSON() ([]byte, error) {
	var badProposers []HexBytes
	for _, b := range x.BadProposers {
		badProposers = append(badProposers, b)
	}
	return json.Marshal(jsonBlockHeader{
		Height:                x.Height,
		Hash:                  x.Hash,
		NetworkId:             x.NetworkId,
		Time:                  x.Time.AsTime().Format(time.DateTime),
		NumTxs:                x.NumTxs,
		TotalTxs:              x.TotalTxs,
		LastBlockHash:         x.LastBlockHash,
		StateRoot:             x.StateRoot,
		TransactionRoot:       x.TransactionRoot,
		ValidatorRoot:         x.ValidatorRoot,
		NextValidatorRoot:     x.NextValidatorRoot,
		ProposerAddress:       x.ProposerAddress,
		DoubleSigners:         x.DoubleSigners,
		BadProposers:          badProposers,
		LastQuorumCertificate: x.LastQuorumCertificate,
	})
}

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
		Time:                  timestamppb.New(t),
		NumTxs:                j.NumTxs,
		TotalTxs:              j.TotalTxs,
		LastBlockHash:         j.LastBlockHash,
		StateRoot:             j.StateRoot,
		TransactionRoot:       j.TransactionRoot,
		ValidatorRoot:         j.ValidatorRoot,
		NextValidatorRoot:     j.NextValidatorRoot,
		ProposerAddress:       j.ProposerAddress,
		DoubleSigners:         j.DoubleSigners,
		BadProposers:          x.BadProposers,
		LastQuorumCertificate: j.LastQuorumCertificate,
	}
	return nil
}

func (x *BlockHeader) Equals(b *BlockHeader) bool {
	if x == nil || b == nil {
		return false
	}
	if x.Height != b.Height {
		return false
	}
	if !bytes.Equal(x.Hash, b.Hash) {
		return false
	}
	if x.NetworkId != b.NetworkId {
		return false
	}
	if x.Time != b.Time {
		return false
	}
	if x.NumTxs != b.NumTxs {
		return false
	}
	if x.TotalTxs != b.TotalTxs {
		return false
	}
	if !bytes.Equal(x.LastBlockHash, b.LastBlockHash) {
		return false
	}
	if !bytes.Equal(x.StateRoot, b.StateRoot) {
		return false
	}
	if !bytes.Equal(x.TransactionRoot, b.TransactionRoot) {
		return false
	}
	if !bytes.Equal(x.ValidatorRoot, b.ValidatorRoot) {
		return false
	}
	if !bytes.Equal(x.NextValidatorRoot, b.NextValidatorRoot) {
		return false
	}
	if !bytes.Equal(x.ProposerAddress, b.ProposerAddress) {
		return false
	}
	for _, ds := range x.DoubleSigners {
		if len(ds.PubKey) != crypto.AddressSize {
			return false
		}
		if ds.Heights == nil || len(ds.Heights) < 1 {
			return false
		}
	}
	qc1Bz, _ := Marshal(x.LastQuorumCertificate)
	qc2Bz, _ := Marshal(b.LastQuorumCertificate)
	return bytes.Equal(qc1Bz, qc2Bz)
}

func (x *Block) Equals(b *Block) bool {
	if x == nil || b == nil {
		return false
	}
	if !x.BlockHeader.Equals(b.BlockHeader) {
		return false
	}
	numTxs := len(x.Transactions)
	if numTxs != len(b.Transactions) {
		return false
	}
	for i := 0; i < numTxs; i++ {
		if !bytes.Equal(x.Transactions[i], b.Transactions[i]) {
			return false
		}
	}
	return true
}

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

type jsonBlock struct {
	BlockHeader  *BlockHeader `json:"block_header,omitempty"`
	Transactions []HexBytes   `json:"transactions,omitempty"`
}

// nolint:all
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

func (x *DoubleSigners) AddHeight(height uint64) {
	for _, h := range x.Heights {
		if h == height {
			return
		}
	}
	x.Heights = append(x.Heights, height)
}
