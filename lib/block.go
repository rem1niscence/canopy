package lib

import (
	"bytes"
	"github.com/ginchuco/ginchu/lib/crypto"
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
	if x.NetworkId == 0 {
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
	if !NewDSE(x.Evidence).Equals(NewDSE(b.Evidence)) {
		return false
	}
	qc1Bz, _ := Marshal(x.LastQuorumCertificate)
	qc2Bz, _ := Marshal(b.LastQuorumCertificate)
	return bytes.Equal(qc1Bz, qc2Bz)
}

func (x *BlockHeader) ValidateByzantineEvidence(getValSet func(height uint64) (ValidatorSet, ErrorI),
	getEvByHeight func(height uint64) (*DoubleSigners, ErrorI), be *ByzantineEvidence, minimumEvidenceHeight uint64) ErrorI {
	if be == nil {
		return nil
	}
	if x.Evidence != nil {
		d := NewDSE(x.Evidence)
		if !d.Equals(NewDSE(be.DSE)) {
			return ErrMismatchEvidenceAndHeader()
		}
		for _, evidence := range x.Evidence {
			if err := evidence.CheckBasic(); err != nil {
				return err
			}
			height := evidence.VoteA.Header.Height
			vs, err := getValSet(height)
			if err != nil {
				return err
			}
			if err = evidence.Check(vs, minimumEvidenceHeight); err != nil {
				return err
			}
			doubleSigners := evidence.GetBadSigners(getEvByHeight, getValSet)
			if len(doubleSigners) != len(evidence.DoubleSigners) {
				return ErrMismatchDoubleSignerCount()
			}
			for i, ds := range evidence.DoubleSigners {
				if !bytes.Equal(doubleSigners[i], ds) {
					return ErrMismatchEvidenceAndHeader()
				}
			}
		}
	}
	if x.BadProposers != nil {
		bpe := NewBPE(be.BPE)
		vs, err := getValSet(x.Height)
		if err != nil {
			return err
		}
		if !bpe.IsValid(nil, x.Height, vs) {
			return ErrInvalidEvidence()
		}
		badProposers := bpe.GetBadProposers()
		if len(badProposers) != len(x.BadProposers) {
			return ErrMismatchBadProducerCount()
		}
		for i, bp := range x.BadProposers {
			if !bytes.Equal(badProposers[i], bp) {
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
