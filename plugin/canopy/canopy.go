package canopy

import (
	"bytes"
	"github.com/ginchuco/ginchu/fsm"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"github.com/ginchuco/ginchu/plugin"
	"math"
	"time"
)

var _ plugin.CanopyPlugin = new(Plugin)

const (
	BlockTimeMinusConsensus = 9*time.Minute + 40*time.Second
)

type Plugin struct {
	FSM                      *fsm.StateMachine
	Mempool                  *Mempool
	PublicKey                []byte
	PrivateKey               crypto.PrivateKeyI
	Config                   lib.Config
	log                      lib.LoggerI
	bftTimerFinishedCallback plugin.BftTimerFinishedCallback
	gossipTxCallback         plugin.GossipTxCallback
	resetBFTTrigger          chan struct{}
}

func (p *Plugin) LoadLastCommitTime(height uint64) time.Time {
	cert, err := p.LoadCertificate(height)
	if err != nil {
		p.log.Error(err.Error())
		return time.Time{}
	}
	block, err := lib.ProposalToBlock(cert.Proposal)
	if err != nil {
		p.log.Error(err.Error())
		return time.Time{}
	}
	if block.BlockHeader == nil {
		return time.Time{}
	}
	return time.UnixMicro(int64(block.BlockHeader.Time))
}

func RegisterNew(c lib.Config, valKey crypto.PrivateKeyI, db lib.StoreI, l lib.LoggerI) lib.ErrorI {
	sm, err := fsm.New(c, db, l)
	if err != nil {
		return err
	}
	mempool, err := NewMempool(sm, c.MempoolConfig, l)
	if err != nil {
		return err
	}
	plugin.RegisteredPlugins[lib.CANOPY_COMMITTEE_ID] = &Plugin{
		FSM:             sm,
		Mempool:         mempool,
		PublicKey:       valKey.PublicKey().Bytes(),
		PrivateKey:      valKey,
		Config:          c,
		log:             l,
		resetBFTTrigger: make(chan struct{}, 1),
	}
	return nil
}

func (p *Plugin) WithCallbacks(bftTimerFinishedCallback plugin.BftTimerFinishedCallback, gossipTxCallback plugin.GossipTxCallback) {
	p.bftTimerFinishedCallback, p.gossipTxCallback = bftTimerFinishedCallback, gossipTxCallback
}
func (p *Plugin) Start() {
	for {
		timer := p.newTimer() // timer waiting for first reset trigger
		select {
		case <-p.resetBFTTrigger: // controller commands a reset and start of the bft trigger timer
			p.resetTimer(timer, BlockTimeMinusConsensus)
		case <-timer.C: // tell controller the bft trigger timer finished
			p.stopTimer(timer)
			p.bftTimerFinishedCallback(lib.CANOPY_COMMITTEE_ID)
		}
	}
}

