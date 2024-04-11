package store

import (
	"encoding/binary"
	"github.com/ginchuco/ginchu/crypto"
	"github.com/ginchuco/ginchu/types"
)

var _ types.RWIndexerI = &Indexer{}

var (
	txHashPrefix         = []byte{1}
	txHeightPrefix       = []byte{2}
	txSenderPrefix       = []byte{3}
	txRecipientPrefix    = []byte{4}
	blockHashPrefix      = []byte{5}
	blockHeightPrefix    = []byte{6}
	evidenceHashPrefix   = []byte{7}
	evidenceHeightPrefix = []byte{8}
	qcHeightPrefix       = []byte{9}
)

type Indexer struct {
	db *TxnWrapper
}

func (t *Indexer) IndexBlock(b *types.BlockResult) types.ErrorI {
	bz, err := types.Marshal(b.BlockHeader)
	if err != nil {
		return err
	}
	hashKey, err := t.indexBlockByHash(crypto.Hash(bz), bz)
	if err != nil {
		return err
	}
	if err = t.indexBlockByHeight(b.BlockHeader.Height, hashKey); err != nil {
		return err
	}
	for _, tx := range b.Transactions {
		if err = t.IndexTx(tx); err != nil {
			return err
		}
	}
	for i, evidence := range b.BlockHeader.Evidence {
		if err = t.IndexEvidence(b.BlockHeader.Height, uint64(i), evidence); err != nil {
			return err
		}
	}
	return nil
}

func (t *Indexer) DeleteBlockForHeight(height uint64) types.ErrorI {
	heightKey := t.blockHeightKey(height)
	hashKey, err := t.db.Get(heightKey)
	if err != nil {
		return err
	}
	if err = t.db.Delete(heightKey); err != nil {
		return err
	}
	if err = t.DeleteTxsForHeight(height); err != nil {
		return err
	}
	if err = t.DeleteEvidenceForHeight(height); err != nil {
		return err
	}
	return t.db.Delete(hashKey)
}

func (t *Indexer) GetBlockByHash(hash []byte) (*types.BlockResult, types.ErrorI) {
	return t.getBlock(t.blockHashKey(hash))
}

func (t *Indexer) GetBlockByHeight(height uint64) (*types.BlockResult, types.ErrorI) {
	hashKey, err := t.db.Get(t.blockHeightKey(height))
	if err != nil {
		return nil, err
	}
	return t.getBlock(hashKey)
}

func (t *Indexer) GetQCByHeight(height uint64) (*types.QuorumCertificate, types.ErrorI) {
	qc, err := t.getQC(t.qcHeightKey(height))
	if err != nil {
		return nil, err
	}
	blkResult, err := t.GetBlockByHeight(height)
	if err != nil {
		return nil, err
	}
	qc.Block, err = blkResult.ToBlock()
	if err != nil {
		return nil, err
	}
	return qc, nil
}

func (t *Indexer) DeleteQCForHeight(height uint64) types.ErrorI {
	return t.db.Delete(t.qcHeightKey(height))
}

func (t *Indexer) IndexQC(qc *types.QuorumCertificate) types.ErrorI {
	bz, err := types.Marshal(qc)
	if err != nil {
		return err
	}
	qc.Block = nil
	return t.indexQCByHeight(qc.Header.Height, bz)
}

func (t *Indexer) IndexEvidence(height, index uint64, e *types.DoubleSignEvidence) types.ErrorI {
	bz, err := types.Marshal(e)
	if err != nil {
		return err
	}
	hashKey, err := t.indexEvidenceByHash(crypto.Hash(bz), bz)
	if err != nil {
		return err
	}
	return t.indexEvidenceByHeight(height, index, hashKey)
}

func (t *Indexer) GetEvidenceByHeight(height uint64) ([]*types.DoubleSignEvidence, types.ErrorI) {
	return t.getEvidences(t.evidenceHeightKey(height))
}

func (t *Indexer) GetEvidenceByHash(hash []byte) (*types.DoubleSignEvidence, types.ErrorI) {
	return t.getEvidence(t.evidenceHashKey(hash))
}

