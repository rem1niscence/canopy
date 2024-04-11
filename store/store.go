package store

import (
	"fmt"
	"github.com/dgraph-io/badger/v4"
	"github.com/ginchuco/ginchu/crypto"
	"github.com/ginchuco/ginchu/types"
	"math"
)

const (
	stateStorePrefix      = "s/"
	stateCommitmentPrefix = "c/"
	stateCommitIDPrefix   = "x/"
	transactionPrefix     = "t/"

	maxKeyBytes = 128
)

var (
	lastCIDKey = []byte("xl/")

	_ types.StoreI = &Store{}
)

type Store struct {
	version uint64
	log     types.LoggerI
	db      *badger.DB
	writer  *badger.Txn
	ss      *TxnWrapper
	sc      *SMTWrapper
	ix      *Indexer
	root    []byte
}

func NewStore(path string, log types.LoggerI) (types.StoreI, types.ErrorI) {
	db, err := badger.OpenManaged(badger.DefaultOptions(path).
		WithLoggingLevel(badger.ERROR).WithMemTableSize(1000000000))
	if err != nil {
		return nil, ErrOpenDB(err)
	}
	return newStore(db, log)
}

func NewStoreInMemory(log types.LoggerI) (types.StoreI, types.ErrorI) {
	db, err := badger.OpenManaged(badger.DefaultOptions("").
		WithInMemory(true).WithLoggingLevel(badger.ERROR))
	if err != nil {
		return nil, ErrOpenDB(err)
	}
	return newStore(db, log)
}

func newStore(db *badger.DB, log types.LoggerI) (*Store, types.ErrorI) {
	id := getLatestCommitID(db, log)
	writer := db.NewTransactionAt(id.Height, true)
	return &Store{
		version: id.Height,
		log:     log,
		db:      db,
		writer:  writer,
		ss:      NewTxnWrapper(writer, log, stateStorePrefix),
		sc:      NewSMTWrapper(NewTxnWrapper(writer, log, stateCommitmentPrefix), id.Root, log),
		ix:      &Indexer{NewTxnWrapper(writer, log, transactionPrefix)},
		root:    id.Root,
	}, nil
}

func (s *Store) NewReadOnly(version uint64) (types.RWStoreI, types.ErrorI) {
	id, err := s.getCommitID(version)
	if err != nil {
		return nil, err
	}
	reader := s.db.NewTransactionAt(version, false)
	return &Store{
		version: version,
		log:     nil,
		ss:      NewTxnWrapper(reader, s.log, stateStorePrefix),
		sc:      NewSMTWrapper(NewTxnWrapper(reader, s.log, stateCommitmentPrefix), id.Root, s.log),
	}, nil
}

func (s *Store) Copy() (types.StoreI, types.ErrorI) {
	writer := s.db.NewTransactionAt(s.version, true)
	return &Store{
		version: s.version,
		log:     s.log,
		db:      s.db,
		writer:  writer,
		ss:      NewTxnWrapper(writer, s.log, stateStorePrefix),
		sc:      NewSMTWrapper(NewTxnWrapper(writer, s.log, stateCommitmentPrefix), s.root, s.log),
		ix:      &Indexer{NewTxnWrapper(writer, s.log, transactionPrefix)},
		root:    s.root,
	}, nil
}

func (s *Store) Get(key []byte) ([]byte, types.ErrorI) { return s.ss.Get(key) }
func (s *Store) Set(k, v []byte) types.ErrorI {
	if err := s.ss.Set(k, v); err != nil {
		return err
	}
	return s.sc.Set(k, v)
}

func (s *Store) Delete(k []byte) types.ErrorI {
	if err := s.ss.Delete(k); err != nil {
		return err
	}
	return s.sc.Delete(k)
}

func (s *Store) GetProof(k []byte) ([]byte, []byte, types.ErrorI) {
	val, err := s.ss.Get(k)
	if err != nil {
		return nil, nil, err
	}
	proof, _, err := s.sc.GetProof(k)
	return proof, val, err
}

func (s *Store) VerifyProof(k, v, p []byte) bool                      { return s.sc.VerifyProof(k, v, p) }
func (s *Store) Iterator(p []byte) (types.IteratorI, types.ErrorI)    { return s.ss.Iterator(p) }
func (s *Store) RevIterator(p []byte) (types.IteratorI, types.ErrorI) { return s.ss.RevIterator(p) }
func (s *Store) Version() uint64                                      { return s.version }
func (s *Store) NewTxn() types.StoreTxnI                              { return NewTxn(s) }
func (s *Store) Root() (root []byte, err types.ErrorI)                { return s.sc.Root() }
func (s *Store) Commit() (root []byte, err types.ErrorI) {
	s.version++
	s.root, err = s.sc.Commit()
	if err != nil {
		return nil, err
	}
	if err = s.setCommitID(s.version, s.root); err != nil {
		return nil, err
	}
	if err := s.writer.CommitAt(s.version, nil); err != nil {
		return nil, ErrCommitDB(err)
	}
	s.resetWriter(s.root)
	return types.CopyBytes(s.root), nil
}