func (p *Plugin) ProduceProposal(committeeHeight uint64) (proposal *lib.Proposal, err lib.ErrorI) {
	defer func() { p.FSM.Reset() }()
	height := p.FSM.Height()
	lastBlock, err := p.FSM.LoadBlock(height - 1)
	qc, err := p.FSM.LoadCertificateWOProposal(height - 1)
	if err != nil {
		return nil, err
	}
	maxBlockSize, err := p.FSM.GetMaxBlockSize()
	if err != nil {
		return nil, err
	}
	p.Mempool.checkMempool()
	pubKey, _ := crypto.NewBLSPublicKeyFromBytes(p.PublicKey)
	numTxs, transactions := p.Mempool.GetTransactions(maxBlockSize)
	header := &lib.BlockHeader{
		Height:                height + 1,
		NetworkId:             p.FSM.NetworkID,
		Time:                  uint64(time.Now().UnixMicro()),
		NumTxs:                uint64(numTxs),
		TotalTxs:              lastBlock.BlockHeader.TotalTxs + uint64(numTxs),
		LastBlockHash:         lastBlock.BlockHeader.LastBlockHash,
		ProposerAddress:       pubKey.Address().Bytes(),
		LastQuorumCertificate: qc,
	}
	block := &lib.Block{
		BlockHeader:  header,
		Transactions: transactions,
	}
	block.BlockHeader, _, _, err = p.FSM.ApplyBlock(block)
	if err != nil {
		return nil, err
	}
	blockBz, err := lib.Marshal(block)
	if err != nil {
		return nil, err
	}
	return &lib.Proposal{
		Block:     blockBz,
		BlockHash: header.Hash,
		RewardRecipients: &lib.RewardRecipients{
			PaymentPercents: []*lib.PaymentPercents{{
				Address: header.ProposerAddress,
				Percent: 100,
			}},
		},
		Meta: &lib.ProposalMeta{
			CommitteeId:     lib.CANOPY_COMMITTEE_ID,
			CommitteeHeight: committeeHeight,
			ChainHeight:     block.BlockHeader.Height,
		},
	}, nil
}

func (p *Plugin) ValidateProposal(committeeHeight uint64, proposal *lib.Proposal) (err lib.ErrorI) {
	defer func() { p.FSM.Reset() }()
	block, err := proposal.Check(lib.CANOPY_COMMITTEE_ID, committeeHeight)
	if err != nil {
		return err
	}
	_, err = p.ApplyAndValidateBlock(block)
	return
}

func (p *Plugin) CheckPeerProposal(maxHeight uint64, prop *lib.Proposal) (outOfSync bool, err lib.ErrorI) {
	block := new(lib.Block)
	if err = lib.Unmarshal(prop.Block, block); err != nil {
		return
	}
	h := p.FSM.Height()
	if h > block.BlockHeader.Height {
		err = lib.ErrWrongMaxHeight()
		return
	}
	outOfSync = h != block.BlockHeader.Height
	if block.BlockHeader.Height > maxHeight {
		err = lib.ErrWrongMaxHeight()
		return
	}
	hash, _ := block.BlockHeader.SetHash()
	if !bytes.Equal(prop.BlockHash, hash) {
		err = lib.ErrMismatchProposalHash()
		return
	}
	return
}

func (p *Plugin) ApplyAndValidateBlock(b *lib.Block) (*lib.BlockResult, lib.ErrorI) {
	if err := b.Check(); err != nil {
		return nil, err
	}
	blockHash, blockHeight := lib.BytesToString(b.BlockHeader.Hash), b.BlockHeader.Height
	p.log.Debugf("Applying block %s for height %d", blockHash, blockHeight)
	header, txResults, _, err := p.FSM.ApplyBlock(b)
	if err != nil {
		return nil, err
	}
	p.log.Debugf("Validating block header %s for height %d", blockHash, blockHeight)
	if err = p.CheckBlockHeader(b.BlockHeader, header); err != nil {
		return nil, err
	}
	p.log.Infof("Block %s is valid for height %d ✅ ", blockHash, blockHeight)
	return &lib.BlockResult{
		BlockHeader:  b.BlockHeader,
		Transactions: txResults,
	}, nil
}

func (p *Plugin) CheckBlockHeader(header *lib.BlockHeader, compare *lib.BlockHeader) lib.ErrorI {
	hash, e := compare.SetHash()
	if e != nil {
		return e
	}
	if !bytes.Equal(hash, header.Hash) {
		return lib.ErrUnequalBlockHash()
	}
	if header.Height > 2 {
		lastQCHeight := header.Height - 1
		vs, err := p.FSM.LoadValSet(lastQCHeight)
		if err != nil {
			return err
		}
		isPartialQC, err := header.LastQuorumCertificate.Check(vs, lastQCHeight)
		if err != nil {
			return err
		}
		if isPartialQC {
			return lib.ErrNoMaj23()
		}
	}
	if err := p.validateBlockTime(header); err != nil {
		return err
	}
	return nil
}

