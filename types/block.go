package types

import (
	"bytes"
	"github.com/ginchuco/ginchu/types/crypto"
)

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
	if x.Hash == nil {
		return ErrNilBlockHash()
	}
	if x.StateRoot == nil {
		return ErrNilStateRoot()
	}
	if x.TransactionRoot == nil {
		return ErrNilTransactionRoot()
	}
	if x.ValidatorRoot == nil {
		return ErrNilValidatorRoot()
	}
	if x.NextValidatorRoot == nil {
		return ErrNilNextValidatorRoot()
	}
	if x.LastQuorumCertificate == nil {
		return ErrNilQuorumCertificate()
	}
	if x.Time.Seconds == 0 {
		return ErrNilBlockTime()
	}
	if x.LastBlockHash == nil {
		return ErrNilLastBlockHash()
	}
	if x.NetworkId == nil {
		return ErrNilNetworkID()
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
	if !bytes.Equal(x.NetworkId, b.NetworkId) {
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

func (b *BlockHeader) ValidateByzantineEvidence(app App, be *ByzantineEvidence) ErrorI {
	if be == nil {
		return nil
	}
	if b.LastDoubleSigners != nil {
		doubleSigners, err := be.DSE.GetDoubleSigners(app)
		if err != nil {
			return err
		}
		if len(doubleSigners) != len(b.LastDoubleSigners) {
			return ErrMismatchDoubleSignerCount()
		}
		for i, ds := range b.LastDoubleSigners {
			if !bytes.Equal(doubleSigners[i], ds) {
				return ErrMismatchEvidenceAndHeader()
			}
		}
	}
	if b.BadProposers != nil {
		height := app.LatestHeight() + 1
		vs, err := app.GetBeginStateValSet(height)
		if err != nil {
			return err
		}
		valSet, err := NewValidatorSet(vs)
		if err != nil {
			return err
		}
		badProposers, err := be.BPE.GetBadProposers(nil, height, valSet)
		if err != nil {
			return err
		}
		if len(badProposers) != len(b.BadProposers) {
			return ErrMismatchBadProducerCount()
		}
		for i, ds := range b.LastDoubleSigners {
			if !bytes.Equal(badProposers[i], ds) {
				return ErrMismatchEvidenceAndHeader()
			}
		}
	}
	return nil
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
