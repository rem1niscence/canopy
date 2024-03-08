package types

type StoreI interface {
	NewReadOnly(version uint64) (ReadOnlyStoreI, error)
	Commit() (root []byte, err error)
	ProvableStoreI
	ReadableStoreI
	WritableStoreI
	Close() error
}

type ReadOnlyStoreI interface {
	ProvableStoreI
	ReadableStoreI
}

type KVStoreI interface {
	ReadableStoreI
	WritableStoreI
}

type WritableStoreI interface {
	Set(key, value []byte) error
	Delete(key []byte) error
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