func (s *Store) Reset() {
	s.resetWriter(s.root)
}

func (s *Store) Discard() {
	s.writer.Discard()
}

func (s *Store) resetWriter(root []byte) {
	s.writer.Discard()
	s.writer = s.db.NewTransactionAt(s.version, true)
	s.ss.setDB(s.writer)
	s.sc.setDB(NewTxnWrapper(s.writer, s.log, stateCommitmentPrefix), root)
	s.ix.setDB(NewTxnWrapper(s.writer, s.log, transactionPrefix))
}

func (s *Store) commitIDKey(version uint64) []byte {
	return []byte(fmt.Sprintf("%s%d", stateCommitIDPrefix, version))
}

func (s *Store) getCommitID(version uint64) (id CommitID, err types.ErrorI) {
	var bz []byte
	bz, err = NewTxnWrapper(s.writer, s.log, "").Get(s.commitIDKey(version))
	if err != nil {
		return
	}
	if err = types.Unmarshal(bz, &id); err != nil {
		return
	}
	return
}

func (s *Store) setCommitID(version uint64, root []byte) types.ErrorI {
	w := NewTxnWrapper(s.writer, s.log, "")
	value, err := types.Marshal(&CommitID{
		Height: version,
		Root:   root,
	})
	if err != nil {
		return err
	}
	if err = w.Set(lastCIDKey, value); err != nil {
		return err
	}
	key := s.commitIDKey(version)
	if err != nil {
		return err
	}
	return w.Set(key, value)
}

func getLatestCommitID(db *badger.DB, log types.LoggerI) (id *CommitID) {
	tx := NewTxnWrapper(db.NewTransactionAt(math.MaxUint64, false), log, "")
	defer tx.Close()
	id = new(CommitID)
	bz, err := tx.Get(lastCIDKey)
	if err != nil {
		log.Fatalf("getLatestCommitID() failed with err: %s", err.Error())
	}
	if err = types.Unmarshal(bz, id); err != nil {
		log.Fatalf("unmarshalCommitID() failed with err: %s", err.Error())
	}
	return
}

func (s *Store) IndexTx(result *types.TxResult) types.ErrorI { return s.ix.IndexTx(result) }
func (s *Store) DeleteTxsForHeight(height uint64) types.ErrorI {
	return s.ix.DeleteTxsForHeight(height)
}
func (s *Store) GetTxByHash(h []byte) (*types.TxResult, types.ErrorI) { return s.ix.GetTxByHash(h) }

func (s *Store) GetTxsByHeight(height uint64, newestToOldest bool) ([]*types.TxResult, types.ErrorI) {
	return s.ix.GetTxsByHeight(height, newestToOldest)
}

func (s *Store) GetTxsBySender(address crypto.AddressI, newestToOldest bool) ([]*types.TxResult, types.ErrorI) {
	return s.ix.GetTxsBySender(address, newestToOldest)
}

func (s *Store) GetTxsByRecipient(address crypto.AddressI, newestToOldest bool) ([]*types.TxResult, types.ErrorI) {
	return s.ix.GetTxsByRecipient(address, newestToOldest)
}

func (s *Store) IndexBlock(b *types.BlockResult) types.ErrorI {
	return s.ix.IndexBlock(b)
}

func (s *Store) DeleteBlockForHeight(h uint64) types.ErrorI {
	return s.ix.DeleteBlockForHeight(h)
}

func (s *Store) GetBlockByHash(h []byte) (*types.BlockResult, types.ErrorI) {
	return s.ix.GetBlockByHash(h)
}

func (s *Store) GetBlockByHeight(h uint64) (*types.BlockResult, types.ErrorI) {
	return s.ix.GetBlockByHeight(h)
}

func (s *Store) DeleteEvidenceForHeight(h uint64) types.ErrorI {
	return s.ix.DeleteEvidenceForHeight(h)
}

func (s *Store) GetEvidenceByHash(h []byte) (*types.DoubleSignEvidence, types.ErrorI) {
	return s.ix.GetEvidenceByHash(h)
}

func (s *Store) IndexQC(qc *types.QuorumCertificate) types.ErrorI {
	return s.ix.IndexQC(qc)
}

func (s *Store) GetQCByHeight(height uint64) (*types.QuorumCertificate, types.ErrorI) {
	return s.ix.GetQCByHeight(height)
}

func (s *Store) DeleteQCForHeight(height uint64) types.ErrorI {
	return s.ix.DeleteQCForHeight(height)
}

func (s *Store) GetEvidenceByHeight(h uint64) ([]*types.DoubleSignEvidence, types.ErrorI) {
	return s.ix.GetEvidenceByHeight(h)
}

func (s *Store) Close() types.ErrorI {
	s.Discard()
	if err := s.db.Close(); err != nil {
		return ErrCloseDB(s.db.Close())
	}
	return nil
}
