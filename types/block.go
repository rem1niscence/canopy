package types

import "github.com/ginchuco/ginchu/crypto"

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
	if x.QuorumCertificate == nil {
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
