package fsm

import (
	"bytes"
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"time"
)

// TODO unstaking limitations and force unstake for slash below minimum stake

type StateMachine struct {
	Config            lib.Config
	BeginBlockParams  *lib.BeginBlockParams
	ProtocolVersion   int
	NetworkID         uint32
	height            uint64
	proposeVoteConfig types.ProposalVoteConfig
	store             lib.RWStoreI
	log               lib.LoggerI
}

func New(c lib.Config, store lib.StoreI, log lib.LoggerI) (*StateMachine, lib.ErrorI) {
	sm := &StateMachine{
		Config:            c,
		BeginBlockParams:  new(lib.BeginBlockParams),
		ProtocolVersion:   c.ProtocolVersion,
		NetworkID:         c.NetworkID,
		proposeVoteConfig: types.AcceptAllProposals,
		log:               log,
	}
	return sm, sm.Initialize(store)
}

func NewWithBeginBlock(bb *lib.BeginBlockParams, c lib.Config, height uint64, store lib.RWStoreI, log lib.LoggerI) *StateMachine {
	return &StateMachine{
		Config:            c,
		BeginBlockParams:  bb,
		ProtocolVersion:   c.ProtocolVersion,
		NetworkID:         c.NetworkID,
		height:            height,
		proposeVoteConfig: types.AcceptAllProposals,
		store:             store,
		log:               log,
	}
}

func (s *StateMachine) Initialize(db lib.StoreI) (error lib.ErrorI) {
	s.height, s.store = db.Version(), db
	if s.height != 0 {
		lastStore, err := db.NewReadOnly(s.height)
		if err != nil {
			return err
		}
		lastFSM := StateMachine{store: lastStore}
		if s.BeginBlockParams.ValidatorSet, err = lastFSM.GetConsensusValidators(); err != nil {
			return err
		}
		if s.height != 1 {
			blk, e := db.GetBlockByHeight(s.height - 1)
			if e != nil {
				return e
			}
			s.BeginBlockParams.BlockHeader = blk.BlockHeader
		} else {
			s.BeginBlockParams.BlockHeader, err = s.GenesisBlockHeader()
			if err != nil {
				return err
			}
		}
		return
	} else {
		if err := s.NewFromGenesisFile(); err != nil {
			return err
		}
		return s.Initialize(db)
	}
}

func (s *StateMachine) ApplyBlock(b *lib.Block) (*lib.BlockHeader, []*lib.TxResult, *lib.ConsensusValidators, lib.ErrorI) {
	store, ok := s.Store().(lib.StoreI)
	if !ok {
		return nil, nil, nil, types.ErrWrongStoreType()
	}
	if err := s.BeginBlock(); err != nil {
		return nil, nil, nil, err
	}
	txResults, txRoot, numTxs, err := s.ApplyTransactions(b)
	if err != nil {
		return nil, nil, nil, err
	}
	eb, err := s.EndBlock()
	if err != nil {
		return nil, nil, nil, err
	}
	validatorRoot, err := s.BeginBlockParams.ValidatorSet.Root()
	if err != nil {
		return nil, nil, nil, err
	}
	nextValidatorRoot, err := eb.ValidatorSet.Root()
	if err != nil {
		return nil, nil, nil, err
	}
	stateRoot, err := store.Root()
	if err != nil {
		return nil, nil, nil, err
	}
	header := lib.BlockHeader{
		Height:                s.Height(),
		Hash:                  nil,
		NetworkId:             s.NetworkID,
		Time:                  b.BlockHeader.Time,
		NumTxs:                uint64(numTxs),
		TotalTxs:              s.BeginBlockParams.BlockHeader.TotalTxs + uint64(numTxs),
		LastBlockHash:         s.BeginBlockParams.BlockHeader.Hash,
		StateRoot:             stateRoot,
		TransactionRoot:       txRoot,
		ValidatorRoot:         validatorRoot,
		NextValidatorRoot:     nextValidatorRoot,
		Evidence:              b.BlockHeader.Evidence,
		ProposerAddress:       b.BlockHeader.ProposerAddress,
		BadProposers:          b.BlockHeader.BadProposers,
		LastQuorumCertificate: b.BlockHeader.LastQuorumCertificate,
	}
	if _, err = header.SetHash(); err != nil {
		return nil, nil, nil, err
	}
	return &header, txResults, eb.ValidatorSet, nil
}

