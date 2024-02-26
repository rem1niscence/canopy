package store

import (
	"github.com/ginchuco/ginchu/crypto"
	"github.com/ginchuco/ginchu/store/smt"
	"github.com/ginchuco/ginchu/types"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/storage"
)

var _ types.StoreI = &Store{}

type Store struct {
	log   types.LoggerI
	store *storeCache
	smt   *smt.SMT
}

func NewStore(path string, log types.LoggerI, opts *opt.Options) types.StoreI {
	db, err := leveldb.OpenFile(path, opts)
	if err != nil {
		log.Fatal(newOpenStoreError(err).Error())
	}
	return newStore(db, log)
}

func NewMemoryStore(log types.LoggerI) types.StoreI {
	db, err := leveldb.Open(storage.NewMemStorage(), nil)
	if err != nil {
		log.Fatal(newOpenStoreError(err).Error())
	}
	return newStore(db, log)
}

func newStore(db *leveldb.DB, log types.LoggerI) types.StoreI {
	levelDB := NewStoreCache(&LevelDBWrapper{db})
	return &Store{log, levelDB, smt.NewSMT(smt.NewCachedMap(levelDB, 100000), crypto.Hasher())}
}

func (s *Store) Get(key []byte) ([]byte, error)                { return s.store.Get(key) }
func (s *Store) Iterator(b, e []byte) (types.IteratorI, error) { return s.store.Iterator(b, e) }
func (s *Store) ReverseIterator(b, e []byte) (types.IteratorI, error) {
	return s.store.ReverseIterator(b, e)
}

func (s *Store) Set(key []byte, value []byte) error {
	if err := s.store.Set(key, value); err != nil {
		return err
	}
	if err := s.smt.Update(key, value); err != nil {
		return err
	}
	return nil
}

func (s *Store) Delete(key []byte) error {
	if err := s.store.Delete(key); err != nil {
		return err
	}
	if err := s.smt.Delete(key); err != nil {
		return err
	}
	return nil
}

func (s *Store) GetProof(key []byte) (compact, value []byte, err error) {
	value, err = s.store.Get(key)
	if err != nil {
		return nil, nil, err
	}
	proof, err := s.smt.Prove(key)
	if err != nil {
		return nil, nil, err
	}
	cProof, err := smt.CompactProof(proof, &s.smt.BaseSMT)
	if err != nil {
		return nil, nil, err
	}
	ptr := &SparseCompactMerkleProof{
		SideNodes:             cProof.SideNodes,
		NonMembershipLeafData: cProof.NonMembershipLeafData,
		BitMask:               cProof.BitMask,
		NumSideNodes:          uint32(cProof.NumSideNodes),
		SiblingData:           cProof.SiblingData,
	}
	if compact, err = cdc.Marshal(ptr); err != nil {
		return nil, nil, err
	}
	return
}

func (s *Store) VerifyProof(key, value, proof []byte) bool {
	ptr := &SparseCompactMerkleProof{}
	if err := cdc.Unmarshal(proof, ptr); err != nil {
		s.log.Error(newUnmarshalCompactProofError(err).Error())
		return false
	}
	sparseMerkleProof, err := smt.DecompactProof(smt.SparseCompactMerkleProof{
		SideNodes:             ptr.SideNodes,
		NonMembershipLeafData: ptr.NonMembershipLeafData,
		BitMask:               ptr.BitMask,
		NumSideNodes:          int(ptr.NumSideNodes),
		SiblingData:           ptr.SiblingData,
	}, &s.smt.BaseSMT)
	if err != nil {
		s.log.Error(newDecompactProofError(err).Error())
		return false
	}
	return smt.VerifyProof(sparseMerkleProof, s.smt.Root(), key, value, &s.smt.BaseSMT)
}

func (s *Store) Commit() (root []byte, err error) {
	// TODO make atomic
	if err = s.smt.Commit(); err != nil {
		return nil, err
	}
	s.store.Write()
	// return copy of root
	rootCpy := make([]byte, len(s.smt.SavedRoot))
	copy(rootCpy, s.smt.SavedRoot)
	return rootCpy, err
}

func (s *Store) Close() error { return s.store.parent.Close() }

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
