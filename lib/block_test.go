package lib

import (
	"bytes"
	"encoding/json"
	"github.com/canopy-network/canopy/lib/crypto"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestCheckBlockHeader(t *testing.T) {
	// predefine a block in order to make a valid hash
	validBlock := &BlockHeader{
		ProposerAddress:   newTestAddressBytes(t),
		StateRoot:         crypto.ShortHash([]byte("hash")),
		TransactionRoot:   crypto.Hash([]byte("hash")),
		ValidatorRoot:     crypto.Hash([]byte("hash")),
		NextValidatorRoot: crypto.Hash([]byte("hash")),
		LastBlockHash:     crypto.Hash([]byte("hash")),
		LastQuorumCertificate: &QuorumCertificate{
			Header: &View{
				NetworkId: 1,
			},
			ResultsHash: crypto.Hash([]byte("hash")),
			BlockHash:   crypto.Hash([]byte("hash")),
			ProposerKey: newTestAddressBytes(t),
			Signature: &AggregateSignature{
				Signature: bytes.Repeat([]byte("F"), 96),
				Bitmap:    []byte("some_bitmap"),
			},
		},
		NetworkId: 1,
		Time:      uint64(time.Now().UnixMicro()),
	}
	// set  the block hash
	_, e := validBlock.SetHash()
	require.NoError(t, e)
	tests := []struct {
		name        string
		detail      string
		blockHeader *BlockHeader
		networkId   uint64
		committeeId uint64
		error       string
	}{
		{
			name:        "nil block header",
			detail:      "the block header is nil",
			blockHeader: nil,
			error:       "block.header is nil",
		},
		{
			name:        "wrong proposer address size",
			detail:      "the proposer address should be of length 'address size'",
			blockHeader: &BlockHeader{ProposerAddress: []byte("wrong_size")},
			error:       "block proposer address is invalid",
		},
		{
			name:   "wrong block hash size",
			detail: "the block should be of length 'hash size'",
			blockHeader: &BlockHeader{
				ProposerAddress: newTestAddressBytes(t),
				Hash:            []byte("wrong_size"),
			},
			error: "wrong length block hash",
		},
		{
			name:   "wrong state root size",
			detail: "the state root should be of length 'hash size'",
			blockHeader: &BlockHeader{
				ProposerAddress: newTestAddressBytes(t),
				Hash:            crypto.Hash([]byte("hash")),
				StateRoot:       []byte("wrong_size"),
			},
			error: "wrong length state root",
		},
		{
			name:   "wrong transaction root size",
			detail: "the transaction root should be of length 'hash size'",
			blockHeader: &BlockHeader{
				ProposerAddress: newTestAddressBytes(t),
				Hash:            crypto.Hash([]byte("hash")),
				StateRoot:       crypto.ShortHash([]byte("hash")),
				TransactionRoot: []byte("wrong_size"),
			},
			error: "wrong length transaction root",
		},
		{
			name:   "wrong validator root size",
			detail: "the validator root should be of length 'hash size'",
			blockHeader: &BlockHeader{
				ProposerAddress: newTestAddressBytes(t),
				Hash:            crypto.Hash([]byte("hash")),
				StateRoot:       crypto.ShortHash([]byte("hash")),
				TransactionRoot: crypto.Hash([]byte("hash")),
				ValidatorRoot:   []byte("wrong_size"),
			},
			error: "wrong length validator root",
		},
		{
			name:   "wrong next validator root size",
			detail: "the next validator root should be of length 'hash size'",
			blockHeader: &BlockHeader{
				ProposerAddress:   newTestAddressBytes(t),
				Hash:              crypto.Hash([]byte("hash")),
				StateRoot:         crypto.ShortHash([]byte("hash")),
				TransactionRoot:   crypto.Hash([]byte("hash")),
				ValidatorRoot:     crypto.Hash([]byte("hash")),
				NextValidatorRoot: []byte("wrong_size"),
			},
			error: "wrong length next validator root",
		},
		{
			name:   "wrong last block hash size",
			detail: "the next last block hash should be of length 'hash size'",
			blockHeader: &BlockHeader{
				ProposerAddress:   newTestAddressBytes(t),
				Hash:              crypto.Hash([]byte("hash")),
				StateRoot:         crypto.ShortHash([]byte("hash")),
				TransactionRoot:   crypto.Hash([]byte("hash")),
				ValidatorRoot:     crypto.Hash([]byte("hash")),
				NextValidatorRoot: crypto.Hash([]byte("hash")),
				LastBlockHash:     []byte("wrong_size"),
			},
			error: "wrong length last block hash",
		},
		{
			name:   "wrong last block hash size",
			detail: "the next last block hash should be of length 'hash size'",
			blockHeader: &BlockHeader{
				ProposerAddress:   newTestAddressBytes(t),
				Hash:              crypto.Hash([]byte("hash")),
				StateRoot:         crypto.ShortHash([]byte("hash")),
				TransactionRoot:   crypto.Hash([]byte("hash")),
				ValidatorRoot:     crypto.Hash([]byte("hash")),
				NextValidatorRoot: crypto.Hash([]byte("hash")),
				LastBlockHash:     []byte("wrong_size"),
			},
			error: "wrong length last block hash",
		},
		{
			name:   "empty last quorum certificate",
			detail: "the last quorum certificate is nil or empty",
			blockHeader: &BlockHeader{
				Height:            2,
				ProposerAddress:   newTestAddressBytes(t),
				Hash:              crypto.Hash([]byte("hash")),
				StateRoot:         crypto.ShortHash([]byte("hash")),
				TransactionRoot:   crypto.Hash([]byte("hash")),
				ValidatorRoot:     crypto.Hash([]byte("hash")),
				NextValidatorRoot: crypto.Hash([]byte("hash")),
				LastBlockHash:     crypto.Hash([]byte("hash")),
			},
			error: "empty quorum certificate",
		},
		{
			name:      "mismatch network id",
			detail:    "the network id != blockHeader.network_id",
			networkId: 1,
			blockHeader: &BlockHeader{
				ProposerAddress:   newTestAddressBytes(t),
				Hash:              crypto.Hash([]byte("hash")),
				StateRoot:         crypto.ShortHash([]byte("hash")),
				TransactionRoot:   crypto.Hash([]byte("hash")),
				ValidatorRoot:     crypto.Hash([]byte("hash")),
				NextValidatorRoot: crypto.Hash([]byte("hash")),
				LastBlockHash:     crypto.Hash([]byte("hash")),
				LastQuorumCertificate: &QuorumCertificate{
					Header:      &View{},
					ResultsHash: crypto.Hash([]byte("hash")),
					BlockHash:   crypto.Hash([]byte("hash")),
					ProposerKey: newTestAddressBytes(t),
					Signature: &AggregateSignature{
						Signature: bytes.Repeat([]byte("F"), 96),
						Bitmap:    []byte("some_bitmap"),
					},
				},
			},
			error: "wrong network id",
		},
		{
			name:        "mismatch committee id",
			detail:      "the committee id != blockHeader.network_id",
			networkId:   1,
			committeeId: 1,
			blockHeader: &BlockHeader{
				Height:            2,
				ProposerAddress:   newTestAddressBytes(t),
				Hash:              crypto.Hash([]byte("hash")),
				StateRoot:         crypto.ShortHash([]byte("hash")),
				TransactionRoot:   crypto.Hash([]byte("hash")),
				ValidatorRoot:     crypto.Hash([]byte("hash")),
				NextValidatorRoot: crypto.Hash([]byte("hash")),
				LastBlockHash:     crypto.Hash([]byte("hash")),
				LastQuorumCertificate: &QuorumCertificate{
					Header:      &View{},
					ResultsHash: crypto.Hash([]byte("hash")),
					BlockHash:   crypto.Hash([]byte("hash")),
					ProposerKey: newTestAddressBytes(t),
					Signature: &AggregateSignature{
						Signature: bytes.Repeat([]byte("F"), 96),
						Bitmap:    []byte("some_bitmap"),
					},
				},
				NetworkId: 1,
			},
			error: "wrong network id",
		},
		{
			name:   "empty block time",
			detail: "the block time is nil or empty",
			blockHeader: &BlockHeader{
				ProposerAddress:   newTestAddressBytes(t),
				Hash:              crypto.Hash([]byte("hash")),
				StateRoot:         crypto.ShortHash([]byte("hash")),
				TransactionRoot:   crypto.Hash([]byte("hash")),
				ValidatorRoot:     crypto.Hash([]byte("hash")),
				NextValidatorRoot: crypto.Hash([]byte("hash")),
				LastBlockHash:     crypto.Hash([]byte("hash")),
				LastQuorumCertificate: &QuorumCertificate{
					Header:      &View{},
					ResultsHash: crypto.Hash([]byte("hash")),
					BlockHash:   crypto.Hash([]byte("hash")),
					ProposerKey: newTestAddressBytes(t),
					Signature: &AggregateSignature{
						Signature: bytes.Repeat([]byte("F"), 96),
						Bitmap:    []byte("some_bitmap"),
					},
				},
			},
			error: "nil block time",
		},
		{
			name:        "valid block",
			detail:      "the block header is valid",
			networkId:   1,
			blockHeader: validBlock,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// execute the function call
			err := test.blockHeader.Check(test.networkId, test.committeeId)
			// validate if an error is expected
			require.Equal(t, err != nil, test.error != "")
			// validate actual error if any
			if err != nil {
				require.ErrorContains(t, err, test.error)
			}
		})
	}
}

