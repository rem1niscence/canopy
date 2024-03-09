package store

import (
	"fmt"
	"github.com/dgraph-io/badger/v4"
	"github.com/ginchuco/ginchu/codec"
	"github.com/ginchuco/ginchu/types"
	"math"
)

const (
	stateStorePrefix      = "s/"
	stateCommitmentPrefix = "c/"
	stateCommitIDPrefix   = "x/"
)

var (
	cdc        = codec.Protobuf{}
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
}

func NewStore(path string, log types.LoggerI) (types.StoreI, error) {
	db, err := badger.OpenManaged(badger.DefaultOptions(path).
		WithLoggingLevel(badger.ERROR))
	if err != nil {
		return nil, err
	}
	return newStore(db, log)
}

func NewStoreInMemory(log types.LoggerI) (types.StoreI, error) {
	db, err := badger.OpenManaged(badger.DefaultOptions("").
		WithInMemory(true).WithLoggingLevel(badger.ERROR))
	if err != nil {
		return nil, err
	}
	return newStore(db, log)
}

func newStore(db *badger.DB, log types.LoggerI) (*Store, error) {
	id := getLatestCommitID(db, log)
	writer := db.NewTransactionAt(uint64(id.Height), true)
	return &Store{
		version: uint64(id.Height),
		log:     log,
		db:      db,
		writer:  writer,
		ss:      NewTxnWrapper(writer, log, stateStorePrefix),
		sc:      NewSMTWrapper(NewTxnWrapper(writer, log, stateCommitmentPrefix), id.Root, log),
	}, nil
}

func (s *Store) NewReadOnly(version uint64) (types.ReadOnlyStoreI, error) {
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

func (s *Store) Get(key []byte) ([]byte, error) { return s.ss.Get(key) }
func (s *Store) Set(k, v []byte) error {
	if err := s.ss.Set(k, v); err != nil {
		return err
	}
	return s.sc.Set(k, v)
}

func (s *Store) Delete(k []byte) error {
	if err := s.ss.Delete(k); err != nil {
		return err
	}
	return s.sc.Delete(k)
}

func (s *Store) GetProof(k []byte) ([]byte, []byte, error) {
	val, err := s.ss.Get(k)
	if err != nil {
		return nil, nil, err
	}
	proof, _, err := s.sc.GetProof(k)
	return proof, val, err
}

func (s *Store) VerifyProof(k, v, p []byte) bool               { return s.sc.VerifyProof(k, v, p) }
func (s *Store) Iterator(p []byte) (types.IteratorI, error)    { return s.ss.Iterator(p) }
func (s *Store) RevIterator(p []byte) (types.IteratorI, error) { return s.ss.RevIterator(p) }
func (s *Store) Version() uint64                               { return s.version }
func (s *Store) Commit() (root []byte, err error) {
	s.version++
	root, err = s.sc.Commit()
	if err != nil {
		return nil, err
	}
	if err = s.setCommitID(s.version, root); err != nil {
		return nil, err
	}
	if err = s.writer.CommitAt(s.version, nil); err != nil {
		return nil, err
	}
	s.writer = s.db.NewTransactionAt(s.version, true)
	s.ss.parent = s.writer
	s.sc.SetParent(NewTxnWrapper(s.writer, s.log, stateCommitmentPrefix), root)
	return
}

func (s *Store) commitIDKey(version uint64) []byte {
	return []byte(fmt.Sprintf("%s%d", stateCommitIDPrefix, version))
}

func (s *Store) getCommitID(version uint64) (id CommitID, err error) {
	var bz []byte
	bz, err = NewTxnWrapper(s.writer, s.log, "").Get(s.commitIDKey(version))
	if err != nil {
		return
	}
	if err = cdc.Unmarshal(bz, &id); err != nil {
		return
	}
	return
}

func (s *Store) setCommitID(version uint64, root []byte) error {
	w := NewTxnWrapper(s.writer, s.log, "")
	value, err := cdc.Marshal(&CommitID{
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
	if err = cdc.Unmarshal(bz, id); err != nil {
		log.Fatalf("unmarshalCommitID() failed with err: %s", err.Error())
	}
	return
}

func (s *Store) Close() error {
	s.writer.Discard()
	return s.db.Close()
}