func (t *Indexer) DeleteEvidenceForHeight(height uint64) types.ErrorI {
	return t.deleteAll(t.evidenceHeightKey(height))
}

func (t *Indexer) IndexTx(result *types.TxResult) types.ErrorI {
	bz, err := result.GetBytes()
	if err != nil {
		return err
	}
	hash, err := types.StringToBytes(result.GetTxHash())
	if err != nil {
		return err
	}
	hashKey, err := t.indexTxByHash(hash, bz)
	if err != nil {
		return err
	}
	heightAndIndexKey := t.heightAndIndexKey(result.GetHeight(), result.GetIndex())
	if err = t.indexTxByHeightAndIndex(heightAndIndexKey, hashKey); err != nil {
		return err
	}
	if err = t.indexTxBySender(result.GetSender(), heightAndIndexKey, hashKey); err != nil {
		return err
	}
	return t.indexTxByRecipient(result.GetRecipient(), heightAndIndexKey, hashKey)
}

func (t *Indexer) GetTxByHash(hash []byte) (*types.TxResult, types.ErrorI) {
	return t.getTx(t.txHashKey(hash))
}

func (t *Indexer) GetTxsByHeight(height uint64, newestToOldest bool) ([]*types.TxResult, types.ErrorI) {
	return t.getTxs(t.txHeightKey(height), newestToOldest)
}

func (t *Indexer) GetTxsBySender(address crypto.AddressI, newestToOldest bool) ([]*types.TxResult, types.ErrorI) {
	return t.getTxs(t.txSenderKey(address.Bytes(), nil), newestToOldest)
}

func (t *Indexer) GetTxsByRecipient(address crypto.AddressI, newestToOldest bool) ([]*types.TxResult, types.ErrorI) {
	return t.getTxs(t.txSenderKey(address.Bytes(), nil), newestToOldest)
}

func (t *Indexer) DeleteTxsForHeight(height uint64) types.ErrorI {
	return t.deleteAll(t.txHeightKey(height))
}

func (t *Indexer) getQC(key []byte) (*types.QuorumCertificate, types.ErrorI) {
	bz, err := t.db.Get(key)
	if err != nil {
		return nil, err
	}
	ptr := new(types.QuorumCertificate)
	if err = types.Unmarshal(bz, ptr); err != nil {
		return nil, err
	}
	return ptr, nil
}

func (t *Indexer) getBlock(key []byte) (*types.BlockResult, types.ErrorI) {
	bz, err := t.db.Get(key)
	if err != nil {
		return nil, err
	}
	ptr := new(types.BlockHeader)
	if err = types.Unmarshal(bz, ptr); err != nil {
		return nil, err
	}
	txs, err := t.GetTxsByHeight(ptr.Height, false)
	if err != nil {
		return nil, err
	}
	return &types.BlockResult{
		BlockHeader:  ptr,
		Transactions: txs,
	}, nil
}

func (t *Indexer) getEvidence(key []byte) (*types.DoubleSignEvidence, types.ErrorI) {
	bz, err := t.db.Get(key)
	if err != nil {
		return nil, err
	}
	ptr := new(types.DoubleSignEvidence)
	if err = types.Unmarshal(bz, ptr); err != nil {
		return nil, err
	}
	return ptr, nil
}

func (t *Indexer) getEvidences(prefix []byte) (result []*types.DoubleSignEvidence, err types.ErrorI) {
	it, err := t.db.RevIterator(prefix)
	if err != nil {
		return nil, err
	}
	defer it.Close()
	for ; it.Valid(); it.Next() {
		tx, err := t.getEvidence(it.Key())
		if err != nil {
			return nil, err
		}
		result = append(result, tx)
	}
	return
}

func (t *Indexer) getTx(key []byte) (*types.TxResult, types.ErrorI) {
	bz, err := t.db.Get(key)
	if err != nil {
		return nil, err
	}
	ptr := new(types.TxResult)
	if err = types.Unmarshal(bz, ptr); err != nil {
		return nil, err
	}
	return ptr, nil
}

