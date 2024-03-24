package types

import "github.com/ginchuco/ginchu/crypto"

type StoreI interface {
	RWStoreI
	ProveStoreI
	RWIndexerI
	NewTxn() StoreTxnI
	Version() uint64
	Copy() (StoreI, ErrorI)
	NewReadOnly(version uint64) (ReadOnlyStoreI, ErrorI)
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
	Index(result TxResultI) ErrorI
	DeleteForHeight(height uint64) ErrorI
}

type RIndexerI interface {
	GetByHash(hash []byte) (TxResultI, ErrorI)
	GetByHeight(height uint64, newestToOldest bool) ([]TxResultI, ErrorI)
	GetBySender(address crypto.AddressI, newestToOldest bool) ([]TxResultI, ErrorI)
	GetByRecipient(address crypto.AddressI, newestToOldest bool) ([]TxResultI, ErrorI)
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
