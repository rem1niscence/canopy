package types

/*
	State Commitment uses a Sparse Merkle Tree to commit to a data and compute Merkle proofs.
	State Storage is used to directly access data.

	State Storage stores data like hash(key) -> hash(value) // hash of key is to uniformly distribute in SMT
	State Storage stores data in a relational database manner

	LevelDB with namespace 's' is used for State Storage
	LevelDB with namespace 'c' Celestia SMT is used for State Commitment

	A cache layer over both must be implemented to allow rollbacks during transaction and block validation
*/

type CommitStoreI interface {
	StoreI
	Commit() (root []byte, err error)                     // Commits the cached data to the underlying databases
	GetProof(key []byte) (proof, value []byte, err error) // Get gets the bytes for a compact merkle proof
	VerifyProof(key, value, proof []byte) bool            // VerifyProof validates the merkle proof
}

type StoreI interface {
	Get(key []byte) ([]byte, error)
	Set(key, value []byte) error
	Delete(key []byte) error
	Iterator(start, end []byte) (IteratorI, error)
	ReverseIterator(start, end []byte) (IteratorI, error)
	Close() error // Closes the database
}

type IteratorI interface {
	Valid() bool
	Next()
	Key() (key []byte)
	Value() (value []byte)
	Error() error
	Close()
}

type StoreObjectI interface {
	Key() []byte
	Value() []byte
	Bytes() []byte
	HashKey() []byte
	HashValue() []byte
}
