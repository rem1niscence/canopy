package store

import (
	"encoding/binary"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"math"
	"time"
)

var _ lib.RWIndexerI = &Indexer{}

var (
	txHashPrefix      = []byte{1}
	txHeightPrefix    = []byte{2}
	txSenderPrefix    = []byte{3}
	txRecipientPrefix = []byte{4}
	blockHashPrefix   = []byte{5}
	blockHeightPrefix = []byte{6}
	qcHeightPrefix    = []byte{7}

	delim = []byte("/")
)

type Indexer struct {
	db *TxnWrapper
}

func (t *Indexer) IndexBlock(b *lib.BlockResult) lib.ErrorI {
	bz, err := lib.Marshal(b.BlockHeader)
	if err != nil {
		return err
	}
	hashKey, err := t.indexBlockByHash(b.BlockHeader.Hash, bz)
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
	return nil
}

func (t *Indexer) DeleteBlockForHeight(height uint64) lib.ErrorI {
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
	return t.db.Delete(hashKey)
}

func (t *Indexer) GetBlockByHash(hash []byte) (*lib.BlockResult, lib.ErrorI) {
	return t.getBlock(t.blockHashKey(hash))
}

func (t *Indexer) GetBlocks(p lib.PageParams) (page *lib.Page, err lib.ErrorI) {
	it, err := t.db.RevIterator(blockHeightPrefix)
	if err != nil {
		return nil, err
	}
	defer it.Close()
	skipIdx, page := p.SkipToIndex(), lib.NewPage(p)
	page.Type = lib.BlockResultsPageName
	blkResults := make(lib.BlockResults, 0)
	for countOnly, i := false, 0; it.Valid(); func() { it.Next(); i++ }() {
		page.TotalCount++
		switch {
		case i < skipIdx || countOnly:
			continue
		}
		block, err := t.getBlock(it.Value())
		if err != nil {
			return nil, err
		}
		bz, err := lib.Marshal(block)
		if err != nil {
			return nil, err
		}
		block.Meta = &lib.BlockResultMeta{Size: uint64(len(bz))}
		if page.Count != 0 {
			nextBlock := blkResults[page.Count-1]
			blockTime := time.UnixMicro(int64(block.BlockHeader.Time))
			nextBlkTime := time.UnixMicro(int64(nextBlock.BlockHeader.Time))
			nextBlock.Meta.Took = nextBlkTime.Sub(blockTime).Truncate(time.Second).String()
		}
		if i == skipIdx+page.PerPage {
			countOnly = true
			continue
		}
		blkResults = append(blkResults, block)
		page.Results = &blkResults
		page.Count++
	}
	page.TotalPages = int(math.Ceil(float64(page.TotalCount) / float64(page.PerPage)))
	return
}

func (t *Indexer) GetBlockByHeight(height uint64) (*lib.BlockResult, lib.ErrorI) {
	hashKey, err := t.db.Get(t.blockHeightKey(height))
	if err != nil {
		return nil, err
	}
	return t.getBlock(hashKey)
}

func (t *Indexer) GetQCByHeight(height uint64) (*lib.QuorumCertificate, lib.ErrorI) {
	qc, err := t.getQC(t.qcHeightKey(height))
	if err != nil {
		return nil, err
	}
	blkResult, err := t.GetBlockByHeight(height)
	if err != nil {
		return nil, err
	}
	block, err := blkResult.ToBlock()
	if err != nil {
		return nil, err
	}
	qc.Proposal.Block, err = lib.Marshal(block)
	if err != nil {
		return nil, err
	}
	return qc, err
}

func (t *Indexer) DeleteQCForHeight(height uint64) lib.ErrorI {
	return t.db.Delete(t.qcHeightKey(height))
}

func (t *Indexer) IndexQC(qc *lib.QuorumCertificate) lib.ErrorI {
	qc.Proposal.Block = nil
	bz, err := lib.Marshal(&lib.QuorumCertificate{
		Header:       qc.Header,
		Proposal:     qc.Proposal,
		ProposalHash: qc.ProposalHash,
		ProposerKey:  qc.ProposerKey,
		Signature:    qc.Signature,
	})
	if err != nil {
		return err
	}
	return t.indexQCByHeight(qc.Header.Height, bz)
}

