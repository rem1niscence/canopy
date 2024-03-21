package types

import "github.com/ginchuco/ginchu/crypto"

type StoreI interface {
	Copy() (StoreI, error)
	NewReadOnly(version uint64) (ReadOnlyStoreI, error)
	Commit() (root []byte, err error)
	Version() uint64
	NewTxn() StoreTxnI
	Discard()
	Reset() error
	ProvableStoreI
	RWStoreI
	IndexerI
	Close() error
}

type ReadOnlyStoreI interface {
	ProvableStoreI
	ReadableStoreI
}

type RWStoreI interface {
	ReadableStoreI
	WritableStoreI
}

type IndexerI interface {
	Index(result TransactionResultI) error
	DeleteForHeight(height uint64) error
	RIndexerI
}

type RIndexerI interface {
	GetByHash(hash []byte) (TransactionResultI, error)
	GetByHeight(height uint64, newestToOldest bool) ([]TransactionResultI, error)
	GetBySender(address crypto.AddressI, newestToOldest bool) ([]TransactionResultI, error)
	GetByRecipient(address crypto.AddressI, newestToOldest bool) ([]TransactionResultI, error)
}

type WritableStoreI interface {
	Set(key, value []byte) error
	Delete(key []byte) error
}

type StoreTxnI interface {
	WritableStoreI
	ReadableStoreI
	Write() error
	Discard()
}

type ReadableStoreI interface {
	Get(key []byte) ([]byte, error)
	Iterator(prefix []byte) (IteratorI, error)
	RevIterator(prefix []byte) (IteratorI, error)
}

type ProvableStoreI interface {
	GetProof(key []byte) (proof, value []byte, err error) // Get gets the bytes for a compact merkle proof
	VerifyProof(key, value, proof []byte) bool            // VerifyProof validates the merkle proof
}

type IteratorI interface {
	Valid() bool
	Next()
	Key() (key []byte)
	Value() (value []byte)
	Error() error
	Close()
}