func (p *Plugin) validateBlockTime(header *lib.BlockHeader) lib.ErrorI {
	now := time.Now()
	maxTime, minTime, t := now.Add(time.Hour), now.Add(-time.Hour), time.UnixMicro(int64(header.Time))
	if minTime.Compare(t) > 0 || maxTime.Compare(t) < 0 {
		return lib.ErrInvalidBlockTime()
	}
	return nil
}

func (p *Plugin) IntegratedChain() bool { return true }

func (p *Plugin) HandleTx(tx []byte) lib.ErrorI {
	hash := crypto.Hash(tx)
	// indexer
	txResult, err := p.FSM.Store().(lib.StoreI).GetTxByHash(hash)
	if err != nil {
		return err
	}
	if txResult.TxHash != "" {
		return lib.ErrDuplicateTx(hash)
	}
	// mempool
	if h := lib.BytesToString(hash); p.Mempool.Contains(h) {
		return lib.ErrTxFoundInMempool(h)
	}
	return p.Mempool.HandleTransaction(tx)
}

// CommitCertificate used after consensus decides on a block
// - applies block against the fsm
// - indexes the block and its transactions
// - removes block transactions from mempool
// - re-checks all transactions in mempool
// - atomically writes all to the underlying db
// - sets up the app for the next height
func (p *Plugin) CommitCertificate(qc *lib.QuorumCertificate) lib.ErrorI {
	p.log.Debugf("TryCommit block %s", lib.BytesToString(qc.ProposalHash))
	store := p.FSM.Store().(lib.StoreI)
	blk, err := lib.ProposalToBlock(qc.Proposal)
	if err != nil {
		return err
	}
	blockResult, err := p.ApplyAndValidateBlock(blk)
	if err != nil {
		return err
	}
	p.log.Debugf("Indexing quorum certificate for height %d", qc.Header.Height)
	if err = store.IndexQC(qc); err != nil {
		return err
	}
	if blk.BlockHeader.Height != 1 {
		p.log.Debugf("Indexing last quorum certificate for height %d", blk.BlockHeader.LastQuorumCertificate.Header.Height)
		if err = store.IndexQC(blk.BlockHeader.LastQuorumCertificate); err != nil {
			return err
		}
	}
	p.log.Debugf("Indexing block %d", blk.BlockHeader.Height)
	if err = store.IndexBlock(blockResult); err != nil {
		return err
	}
	for _, tx := range blk.Transactions {
		p.log.Debugf("tx %s was included in a block so removing from mempool", crypto.HashString(tx))
		p.Mempool.DeleteTransaction(tx)
	}
	p.log.Debug("Checking mempool for newly invalid transactions")
	p.Mempool.checkMempool()
	p.log.Debug("Committing to store")
	if _, err = store.Commit(); err != nil {
		return err
	}
	p.log.Infof("Committed block %s at H:%d 🔒", lib.BytesToString(qc.ProposalHash), blk.BlockHeader.Height)
	p.log.Debug("Setting up FSM for next height")
	p.FSM, err = fsm.New(p.Config, store, p.log)
	if err != nil {
		return err
	}
	p.log.Debug("Setting up Mempool for next height")
	if p.Mempool.FSM, err = p.FSM.Copy(); err != nil {
		return err
	}
	if err = p.Mempool.FSM.BeginBlock(); err != nil {
		return err
	}
	p.log.Debug("Commit done")
	return nil
}

func (p *Plugin) LoadCertificate(height uint64) (*lib.QuorumCertificate, lib.ErrorI) {
	return p.FSM.LoadCertificateWOProposal(height)
}
func (p *Plugin) GetFSM() *fsm.StateMachine { return p.FSM }
func (p *Plugin) ResetAndStartBFTTimer()    { p.resetBFTTrigger <- struct{}{} }
func (p *Plugin) Height() uint64            { return p.FSM.Height() }
func (p *Plugin) newTimer() (timer *time.Timer) {
	timer = time.NewTimer(0)
	<-timer.C
	return
}

