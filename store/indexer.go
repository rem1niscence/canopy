package store

import (
	"encoding/binary"
	"encoding/hex"
	"github.com/ginchuco/ginchu/crypto"
	"github.com/ginchuco/ginchu/types"
)

var _ types.IndexerI = &Indexer{}

var (
	hashPrefix      = []byte{1}
	heightPrefix    = []byte{2}
	senderPrefix    = []byte{3}
	recipientPrefix = []byte{4}
)

type Indexer struct {
	db *TxnWrapper
}

func (t *Indexer) Index(result types.TransactionResultI) error {
	bz, err := result.GetBytes()
	if err != nil {
		return err
	}
	hash, err := hex.DecodeString(result.GetTxHash())
	if err != nil {
		return err
	}
	hashKey, err := t.indexByHash(hash, bz)
	if err != nil {
		return err
	}
	heightAndIndexKey := t.heightAndIndexKey(result.GetHeight(), result.GetIndex())
	if err = t.indexByHeightAndIndex(heightAndIndexKey, hashKey); err != nil {
		return err
	}
	if err = t.indexBySender(result.GetSender(), heightAndIndexKey, hashKey); err != nil {
		return err
	}
	return t.indexByRecipient(result.GetRecipient(), heightAndIndexKey, hashKey)
}

func (t *Indexer) GetByHash(hash []byte) (types.TransactionResultI, error) {
	return t.get(t.hashKey(hash))
}

func (t *Indexer) GetByHeight(height uint64, newestToOldest bool) ([]types.TransactionResultI, error) {
	return t.getAll(t.heightKey(height), newestToOldest)
}

func (t *Indexer) GetBySender(address crypto.AddressI, newestToOldest bool) ([]types.TransactionResultI, error) {
	return t.getAll(t.senderKey(address.Bytes(), nil), newestToOldest)
}

func (t *Indexer) GetByRecipient(address crypto.AddressI, newestToOldest bool) ([]types.TransactionResultI, error) {
	return t.getAll(t.senderKey(address.Bytes(), nil), newestToOldest)
}

func (t *Indexer) DeleteForHeight(height uint64) error {
	return t.deleteAll(t.heightKey(height))
}

func (t *Indexer) get(key []byte) (types.TransactionResultI, error) {
	bz, err := t.db.Get(key)
	if err != nil {
		return nil, err
	}
	ptr := new(types.TransactionResult)
	if err = cdc.Unmarshal(bz, ptr); err != nil {
		return nil, err
	}
	return ptr, nil
}

func (t *Indexer) getAll(prefix []byte, newestToOldest bool) (result []types.TransactionResultI, err error) {
	var it types.IteratorI
	switch newestToOldest {
	case true:
		it, err = t.db.RevIterator(prefix)
	case false:
		it, err = t.db.Iterator(prefix)
	}
	if err != nil {
		return nil, err
	}
	defer it.Close()
	for ; it.Valid(); it.Next() {
		tx, err := t.get(it.Key())
		if err != nil {
			return nil, err
		}
		result = append(result, tx)
	}
	return
}

func (t *Indexer) deleteAll(prefix []byte) error {
	it, err := t.db.Iterator(prefix)
	if err != nil {
		return err
	}
	var keysToDelete [][]byte
	for ; it.Valid(); it.Next() {
		keysToDelete = append(keysToDelete, it.Key())
	}
	for _, key := range keysToDelete {
		if err = t.db.Delete(key); err != nil {
			return err
		}
	}
	return nil
}

func (t *Indexer) indexByHash(hash, bz []byte) (hashKey []byte, err error) {
	key := t.hashKey(hash)
	return key, t.db.Set(key, bz)
}

func (t *Indexer) indexByHeightAndIndex(heightAndIndexKey []byte, bz []byte) error {
	return t.db.Set(heightAndIndexKey, bz)
}

func (t *Indexer) indexBySender(sender, heightAndIndexKey []byte, bz []byte) error {
	return t.db.Set(t.senderKey(sender, heightAndIndexKey), bz)
}

func (t *Indexer) indexByRecipient(recipient, heightAndIndexKey []byte, bz []byte) error {
	if recipient == nil {
		return nil
	}
	return t.db.Set(t.recipientKey(recipient, heightAndIndexKey), bz)
}

func (t *Indexer) hashKey(hash []byte) []byte {
	return t.key(hashPrefix, hash, nil)
}

func (t *Indexer) heightAndIndexKey(height, index uint64) []byte {
	return append(t.heightKey(height), t.encodeBigEndian(index)...)
}

func (t *Indexer) heightKey(height uint64) []byte {
	return t.key(heightPrefix, t.encodeBigEndian(height), nil)
}

func (t *Indexer) senderKey(address, heightAndIndexKey []byte) []byte {
	return t.key(senderPrefix, heightAndIndexKey, address)
}

func (t *Indexer) recipientKey(address, heightAndIndexKey []byte) []byte {
	return t.key(recipientPrefix, heightAndIndexKey, address)
}

func (t *Indexer) key(prefix []byte, param1, param2 []byte) []byte {
	return append(append(prefix, param1...), param2...)
}

func (t *Indexer) encodeBigEndian(i uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, i)
	return b
}

func (t *Indexer) setDB(db *TxnWrapper) { t.db = db }