func (s *StateMachine) ValidateBlock(header *lib.BlockHeader, compare *lib.BlockHeader, evidence *lib.ByzantineEvidence, isCandidateBlock bool) lib.ErrorI {
	hash, err := compare.SetHash()
	if err != nil {
		return err
	}
	if !bytes.Equal(hash, header.Hash) {
		return lib.ErrUnequalBlockHash()
	}
	vs, err := lib.NewValidatorSet(s.BeginBlockParams.ValidatorSet)
	if err != nil {
		return err
	}
	if header.Height > 2 {
		isPartialQC, err := header.LastQuorumCertificate.Check(header.Height-1, vs)
		if err != nil {
			return err
		}
		if isPartialQC {
			return lib.ErrNoMaj23()
		}
	}
	if err = s.validateBlockTime(header); err != nil {
		return err
	}
	if isCandidateBlock {
		var minimumEvidenceAge uint64
		minimumEvidenceAge, err = s.GetMinimumEvidenceHeight()
		if err != nil {
			return err
		}
		if err = header.ValidateByzantineEvidence(s.LoadValSet, s.store.(lib.StoreI).GetDoubleSigners, evidence, minimumEvidenceAge); err != nil {
			return err
		}
	}
	return nil
}

func (s *StateMachine) ApplyAndValidateBlock(b *lib.Block, evidence *lib.ByzantineEvidence, isCandidateBlock bool) (*lib.BlockResult, *lib.ConsensusValidators, lib.ErrorI) {
	blockHash, blockHeight := lib.BytesToString(b.BlockHeader.Hash), b.BlockHeader.Height
	s.log.Debugf("Applying block %s for height %d", blockHash, blockHeight)
	header, txResults, valSet, err := s.ApplyBlock(b)
	if err != nil {
		return nil, nil, err
	}
	s.log.Debugf("Validating block %s for height %d", blockHash, blockHeight)
	if err = s.ValidateBlock(b.BlockHeader, header, evidence, isCandidateBlock); err != nil {
		return nil, nil, err
	}
	s.log.Infof("Block %s is valid for height %d ✅ ", blockHash, blockHeight)
	return &lib.BlockResult{
		BlockHeader:  b.BlockHeader,
		Transactions: txResults,
	}, valSet, nil
}

func (s *StateMachine) ApplyTransactions(block *lib.Block) (results []*lib.TxResult, root []byte, n int, er lib.ErrorI) {
	var txBytes [][]byte
	blockSize := uint64(0)
	for index, tx := range block.Transactions {
		result, err := s.ApplyTransaction(uint64(index), tx, crypto.HashString(tx))
		if err != nil {
			return nil, nil, 0, err
		}
		bz, err := result.GetBytes()
		if err != nil {
			return nil, nil, 0, err
		}
		results = append(results, result)
		txBytes = append(txBytes, bz)
		blockSize += uint64(len(bz))
		n++
	}
	maxBlockSize, err := s.GetMaxBlockSize()
	if err != nil {
		return nil, nil, 0, err
	}
	if blockSize > maxBlockSize {
		return nil, nil, 0, types.ErrMaxBlockSize()
	}
	root, _, err = lib.MerkleTree(txBytes)
	return results, root, n, err
}

func (s *StateMachine) TxnWrap() (lib.StoreTxnI, lib.ErrorI) {
	store, ok := s.store.(lib.StoreI)
	if !ok {
		return nil, types.ErrWrongStoreType()
	}
	txn := store.NewTxn()
	s.SetStore(txn)
	return txn, nil
}

func (s *StateMachine) TimeMachine(height uint64) (*StateMachine, lib.ErrorI) {
	store, ok := s.store.(lib.StoreI)
	if !ok {
		return nil, types.ErrWrongStoreType()
	}
	heightBeforeHeight := height - 1
	if heightBeforeHeight <= 1 {
		heightBeforeHeight = 1
	}
	heightBeforeStore, err := store.NewReadOnly(heightBeforeHeight)
	if err != nil {
		return nil, err
	}
	qc, err := heightBeforeStore.GetBlockByHeight(heightBeforeHeight)
	if err != nil {
		return nil, err
	}
	if height < 1 {
		qc.BlockHeader, err = s.GenesisBlockHeader()
		if err != nil {
			return nil, err
		}
	}
	heightBeforeStateMachine := StateMachine{store: heightBeforeStore}
	consensusValidators, err := heightBeforeStateMachine.GetConsensusValidators()
	if err != nil {
		return nil, err
	}
	heightStore, err := store.NewReadOnly(height)
	if err != nil {
		return nil, err
	}
	return NewWithBeginBlock(&lib.BeginBlockParams{
		BlockHeader:  qc.BlockHeader,
		ValidatorSet: consensusValidators,
	}, s.Config, height, heightStore, s.log), nil
}

func (s *StateMachine) LoadValSet(height uint64) (lib.ValidatorSet, lib.ErrorI) {
	if height <= 1 {
		height = 1 // 1 is first non-genesis height
	} else {
		height -= 1 // end block state is begin block of next height
	}
	fsm, err := s.TimeMachine(height)
	if err != nil {
		return lib.ValidatorSet{}, err
	}
	vs, err := fsm.GetConsensusValidators()
	if err != nil {
		return lib.ValidatorSet{}, err
	}
	return lib.NewValidatorSet(vs)
}

