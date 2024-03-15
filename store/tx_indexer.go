package store

import (
	"encoding/binary"
	"github.com/ginchuco/ginchu/crypto"
	"github.com/ginchuco/ginchu/types"
)

var _ types.TxIndexerI = &TxIndexer{}

var (
	hashPrefix      = []byte{1}
	heightPrefix    = []byte{2}
	senderPrefix    = []byte{3}
	recipientPrefix = []byte{4}
)

type TxIndexer struct {
	db *TxnWrapper
}

func (t *TxIndexer) IndexTx(result types.TransactionResultI) error {
	bz, err := result.GetBytes()
	if err != nil {
		return err
	}
	txHash, err := result.GetTxHash()
	if err != nil {
		return err
	}
	hashKey, err := t.indexByHash(txHash, bz)
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

func (t *TxIndexer) GetTxByHash(hash []byte) (types.TransactionResultI, error) {
	return t.get(t.hashKey(hash))
}

func (t *TxIndexer) GetTxByHeight(height uint64, newestToOldest bool) ([]types.TransactionResultI, error) {
	return t.getAll(t.heightKey(height), newestToOldest)
}

func (t *TxIndexer) GetTxsBySender(address crypto.AddressI, newestToOldest bool) ([]types.TransactionResultI, error) {
	return t.getAll(t.senderKey(address.Bytes(), nil), newestToOldest)
}

func (t *TxIndexer) GetTxsByRecipient(address crypto.AddressI, newestToOldest bool) ([]types.TransactionResultI, error) {
	return t.getAll(t.senderKey(address.Bytes(), nil), newestToOldest)
}

func (t *TxIndexer) DeleteTxsByHeight(height uint64) error {
	return t.deleteAll(t.heightKey(height))
}

func (t *TxIndexer) get(key []byte) (types.TransactionResultI, error) {
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

func (t *TxIndexer) getAll(prefix []byte, newestToOldest bool) (result []types.TransactionResultI, err error) {
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
		txn, err := t.get(it.Key())
		if err != nil {
			return nil, err
		}
		result = append(result, txn)
	}
	return
}

func (t *TxIndexer) deleteAll(prefix []byte) error {
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

func (t *TxIndexer) indexByHash(hash, bz []byte) (hashKey []byte, err error) {
	key := t.hashKey(hash)
	return key, t.db.Set(key, bz)
}

func (t *TxIndexer) indexByHeightAndIndex(heightAndIndexKey []byte, bz []byte) error {
	return t.db.Set(heightAndIndexKey, bz)
}

func (t *TxIndexer) indexBySender(sender, heightAndIndexKey []byte, bz []byte) error {
	return t.db.Set(t.senderKey(sender, heightAndIndexKey), bz)
}

func (t *TxIndexer) indexByRecipient(recipient, heightAndIndexKey []byte, bz []byte) error {
	if recipient == nil {
		return nil
	}
	return t.db.Set(t.recipientKey(recipient, heightAndIndexKey), bz)
}

func (t *TxIndexer) hashKey(hash []byte) []byte {
	return t.key(hashPrefix, hash, nil)
}

func (t *TxIndexer) heightAndIndexKey(height, index uint64) []byte {
	return append(t.heightKey(height), t.encodeBigEndian(index)...)
}

func (t *TxIndexer) heightKey(height uint64) []byte {
	return t.key(heightPrefix, t.encodeBigEndian(height), nil)
}

func (t *TxIndexer) senderKey(address, heightAndIndexKey []byte) []byte {
	return t.key(senderPrefix, heightAndIndexKey, address)
}

func (t *TxIndexer) recipientKey(address, heightAndIndexKey []byte) []byte {
	return t.key(recipientPrefix, heightAndIndexKey, address)
}

func (t *TxIndexer) key(prefix []byte, param1, param2 []byte) []byte {
	return append(append(prefix, param1...), param2...)
}

func (t *TxIndexer) encodeBigEndian(i uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, i)
	return b
}

func (t *TxIndexer) setDB(db *TxnWrapper) { t.db = db }