func TestSetHash(t *testing.T) {
	// predefine a block in order to make a valid hash
	validBlock := &BlockHeader{
		ProposerAddress:   newTestAddressBytes(t),
		StateRoot:         crypto.Hash([]byte("hash")),
		TransactionRoot:   crypto.Hash([]byte("hash")),
		ValidatorRoot:     crypto.Hash([]byte("hash")),
		NextValidatorRoot: crypto.Hash([]byte("hash")),
		LastBlockHash:     crypto.Hash([]byte("hash")),
		LastQuorumCertificate: &QuorumCertificate{
			Header: &View{
				NetworkId: 1,
			},
			ResultsHash: crypto.Hash([]byte("hash")),
			BlockHash:   crypto.Hash([]byte("hash")),
			ProposerKey: newTestAddressBytes(t),
			Signature: &AggregateSignature{
				Signature: bytes.Repeat([]byte("F"), 96),
				Bitmap:    []byte("some_bitmap"),
			},
		},
		NetworkId: 1,
		Time:      uint64(time.Now().UnixMicro()),
	}
	// set the hash
	gotHash, err := validBlock.SetHash()
	// validate no error
	require.NoError(t, err)
	// validate hash was set
	require.Equal(t, gotHash, validBlock.Hash)
	// manually recompute hash
	validBlock.Hash = nil
	// get block bytes
	bz, err := Marshal(validBlock)
	// ensure no error
	require.NoError(t, err)
	// reset the hash
	validBlock.Hash = crypto.Hash(bz)
	// check got vs expected
	require.Equal(t, gotHash, validBlock.Hash)
}

