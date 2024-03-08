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
	store.SetParent(db, root)
	return store
}

func (s *SMTWrapper) Set(key, value []byte) error { return s.smt.Update(crypto.Hash(key), value) }
func (s *SMTWrapper) Delete(key []byte) error {
	if err := s.smt.Delete(crypto.Hash(key)); err != nil && err != smt.ErrKeyNotPresent {
		return err
	}
	return nil
}

func (s *SMTWrapper) Commit() (root []byte, err error) {
	if err = s.smt.Commit(); err != nil {
		return nil, err
	}
	root = types.CopyBytes(s.smt.LastSavedRoot())
	return
}

func (s *SMTWrapper) SetParent(db *TxnWrapper, root []byte) {
	//parent := NewPrefixedMapStore(db, stateCommitmentPrefix)
	if root == nil {
		s.smt = smt.NewSMT(db, crypto.Hasher())
	} else {
		s.smt = smt.ImportSMT(db, crypto.Hasher(), root)
	}
}

func (s *SMTWrapper) GetProof(key []byte) (proof, _ []byte, err error) {
	pr, err := s.smt.Prove(crypto.Hash(key))
	if err != nil {
		return nil, nil, err
	}
	cProof, err := smt.CompactProof(pr, &s.smt.BaseSMT)
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
	if proof, err = cdc.Marshal(ptr); err != nil {
		return nil, nil, err
	}
	return
}

func (s *SMTWrapper) VerifyProof(key, value, proof []byte) bool {
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
	return smt.VerifyProof(sparseMerkleProof, s.smt.Root(), crypto.Hash(key), value, &s.smt.BaseSMT)
}

var _ smt.MapStore = &PrefixedMapStore{}

type PrefixedMapStore struct {
	db     *TxnWrapper
	prefix []byte
}

func NewPrefixedMapStore(db *TxnWrapper, prefix []byte) *PrefixedMapStore {
	return &PrefixedMapStore{db, prefix}
}

func (p *PrefixedMapStore) Get(k []byte) ([]byte, error) { return p.db.Get(append(p.prefix, k...)) }
func (p *PrefixedMapStore) Set(k []byte, v []byte) error { return p.db.Set(append(p.prefix, k...), v) }
func (p *PrefixedMapStore) Delete(k []byte) error        { return p.db.Delete(append(p.prefix, k...)) }