func (t *Indexer) getTxs(prefix []byte, newestToOldest bool) (result []*types.TxResult, err types.ErrorI) {
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
		tx, err := t.getTx(it.Key())
		if err != nil {
			return nil, err
		}
		result = append(result, tx)
	}
	return
}

func (t *Indexer) deleteAll(prefix []byte) types.ErrorI {
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

func (t *Indexer) indexTxByHash(hash, bz []byte) (hashKey []byte, err types.ErrorI) {
	key := t.txHashKey(hash)
	return key, t.db.Set(key, bz)
}

func (t *Indexer) indexTxByHeightAndIndex(heightAndIndexKey []byte, bz []byte) types.ErrorI {
	return t.db.Set(heightAndIndexKey, bz)
}

func (t *Indexer) indexTxBySender(sender, heightAndIndexKey []byte, bz []byte) types.ErrorI {
	return t.db.Set(t.txSenderKey(sender, heightAndIndexKey), bz)
}

func (t *Indexer) indexTxByRecipient(recipient, heightAndIndexKey []byte, bz []byte) types.ErrorI {
	if recipient == nil {
		return nil
	}
	return t.db.Set(t.txRecipientKey(recipient, heightAndIndexKey), bz)
}

func (t *Indexer) indexEvidenceByHash(hash, bz []byte) (hashKey []byte, err types.ErrorI) {
	key := t.evidenceHashKey(hash)
	return key, t.db.Set(key, bz)
}

func (t *Indexer) indexEvidenceByHeight(height, index uint64, bz []byte) types.ErrorI {
	return t.db.Set(t.evidenceHeightAndIndexKey(height, index), bz)
}

func (t *Indexer) indexQCByHeight(height uint64, bz []byte) types.ErrorI {
	return t.db.Set(t.qcHeightKey(height), bz)
}

func (t *Indexer) indexBlockByHash(hash, bz []byte) (hashKey []byte, err types.ErrorI) {
	key := t.blockHashKey(hash)
	return key, t.db.Set(key, bz)
}

func (t *Indexer) indexBlockByHeight(height uint64, bz []byte) types.ErrorI {
	return t.db.Set(t.blockHeightKey(height), bz)
}

func (t *Indexer) txHashKey(hash []byte) []byte {
	return t.key(txHashPrefix, hash, nil)
}

func (t *Indexer) heightAndIndexKey(height, index uint64) []byte {
	return append(t.txHeightKey(height), t.encodeBigEndian(index)...)
}

func (t *Indexer) txHeightKey(height uint64) []byte {
	return t.key(txHeightPrefix, t.encodeBigEndian(height), nil)
}

func (t *Indexer) txSenderKey(address, heightAndIndexKey []byte) []byte {
	return t.key(txSenderPrefix, heightAndIndexKey, address)
}

func (t *Indexer) txRecipientKey(address, heightAndIndexKey []byte) []byte {
	return t.key(txRecipientPrefix, heightAndIndexKey, address)
}

func (t *Indexer) evidenceHashKey(hash []byte) []byte {
	return t.key(evidenceHashPrefix, hash, nil)
}

func (t *Indexer) evidenceHeightAndIndexKey(height, index uint64) []byte {
	return append(t.evidenceHeightKey(height), t.encodeBigEndian(index)...)
}

func (t *Indexer) evidenceHeightKey(height uint64) []byte {
	return t.key(evidenceHeightPrefix, t.encodeBigEndian(height), nil)
}

func (t *Indexer) blockHashKey(hash []byte) []byte {
	return t.key(blockHashPrefix, hash, nil)
}

func (t *Indexer) blockHeightKey(height uint64) []byte {
	return t.key(blockHeightPrefix, t.encodeBigEndian(height), nil)
}

func (t *Indexer) qcHeightKey(height uint64) []byte {
	return t.key(qcHeightPrefix, t.encodeBigEndian(height), nil)
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