func (t *Indexer) IndexTx(result *lib.TxResult) lib.ErrorI {
	bz, err := result.GetBytes()
	if err != nil {
		return err
	}
	hash, err := lib.StringToBytes(result.GetTxHash())
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

func (t *Indexer) GetTxByHash(hash []byte) (*lib.TxResult, lib.ErrorI) {
	return t.getTx(t.txHashKey(hash))
}

func (t *Indexer) GetTxsByHeight(height uint64, newestToOldest bool, p lib.PageParams) (*lib.Page, lib.ErrorI) {
	return t.getTxs(t.txHeightKey(height), newestToOldest, p)
}

func (t *Indexer) GetTxsByHeightNonPaginated(height uint64, newestToOldest bool) ([]*lib.TxResult, lib.ErrorI) {
	return t.getTxsNonPaginated(t.txHeightKey(height), newestToOldest)
}

func (t *Indexer) GetTxsBySender(address crypto.AddressI, newestToOldest bool, p lib.PageParams) (*lib.Page, lib.ErrorI) {
	return t.getTxs(t.txSenderKey(address.Bytes(), nil), newestToOldest, p)
}

func (t *Indexer) GetTxsByRecipient(address crypto.AddressI, newestToOldest bool, p lib.PageParams) (*lib.Page, lib.ErrorI) {
	return t.getTxs(t.txRecipientKey(address.Bytes(), nil), newestToOldest, p)
}

func (t *Indexer) DeleteTxsForHeight(height uint64) lib.ErrorI {
	return t.deleteAll(t.txHeightKey(height))
}

func (t *Indexer) getQC(key []byte) (*lib.QuorumCertificate, lib.ErrorI) {
	bz, err := t.db.Get(key)
	if err != nil {
		return nil, err
	}
	ptr := new(lib.QuorumCertificate)
	if err = lib.Unmarshal(bz, ptr); err != nil {
		return nil, err
	}
	return ptr, nil
}

func (t *Indexer) getBlock(key []byte) (*lib.BlockResult, lib.ErrorI) {
	bz, err := t.db.Get(key)
	if err != nil {
		return nil, err
	}
	ptr := new(lib.BlockHeader)
	if err = lib.Unmarshal(bz, ptr); err != nil {
		return nil, err
	}
	txs, err := t.GetTxsByHeightNonPaginated(ptr.Height, false)
	if err != nil {
		return nil, err
	}
	return &lib.BlockResult{
		BlockHeader:  ptr,
		Transactions: txs,
	}, nil
}

func (t *Indexer) getTx(key []byte) (*lib.TxResult, lib.ErrorI) {
	bz, err := t.db.Get(key)
	if err != nil {
		return nil, err
	}
	ptr := new(lib.TxResult)
	if err = lib.Unmarshal(bz, ptr); err != nil {
		return nil, err
	}
	return ptr, nil
}

func (t *Indexer) getTxsNonPaginated(prefix []byte, newestToOldest bool) (results []*lib.TxResult, err lib.ErrorI) {
	var it lib.IteratorI
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
		tx, err := t.getTx(it.Value())
		if err != nil {
			return nil, err
		}
		results = append(results, tx)
	}
	return
}

func (t *Indexer) getTxs(prefix []byte, newestToOldest bool, p lib.PageParams) (page *lib.Page, err lib.ErrorI) {
	var it lib.IteratorI
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
	skipIdx, page := p.SkipToIndex(), lib.NewPage(p)
	page.Type = lib.TxResultsPageName
	txResults := make(lib.TxResults, 0)
	for countOnly, i := false, 0; it.Valid(); func() { it.Next(); i++ }() {
		page.TotalCount++
		switch {
		case i < skipIdx || countOnly:
			continue
		case i == skipIdx+page.PerPage:
			countOnly = true
			continue
		}
		tx, err := t.getTx(it.Value())
		if err != nil {
			return nil, err
		}
		txResults = append(txResults, tx)
		page.Results = &txResults
		page.Count++
	}
	page.TotalPages = int(math.Ceil(float64(page.TotalCount) / float64(page.PerPage)))
	return
}

func (t *Indexer) deleteAll(prefix []byte) lib.ErrorI {
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

func (t *Indexer) indexTxByHash(hash, bz []byte) (hashKey []byte, err lib.ErrorI) {
	key := t.txHashKey(hash)
	return key, t.db.Set(key, bz)
}

func (t *Indexer) indexTxByHeightAndIndex(heightAndIndexKey []byte, bz []byte) lib.ErrorI {
	return t.db.Set(heightAndIndexKey, bz)
}

func (t *Indexer) indexTxBySender(sender, heightAndIndexKey []byte, bz []byte) lib.ErrorI {
	return t.db.Set(t.txSenderKey(sender, heightAndIndexKey), bz)
}

func (t *Indexer) indexTxByRecipient(recipient, heightAndIndexKey []byte, bz []byte) lib.ErrorI {
	if recipient == nil {
		return nil
	}
	return t.db.Set(t.txRecipientKey(recipient, heightAndIndexKey), bz)
}

func (t *Indexer) indexQCByHeight(height uint64, bz []byte) lib.ErrorI {
	return t.db.Set(t.qcHeightKey(height), bz)
}

func (t *Indexer) indexBlockByHash(hash, bz []byte) (hashKey []byte, err lib.ErrorI) {
	key := t.blockHashKey(hash)
	return key, t.db.Set(key, bz)
}

func (t *Indexer) indexBlockByHeight(height uint64, bz []byte) lib.ErrorI {
	return t.db.Set(t.blockHeightKey(height), bz)
}

func (t *Indexer) txHashKey(hash []byte) []byte {
	return t.key(txHashPrefix, hash, nil)
}

func (t *Indexer) heightAndIndexKey(height, index uint64) []byte {
	return multiAppendWithDelimiter(t.txHeightKey(height), t.encodeBigEndian(index))
}

func (t *Indexer) txHeightKey(height uint64) []byte {
	return t.key(txHeightPrefix, t.encodeBigEndian(height), nil)
}

func (t *Indexer) txSenderKey(address, heightAndIndexKey []byte) []byte {
	return t.key(txSenderPrefix, address, heightAndIndexKey)
}

func (t *Indexer) txRecipientKey(address, heightAndIndexKey []byte) []byte {
	return t.key(txRecipientPrefix, address, heightAndIndexKey)
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
	return multiAppendWithDelimiter(prefix, param1, param2)
}

func multiAppendWithDelimiter(toAppend ...[]byte) (res []byte) {
	// fn("tx", "10", nil)
	// return "tx/10/"
	for _, a := range toAppend {
		if a == nil {
			continue
		}
		withTerminatingDelim := append(a, delim...)
		res = append(res, withTerminatingDelim...)
	}
	return
}

func (t *Indexer) encodeBigEndian(i uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, i)
	return b
}

func (t *Indexer) setDB(db *TxnWrapper) { t.db = db }
