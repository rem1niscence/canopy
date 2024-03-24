package store

import (
	"github.com/ginchuco/ginchu/crypto"
	"github.com/ginchuco/ginchu/store/smt"
	"github.com/ginchuco/ginchu/types"
)

type SMTWrapper struct {
	smt *smt.SMT
	log types.LoggerI
}

func NewSMTWrapper(db *TxnWrapper, root []byte, log types.LoggerI) *SMTWrapper {
	store := &SMTWrapper{
		log: log,
	}
	store.setDB(db, root)
	return store
}

func (s *SMTWrapper) Set(k, v []byte) types.ErrorI {
	if err := s.smt.Update(crypto.Hash(k), v); err != nil {
		return ErrStoreSet(err)
	}
	return nil
}

func (s *SMTWrapper) Delete(key []byte) types.ErrorI {
	if err := s.smt.Delete(crypto.Hash(key)); err != nil && err != smt.ErrKeyNotPresent {
		return ErrStoreDelete(err)
	}
	return nil
}

func (s *SMTWrapper) Commit() ([]byte, types.ErrorI) {
	if err := s.smt.Commit(); err != nil {
		return nil, ErrCommitTree(err)
	}
	return types.CopyBytes(s.smt.LastSavedRoot()), nil
}

func (s *SMTWrapper) setDB(db *TxnWrapper, root []byte) {
	if root == nil {
		s.smt = smt.NewSMT(NewMapStore(db), crypto.Hasher())
	} else {
		s.smt = smt.ImportSMT(NewMapStore(db), crypto.Hasher(), root)
	}
}

func (s *SMTWrapper) GetProof(key []byte) ([]byte, []byte, types.ErrorI) {
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
	proof, er := types.Marshal(ptr)
	if er != nil {
		return nil, nil, er
	}
	return proof, nil, nil
}

func (s *SMTWrapper) VerifyProof(key, value, proof []byte) bool {
	ptr := &SparseCompactMerkleProof{}
	if err := types.Unmarshal(proof, ptr); err != nil {
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
