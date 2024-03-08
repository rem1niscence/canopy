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

type MultiStoreI interface {
	NewHistorical(height uint32) (HistoricalStoreI, error)
	CommittableStoreI
	ProvableStoreI
	ReadableStoreI
	WritableStoreI
	ClosableStoreI
}

type StateStoreI interface {
	NewHistorical(height uint32) StateStoreI
	WritableStoreI
	ReadableStoreI
	CommittableStoreI
	VersionedStoreI
	ClosableStoreI
}

type StateCommitI interface {
	NewHistorical(height uint32, root []byte) StateCommitI
	WritableStoreI
	CommittableStoreI
	VersionedStoreI
	ProvableStoreI
}

type HistoricalStoreI interface {
	ProvableStoreI
	ReadableStoreI
}

type KVStoreI interface {
	ReadableStoreI
	WritableStoreI
}

type BatchStoreI interface {
	KVStoreI
	BatchableStoreI
}

type PrefixStoreI interface {
	ReadableStoreI
	WritableStoreI
	PrefixableStoreI
}

type CacheStoreI interface {
	KVStoreI
	WriteToBatch(b BatchI) error
	Write() error
}

type VersionedStoreI interface {
	GetHeight() uint32
}

type BatchableStoreI interface {
	NewBatch() BatchI
}

type BatchI interface {
	WritableStoreI
	Write() error
	ClosableStoreI
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

type ClosableStoreI interface {
	Close() error
}

type CommittableStoreI interface {
	Commit() (root []byte, err error)
}

type ProvableStoreI interface {
	GetProof(key []byte) (proof, value []byte, err error) // Get gets the bytes for a compact merkle proof
	VerifyProof(key, value, proof []byte) bool            // VerifyProof validates the merkle proof
}

type PrefixableStoreI interface {
	UpdatePrefix(prefix []byte)
}

type IteratorI interface {
	Valid() bool
	Next()
	Key() (key []byte)
	Value() (value []byte)
	Error() error
	Close()
}

func PrefixEndBytes(prefix []byte) []byte {
	if len(prefix) == 0 {
		return []byte{byte(255)}
	}
	end := make([]byte, len(prefix))
	copy(end, prefix)
	for {
		if end[len(end)-1] != byte(255) {
			end[len(end)-1]++
			break
		} else {
			end = end[:len(end)-1]
			if len(end) == 0 {
				end = nil
				break
			}
		}
	}
	return end
}
