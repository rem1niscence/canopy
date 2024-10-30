package store

import (
	"bytes"
	"github.com/ginchuco/canopy/lib"
	"github.com/ginchuco/canopy/lib/crypto"
	"github.com/ginchuco/canopy/store/smt"
)

// SMTWrapper wraps the Sparse Merkle Tree implementation
type SMTWrapper struct {
	smt *smt.SMT
	log lib.LoggerI
}

// NewSMTWrapper() creates a new instance of the wrapper
func NewSMTWrapper(db *TxnWrapper, root []byte, log lib.LoggerI) *SMTWrapper {
	store := &SMTWrapper{
		log: log,
	}
	store.setDB(db, root)
	return store
}

// Set() updates the underlying sparse merkle tree, using the hash of the input key as the key
func (s *SMTWrapper) Set(k, v []byte) lib.ErrorI {
	if err := s.smt.Update(crypto.Hash(k), v); err != nil {
		return ErrStoreSet(err)
	}
	return nil
}

// Delete() removes deletes from the underlying sparse merkle tree, gracefully handling the KeyNotPresent error
func (s *SMTWrapper) Delete(key []byte) lib.ErrorI {
	if err := s.smt.Delete(crypto.Hash(key)); err != nil && err != smt.ErrKeyNotPresent {
		return ErrStoreDelete(err)
	}
	return nil
}

// Commit() saves the version of to the immutable tree and returns the root of the tree
func (s *SMTWrapper) Commit() ([]byte, lib.ErrorI) {
	if err := s.smt.Commit(); err != nil {
		return nil, ErrCommitTree(err)
	}
	return bytes.Clone(s.smt.LastSavedRoot()), nil
}

// Root() returns a copy of the root hash of the SMT
func (s *SMTWrapper) Root() ([]byte, lib.ErrorI) {
	return bytes.Clone(s.smt.Root()), nil
}

// setDB() initializes the SMT with a new or existing root based on the provided database
func (s *SMTWrapper) setDB(db *TxnWrapper, root []byte) {
	if root == nil {
		s.smt = smt.NewSMT(NewMapStore(db), crypto.Hasher())
	} else {
		s.smt = smt.ImportSMT(NewMapStore(db), crypto.Hasher(), root)
	}
}

// GetProof() generates a proof for the given key using the sparse Merkle tree
func (s *SMTWrapper) GetProof(key []byte) ([]byte, []byte, lib.ErrorI) {
	pr, err := s.smt.Prove(crypto.Hash(key))
	if err != nil {
		return nil, nil, ErrProve(err)
	}
	cProof, err := smt.CompactProof(pr, &s.smt.BaseSMT)
	if err != nil {
		return nil, nil, ErrCompactProof(err)
	}
	ptr := &SparseCompactMerkleProof{
		SideNodes:             cProof.SideNodes,
		NonMembershipLeafData: cProof.NonMembershipLeafData,
		BitMask:               cProof.BitMask,
		NumSideNodes:          uint32(cProof.NumSideNodes),
		SiblingData:           cProof.SiblingData,
	}
	proof, er := lib.Marshal(ptr)
	if er != nil {
		return nil, nil, er
	}
	return proof, nil, nil
}

// VerifyProof() verifies the validity of the proof for a given key-value pair
func (s *SMTWrapper) VerifyProof(key, value, proof []byte) bool {
	ptr := &SparseCompactMerkleProof{}
	if err := lib.Unmarshal(proof, ptr); err != nil {
		s.log.Error(err.Error())
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
		s.log.Error(ErrDecompactProof(err).Error())
		return false
	}
	return smt.VerifyProof(sparseMerkleProof, s.smt.Root(), crypto.Hash(key), value, &s.smt.BaseSMT)
}

// MapStore is an interface requirement of the Celestia SMT package

var _ smt.MapStore = &MapStore{}

type MapStore struct {
	db *TxnWrapper
}

func NewMapStore(db *TxnWrapper) *MapStore {
	return &MapStore{db}
}

func (p *MapStore) Get(k []byte) ([]byte, error) { return p.db.Get(k) }
func (p *MapStore) Set(k []byte, v []byte) error { return p.db.Set(k, v) }
func (p *MapStore) Delete(k []byte) error        { return p.db.Delete(k) }