func (p *Plugin) resetTimer(t *time.Timer, duration time.Duration) {
	p.stopTimer(t)
	t.Reset(duration)
}

func (p *Plugin) stopTimer(t *time.Timer) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
}

// Mempool accepts or rejects incoming txs based on the mempool state
// - recheck when
//   - mempool dropped some percent of the lowest fee txs
//   - new tx has higher fee than the lowest
//
// - notes:
//   - new tx added may also be evicted, this is expected behavior
type Mempool struct {
	log           lib.LoggerI
	FSM           *fsm.StateMachine
	cachedResults lib.TxResults
	lib.Mempool
}

func NewMempool(fsm *fsm.StateMachine, config lib.MempoolConfig, log lib.LoggerI) (m *Mempool, err lib.ErrorI) {
	m = &Mempool{
		log:     log,
		Mempool: lib.NewMempool(config),
	}
	m.FSM, err = fsm.Copy()
	if err != nil {
		return nil, err
	}
	return m, err
}

func (m *Mempool) HandleTransaction(tx []byte) lib.ErrorI {
	result, err := m.applyAndWriteTx(tx)
	if err != nil {
		return err
	}
	recheck, err := m.AddTransaction(tx, result.Transaction.Fee)
	if err != nil {
		return err
	}
	m.log.Infof("added tx %s to mempool for checking", crypto.HashString(tx))
	m.cachedResults = append(m.cachedResults, result)
	if recheck {
		m.checkMempool()
	}
	return nil
}

func (m *Mempool) checkMempool() {
	m.FSM.ResetToBeginBlock()
	m.cachedResults = nil
	var remove [][]byte
	it := m.Iterator()
	defer it.Close()
	for ; it.Valid(); it.Next() {
		tx := it.Key()
		result, err := m.applyAndWriteTx(tx)
		if err != nil {
			m.log.Error(err.Error())
			remove = append(remove, tx)
			continue
		}
		m.cachedResults = append(m.cachedResults, result)
	}
	for _, tx := range remove {
		m.log.Infof("removed tx %s from mempool", crypto.HashString(tx))
		m.DeleteTransaction(tx)
	}
}

func (m *Mempool) applyAndWriteTx(tx []byte) (result *lib.TxResult, err lib.ErrorI) {
	store := m.FSM.Store()
	txn, err := m.FSM.TxnWrap()
	if err != nil {
		return nil, err
	}
	defer func() { m.FSM.SetStore(store); txn.Discard() }()
	result, err = m.FSM.ApplyTransaction(uint64(m.Size()), tx, crypto.HashString(tx))
	if err != nil {
		return nil, err
	}
	if err = txn.Write(); err != nil {
		return nil, err
	}
	return result, nil
}
func (p *Plugin) PendingPageForRPC(params lib.PageParams) (page *lib.Page, err lib.ErrorI) {
	return p.Mempool.PendingPageForRPC(params)
}
func (m *Mempool) PendingPageForRPC(p lib.PageParams) (page *lib.Page, err lib.ErrorI) {
	skipIdx, page := p.SkipToIndex(), lib.NewPage(p)
	page.Type = lib.PendingResultsPageName
	txResults := make(lib.TxResults, 0)
	for countOnly, i := false, 0; i < len(m.cachedResults); i++ {
		page.TotalCount++
		switch {
		case i < skipIdx || countOnly:
			continue
		case i == skipIdx+page.PerPage:
			countOnly = true
			continue
		}
		txResults = append(txResults, m.cachedResults[i])
		page.Results = &txResults
		page.Count++
	}
	page.TotalPages = int(math.Ceil(float64(page.TotalCount) / float64(page.PerPage)))
	return
}
