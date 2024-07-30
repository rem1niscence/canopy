package lib

import (
	"github.com/dgraph-io/badger/v4"
	"github.com/ginchuco/ginchu/lib/crypto"
)

type StoreI interface {
	RWStoreI
	ProveStoreI
	RWIndexerI
	NewTxn() StoreTxnI
	Root() ([]byte, ErrorI)
	DB() *badger.DB
	Version() uint64
	Copy() (StoreI, ErrorI)
	NewReadOnly(version uint64) (StoreI, ErrorI)
	Commit() (root []byte, err ErrorI)
	Discard()
	Reset()
	Close() ErrorI
}

type ReadOnlyStoreI interface {
	ProveStoreI
	RStoreI
	RIndexerI
}

type RWStoreI interface {
	RStoreI
	WStoreI
}

type RWIndexerI interface {
	WIndexerI
	RIndexerI
}

type WIndexerI interface {
	IndexQC(qc *QuorumCertificate) ErrorI
	IndexTx(result *TxResult) ErrorI
	IndexBlock(b *BlockResult) ErrorI
	DeleteTxsForHeight(height uint64) ErrorI
	DeleteBlockForHeight(height uint64) ErrorI
	DeleteQCForHeight(height uint64) ErrorI
}

type RIndexerI interface {
	GetTxByHash(hash []byte) (*TxResult, ErrorI)
	GetTxsByHeight(height uint64, newestToOldest bool, p PageParams) (*Page, ErrorI)
	GetTxsBySender(address crypto.AddressI, newestToOldest bool, p PageParams) (*Page, ErrorI)
	GetTxsByRecipient(address crypto.AddressI, newestToOldest bool, p PageParams) (*Page, ErrorI)
	GetBlockByHash(hash []byte) (*BlockResult, ErrorI)
	GetBlockByHeight(height uint64) (*BlockResult, ErrorI)
	GetBlocks(p PageParams) (*Page, ErrorI)
	GetQCByHeight(height uint64) (*QuorumCertificate, ErrorI)
}

type StoreTxnI interface {
	WStoreI
	RStoreI
	Write() ErrorI
	Discard()
}

type WStoreI interface {
	Set(key, value []byte) ErrorI
	Delete(key []byte) ErrorI
}

type RStoreI interface {
	Get(key []byte) ([]byte, ErrorI)
	Iterator(prefix []byte) (IteratorI, ErrorI)
	RevIterator(prefix []byte) (IteratorI, ErrorI)
}

type ProveStoreI interface {
	GetProof(key []byte) (proof, value []byte, err ErrorI) // Get gets the bytes for a compact merkle proof
	VerifyProof(key, value, proof []byte) bool             // VerifyProof validates the merkle proof
}

type IteratorI interface {
	Valid() bool
	Next()
	Key() (key []byte)
	Value() (value []byte)
	Close()
}
