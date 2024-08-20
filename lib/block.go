package lib

import (
	"bytes"
	"encoding/json"
	"github.com/ginchuco/ginchu/lib/crypto"
	"slices"
	"time"
)

const (
	BlockResultsPageName      = "block-results-page"
	CANOPY_COMMITTEE_ID       = 0
	CANOPY_MAINNET_NETWORK_ID = 0
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
	if x.Time == 0 {
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
	LastQuorumCertificate *QuorumCertificate `json:"last_quorum_certificate,omitempty"`
}

// nolint:all
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

func (x *Proposal) CheckBasic(committeeId, height uint64) ErrorI {
	if err := x.Meta.CheckBasic(committeeId, height); err != nil {
		return err
	}
	if err := x.RewardRecipients.CheckBasic(); err != nil {
		return err
	}
	if crypto.HashSize != len(x.BlockHash) {
		return ErrUnequalBlockHash()
	}
	return nil
}

func (x *RewardRecipients) CheckBasic() ErrorI {
	return CheckPaymentPercents(x.PaymentPercents)
}

func (x *ProposalMeta) CheckBasic(committeeId, height uint64) ErrorI {
	if x.CommitteeId != committeeId {
		return ErrWrongCommitteeID()
	}
	if x.CommitteeHeight != height {
		return ErrWrongHeight()
	}
	return nil
}

func (x *Proposal) Check(committeeId, height uint64) (block *Block, err ErrorI) {
	if err = x.CheckBasic(committeeId, height); err != nil {
		return
	}
	if x.Block != nil {
		block = new(Block)
		if err = Unmarshal(x.Block, block); err != nil {
			return
		}
		err = block.Check()
		blockHash, e := block.Hash()
		if e != nil {
			return nil, e
		}
		if bytes.Equal(x.BlockHash, blockHash) {
			return nil, ErrMismatchBlockHash()
		}
	}
	return
}

func (x *Proposal) Hash() []byte {
	bz, _ := x.SignBytes()
	return crypto.Hash(bz)
}

func (x *Proposal) SignBytes() (bz []byte, err ErrorI) {
	block := x.Block
	x.Block = nil
	bz, err = Marshal(x)
	x.Block = block
	return
}

func (x *Proposal) Equals(p *Proposal) bool {
	if x == nil && p == nil {
		return true
	}
	if x == nil || p == nil {
		return false
	}
	if !x.Meta.Equals(p.Meta) {
		return false
	}
	return x.RewardRecipients.Equals(p.RewardRecipients)
}

func (x *RewardRecipients) Equals(p *RewardRecipients) bool {
	if x == nil && p == nil {
		return true
	}
	if x == nil || p == nil {
		return false
	}
	if !slices.Equal(x.PaymentPercents, p.PaymentPercents) {
		return false
	}
	return x.NumberOfSamples == p.NumberOfSamples
}

func (x *ProposalMeta) Equals(p *ProposalMeta) bool {
	if x == nil && p == nil {
		return true
	}
	if x == nil || p == nil {
		return false
	}
	if x.ChainHeight != p.ChainHeight {
		return false
	}
	if x.CommitteeHeight != p.CommitteeHeight {
		return false
	}
	if x.CommitteeId != p.CommitteeId {
		return false
	}
	if !slices.EqualFunc(x.BadProposers, p.BadProposers, func(a []byte, b []byte) bool {
		return bytes.Equal(a, b)
	}) {
		return false
	}
	return slices.EqualFunc(x.DoubleSigners, p.DoubleSigners, func(a *DoubleSigners, b *DoubleSigners) bool {
		return a.Equals(b)
	})
}

func (x *Proposal) Combine(p *Proposal) ErrorI {
	if p == nil {
		return nil
	}
	for _, ep := range p.RewardRecipients.PaymentPercents {
		x.addPercents(ep.Address, ep.Percent)
	}
	*x = Proposal{
		Block:     p.Block,
		BlockHash: p.BlockHash,
		RewardRecipients: &RewardRecipients{
			PaymentPercents: x.RewardRecipients.PaymentPercents,
			NumberOfSamples: x.RewardRecipients.NumberOfSamples + 1,
		},
		Meta: &ProposalMeta{
			CommitteeId:     p.Meta.CommitteeId,
			CommitteeHeight: p.Meta.CommitteeHeight,
			ChainHeight:     p.Meta.ChainHeight,
		},
	}
	return nil
}

func (x *Proposal) AwardPercents(percents []*PaymentPercents) ErrorI {
	x.RewardRecipients.NumberOfSamples++
	for _, ep := range percents {
		x.addPercents(ep.Address, ep.Percent)
	}
	return nil
}

func (x *Proposal) addPercents(address []byte, basisPercents uint64) {
	for i, ep := range x.RewardRecipients.PaymentPercents {
		if bytes.Equal(address, ep.Address) {
			x.RewardRecipients.PaymentPercents[i].Percent += ep.Percent
			return
		}
	}
	x.RewardRecipients.PaymentPercents = append(x.RewardRecipients.PaymentPercents, &PaymentPercents{
		Address: address,
		Percent: basisPercents,
	})
}

func (x *Proposal) MarshalJSON() ([]byte, error) {
	var badProposers []HexBytes
	for _, b := range x.Meta.BadProposers {
		badProposers = append(badProposers, b)
	}
	return json.Marshal(jsonProposal{
		Block:            x.Block,
		BlockHash:        x.BlockHash,
		RewardRecipients: x.RewardRecipients,
		Meta:             x.Meta,
	})
}

func (x *Proposal) UnmarshalJSON(b []byte) error {
	j := new(jsonProposal)
	if err := json.Unmarshal(b, j); err != nil {
		return err
	}
	*x = Proposal{
		Block:            j.Block,
		BlockHash:        j.BlockHash,
		RewardRecipients: j.RewardRecipients,
		Meta:             j.Meta,
	}
	return nil
}

func ProposalToBlock(p *Proposal) (block *Block, err ErrorI) {
	block = new(Block)
	err = Unmarshal(p.Block, block)
	return
}

func UnmarshalProposal(proposal []byte) (p *Proposal, err ErrorI) {
	p = new(Proposal)
	err = Unmarshal(proposal, p)
	return
}

type jsonProposal struct {
	// BLOCK
	Block     []byte `protobuf:"bytes,1,opt,name=block,proto3" json:"block,omitempty"`                          // Carries the block, omitted in txn form
	BlockHash []byte `protobuf:"bytes,2,opt,name=block_hash,json=blockHash,proto3" json:"block_hash,omitempty"` // Maintain the block hash, included in the canopy proper blockchain
	// PAYMENT PERCENTS
	RewardRecipients *RewardRecipients `json:"reward_recipients,omitempty"`
	// SECURITY INFO
	Meta *ProposalMeta `json:"Meta,omitempty"`
}

type jsonRewardRecipients struct {
	PaymentPercents []*PaymentPercents `json:"payment_percents,omitempty"` // recipients of the block reward by percentage
	NumberOfSamples uint64             `json:"number_of_samples,omitempty"`
}

func (x *RewardRecipients) UnmarshalJSON(i []byte) error {
	j := new(jsonRewardRecipients)
	if err := json.Unmarshal(i, j); err != nil {
		return err
	}
	*x = RewardRecipients{
		PaymentPercents: j.PaymentPercents,
		NumberOfSamples: j.NumberOfSamples,
	}
	return nil
}

func (x *RewardRecipients) MarshalJSON() ([]byte, error) {
	return json.Marshal(jsonRewardRecipients{
		PaymentPercents: x.PaymentPercents,
		NumberOfSamples: x.NumberOfSamples,
	})
}

type jsonProposalMeta struct {
	CommitteeId     uint64           `json:"committee_id,omitempty"`
	CommitteeHeight uint64           `json:"committee_height,omitempty"` // Needed to allow integrated-chains to validate the proposal
	ChainHeight     uint64           `json:"chain_height,omitempty"`     // Needed to prevent replay attacks
	DoubleSigners   []*DoubleSigners `json:"double_signers,omitempty"`   // who did the bft decide was a double signer
	BadProposers    []HexBytes       `json:"bad_proposers,omitempty"`    // who did the bft decide was a bad proposer
}

func (x *ProposalMeta) UnmarshalJSON(i []byte) error {
	j := new(jsonProposalMeta)
	if err := json.Unmarshal(i, j); err != nil {
		return err
	}
	var badProposers [][]byte
	for _, bp := range j.BadProposers {
		badProposers = append(badProposers, bp)
	}
	*x = ProposalMeta{
		CommitteeId:     j.CommitteeId,
		CommitteeHeight: j.CommitteeHeight,
		ChainHeight:     j.ChainHeight,
		DoubleSigners:   j.DoubleSigners,
		BadProposers:    badProposers,
	}
	return nil
}

func (x *ProposalMeta) MarshalJSON() ([]byte, error) {
	var badProposers []HexBytes
	for _, bp := range x.BadProposers {
		badProposers = append(badProposers, bp)
	}
	return json.Marshal(jsonProposalMeta{
		CommitteeId:     x.CommitteeId,
		CommitteeHeight: x.CommitteeHeight,
		ChainHeight:     x.ChainHeight,
		DoubleSigners:   x.DoubleSigners,
		BadProposers:    badProposers,
	})
}

func CheckPaymentPercents(percent []*PaymentPercents) ErrorI {
	numProposalRecipients := len(percent)
	if numProposalRecipients == 0 || numProposalRecipients > 100 {
		return ErrInvalidNumOfRecipients()
	}
	totalPercent := uint64(0)
	for _, ep := range percent {
		if ep == nil {
			return ErrInvalidPercentAllocation()
		}
		if len(ep.Address) != crypto.AddressSize {
			return ErrInvalidAddress()
		}
		if ep.Percent == 0 {
			return ErrInvalidPercentAllocation()
		}
		totalPercent += ep.Percent
		if totalPercent > 100 {
			return ErrInvalidPercentAllocation()
		}
	}
	return nil
}

func (x *PaymentPercents) MarshalJSON() ([]byte, error) {
	return json.Marshal(paymentPercents{
		Address:  x.Address,
		Percents: x.Percent,
	})
}

func (x *PaymentPercents) UnmarshalJSON(b []byte) error {
	var ep paymentPercents
	if err := json.Unmarshal(b, &ep); err != nil {
		return err
	}
	x.Address, x.Percent = ep.Address, ep.Percents
	return nil
}

type paymentPercents struct {
	Address  HexBytes `json:"address"`
	Percents uint64   `json:"percents"`
}

func (x *DoubleSigners) AddHeight(height uint64) {
	for _, h := range x.Heights {
		if h == height {
			return
		}
	}
	x.Heights = append(x.Heights, height)
}

func (x *DoubleSigners) Equals(d *DoubleSigners) bool {
	if x == nil && d == nil {
		return true
	}
	if x == nil || d == nil {
		return false
	}
	if !bytes.Equal(x.PubKey, d.PubKey) {
		return false
	}
	return slices.Equal(x.Heights, d.Heights)
}