func TestBlockHeaderJSON(t *testing.T) {
	// predefine a valid block header
	validBlock := &BlockHeader{
		ProposerAddress:   newTestAddressBytes(t),
		StateRoot:         crypto.Hash([]byte("hash")),
		TransactionRoot:   crypto.Hash([]byte("hash")),
		ValidatorRoot:     crypto.Hash([]byte("hash")),
		NextValidatorRoot: crypto.Hash([]byte("hash")),
		LastBlockHash:     crypto.Hash([]byte("hash")),
		LastQuorumCertificate: &QuorumCertificate{
			Header: &View{
				NetworkId: 1,
			},
			ResultsHash: crypto.Hash([]byte("hash")),
			BlockHash:   crypto.Hash([]byte("hash")),
			ProposerKey: newTestAddressBytes(t),
			Signature: &AggregateSignature{
				Signature: bytes.Repeat([]byte("F"), 96),
				Bitmap:    []byte("some_bitmap"),
			},
		},
		NetworkId: 1,
		Time:      uint64(1731418066000000),
	}
	// set the hash
	_, err := validBlock.SetHash()
	require.NoError(t, err)
	// convert to json bytes
	jsonBytes, e := json.Marshal(validBlock)
	require.NoError(t, e)
	// define a new bh object
	blockHeader := new(BlockHeader)
	// unmarshal bytes into the block header object
	require.NoError(t, json.Unmarshal(jsonBytes, blockHeader))
	// hardcode time as we'll lose precision
	blockHeader.Time = validBlock.Time
	// ensure the new object is equal to the old
	require.EqualExportedValues(t, validBlock, blockHeader)
}

func TestCheckBlock(t *testing.T) {
	// predefine a valid block header
	validBlock := Block{BlockHeader: &BlockHeader{
		ProposerAddress:   newTestAddressBytes(t),
		StateRoot:         crypto.ShortHash([]byte("hash")),
		TransactionRoot:   crypto.Hash([]byte("hash")),
		ValidatorRoot:     crypto.Hash([]byte("hash")),
		NextValidatorRoot: crypto.Hash([]byte("hash")),
		LastBlockHash:     crypto.Hash([]byte("hash")),
		LastQuorumCertificate: &QuorumCertificate{
			Header: &View{
				NetworkId: 1,
			},
			ResultsHash: crypto.Hash([]byte("hash")),
			BlockHash:   crypto.Hash([]byte("hash")),
			ProposerKey: newTestAddressBytes(t),
			Signature: &AggregateSignature{
				Signature: bytes.Repeat([]byte("F"), 96),
				Bitmap:    []byte("some_bitmap"),
			},
		},
		NetworkId: 1,
		Time:      uint64(1731418066000000),
	}}
	// set the block hash
	_, e := validBlock.Hash()
	require.NoError(t, e)
	// define test cases
	tests := []struct {
		name   string
		detail string
		block  *Block
		error  string
	}{
		{
			name:   "nil block",
			detail: "nil / empty block",
			block:  nil,
			error:  "block is nil",
		},
		{
			name:   "nil block header",
			detail: "nil / empty block header",
			block: &Block{
				BlockHeader: nil,
			},
			error: "block.header is nil",
		},
		{
			name:   "valid block",
			detail: "testing the happy path 'valid block'",
			block:  &validBlock,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// execute the function call
			err := test.block.Check(uint64(validBlock.BlockHeader.NetworkId), 0)
			// validate if an error is expected
			require.Equal(t, err != nil, test.error != "", err)
			// validate actual error if any
			if err != nil {
				require.ErrorContains(t, err, test.error)
			}
		})
	}
}