func (s *StateMachine) LoadEvidence(height uint64) (*lib.DoubleSigners, lib.ErrorI) {
	return s.store.(lib.StoreI).GetDoubleSigners(height)
}

func (s *StateMachine) GetMaxValidators() (uint64, lib.ErrorI) {
	valParams, err := s.GetParamsVal()
	if err != nil {
		return 0, err
	}
	return valParams.ValidatorMaxCount, nil
}

func (s *StateMachine) GetMaxBlockSize() (uint64, lib.ErrorI) {
	consParams, err := s.GetParamsCons()
	if err != nil {
		return 0, err
	}
	return consParams.BlockSize, nil
}

func (s *StateMachine) LoadBlockAndQC(height uint64) (*lib.QuorumCertificate, lib.ErrorI) {
	store, ok := s.store.(lib.StoreI)
	if !ok {
		return nil, types.ErrWrongStoreType()
	}
	return store.GetQCByHeight(height)
}

func (s *StateMachine) LoadCertificate(height uint64) (*lib.QuorumCertificate, lib.ErrorI) {
	qc, err := s.LoadBlockAndQC(height)
	if err != nil {
		return nil, err
	}
	qc.Block = nil
	return qc, nil
}

func (s *StateMachine) Copy() (*StateMachine, lib.ErrorI) {
	st, ok := s.store.(lib.StoreI)
	if !ok {
		return nil, types.ErrWrongStoreType()
	}
	storeCopy, err := st.Copy()
	if err != nil {
		return nil, err
	}
	return &StateMachine{
		Config:            s.Config,
		BeginBlockParams:  s.BeginBlockParams,
		ProtocolVersion:   s.ProtocolVersion,
		NetworkID:         s.NetworkID,
		height:            s.height,
		proposeVoteConfig: s.proposeVoteConfig,
		store:             storeCopy,
		log:               s.log,
	}, nil
}

func (s *StateMachine) LastBlockHeader() *lib.BlockHeader                { return s.BeginBlockParams.BlockHeader }
func (s *StateMachine) Store() lib.RWStoreI                              { return s.store }
func (s *StateMachine) SetStore(store lib.RWStoreI)                      { s.store = store }
func (s *StateMachine) Height() uint64                                   { return s.height }
func (s *StateMachine) Reset()                                           { s.store.(lib.StoreI).Reset() }
func (s *StateMachine) SetProposalVoteConfig(c types.ProposalVoteConfig) { s.proposeVoteConfig = c }
func (s *StateMachine) validateBlockTime(header *lib.BlockHeader) lib.ErrorI {
	now := time.Now()
	maxTime, minTime, t := now.Add(time.Hour), now.Add(-time.Hour), header.Time.AsTime()
	if minTime.Compare(t) > 0 || maxTime.Compare(t) < 0 {
		return lib.ErrInvalidBlockTime()
	}
	return nil
}
func (s *StateMachine) ResetToBeginBlock() {
	s.Reset()
	if err := s.BeginBlock(); err != nil {
		panic(err)
	}
}

func (s *StateMachine) Set(k, v []byte) lib.ErrorI {
	store := s.Store()
	if err := store.Set(k, v); err != nil {
		return err
	}
	return nil
}

func (s *StateMachine) Get(key []byte) ([]byte, lib.ErrorI) {
	store := s.Store()
	bz, err := store.Get(key)
	if err != nil {
		return nil, err
	}
	return bz, nil
}

func (s *StateMachine) Delete(key []byte) lib.ErrorI {
	store := s.Store()
	if err := store.Delete(key); err != nil {
		return err
	}
	return nil
}

func (s *StateMachine) DeleteAll(keys [][]byte) lib.ErrorI {
	for _, key := range keys {
		if err := s.Delete(key); err != nil {
			return err
		}
	}
	return nil
}

func (s *StateMachine) IterateAndExecute(prefix []byte, callback func(key, value []byte) lib.ErrorI) lib.ErrorI {
	it, err := s.Iterator(prefix)
	if err != nil {
		return err
	}
	defer it.Close()
	for ; it.Valid(); it.Next() {
		if err = callback(it.Key(), it.Value()); err != nil {
			return err
		}
	}
	return nil
}

func (s *StateMachine) Iterator(key []byte) (lib.IteratorI, lib.ErrorI) {
	store := s.Store()
	it, err := store.Iterator(key)
	if err != nil {
		return nil, err
	}
	return it, nil
}

func (s *StateMachine) RevIterator(key []byte) (lib.IteratorI, lib.ErrorI) {
	store := s.Store()
	it, err := store.RevIterator(key)
	if err != nil {
		return nil, err
	}
	return it, nil
}