func TestBlockJSON(t *testing.T) {
	// predefine a valid block header
	validBlock := &Block{BlockHeader: &BlockHeader{
		ProposerAddress:   newTestAddressBytes(t),
		StateRoot:         crypto.Hash([]byte("hash")),
		TransactionRoot:   crypto.Hash([]byte("hash")),
		ValidatorRoot:     crypto.Hash([]byte("hash")),
		NextValidatorRoot: crypto.Hash([]byte("hash")),
		LastBlockHash:     crypto.Hash([]byte("hash")),
		LastQuorumCertificate: &QuorumCertificate{
			Header: &View{
				NetworkId: 1,
			},
			ResultsHash: crypto.Hash([]byte("hash")),
			BlockHash:   crypto.Hash([]byte("hash")),
			ProposerKey: newTestAddressBytes(t),
			Signature: &AggregateSignature{
				Signature: bytes.Repeat([]byte("F"), 96),
				Bitmap:    []byte("some_bitmap"),
			},
		},
		NetworkId: 1,
		Time:      uint64(1731418066000000),
	}, Transactions: [][]byte{[]byte("abcdef")}}
	// set the hash
	_, err := validBlock.Hash()
	require.NoError(t, err)
	// convert to json bytes
	jsonBytes, e := json.Marshal(validBlock)
	require.NoError(t, e)
	// define a new bh object
	block := new(Block)
	// unmarshal bytes into the block header object
	require.NoError(t, json.Unmarshal(jsonBytes, block))
	// hardcode time as we'll lose precision
	block.BlockHeader.Time = validBlock.BlockHeader.Time
	// ensure the new object is equal to the old
	require.EqualExportedValues(t, validBlock, block)
}

func newTestAddress(t *testing.T, variation ...int) crypto.AddressI {
	kg := newTestKeyGroup(t, variation...)
	return kg.Address
}

func newTestAddressBytes(t *testing.T, variation ...int) []byte {
	return newTestAddress(t, variation...).Bytes()
}

func newTestPublicKey(t *testing.T, variation ...int) crypto.PublicKeyI {
	kg := newTestKeyGroup(t, variation...)
	return kg.PublicKey
}

func newTestPublicKeyBytes(t *testing.T, variation ...int) []byte {
	return newTestPublicKey(t, variation...).Bytes()
}

func newTestKeyGroup(t *testing.T, variation ...int) *crypto.KeyGroup {
	var (
		key  crypto.PrivateKeyI
		err  error
		keys = []string{
			"01553a101301cd7019b78ffa1186842dd93923e563b8ae22e2ab33ae889b23ee",
			"1b6b244fbdf614acb5f0d00a2b56ffcbe2aa23dabd66365dffcd3f06491ae50a",
			"2ee868f74134032eacba191ca529115c64aa849ac121b75ca79b37420a623036",
			"3e3ab94c10159d63a12cb26aca4b0e76070a987d49dd10fc5f526031e05801da",
			"479839d3edbd0eefa60111db569ded6a1a642cc84781600f0594bd8d4a429319",
			"51eb5eb6eca0b47c8383652a6043aadc66ddbcbe240474d152f4d9a7439eae42",
			"637cb8e916bba4c1773ed34d89ebc4cb86e85c145aea5653a58de930590a2aa4",
			"7235e5757e6f52e6ae4f9e20726d9c514281e58e839e33a7f667167c524ff658"}
	)

	if len(variation) == 1 {
		key, err = crypto.StringToBLS12381PrivateKey(keys[variation[0]])
	} else {
		key, err = crypto.StringToBLS12381PrivateKey(keys[0])
	}
	require.NoError(t, err)
	return crypto.NewKeyGroup(key)
}
