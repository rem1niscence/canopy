package fsm

import (
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"runtime/debug"
)

// TODO unstaking limitations and force unstake for slash below minimum stake

type StateMachine struct {
	Config            lib.Config
	ProtocolVersion   int
	NetworkID         uint32
	height            uint64
	proposeVoteConfig types.GovProposalVoteConfig
	store             lib.RWStoreI
	log               lib.LoggerI
}

func New(c lib.Config, store lib.StoreI, log lib.LoggerI) (*StateMachine, lib.ErrorI) {
	sm := &StateMachine{
		Config:            c,
		ProtocolVersion:   c.ProtocolVersion,
		NetworkID:         c.NetworkID,
		proposeVoteConfig: types.AcceptAllProposals,
		log:               log,
	}
	return sm, sm.Initialize(store)
}

func (s *StateMachine) Initialize(db lib.StoreI) (error lib.ErrorI) {
	s.height, s.store = db.Version(), db
	if s.height == 0 {
		return s.NewFromGenesisFile()
	}
	return nil
}

func (s *StateMachine) catchPanic() {
	if r := recover(); r != nil {
		s.log.Error(string(debug.Stack()))
	}
}

func (s *StateMachine) ApplyBlock(b *lib.Block) (*lib.BlockHeader, []*lib.TxResult, *lib.ConsensusValidators, lib.ErrorI) {
	defer s.catchPanic()
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
	if len(txRoot) == 0 {
		txRoot = lib.MaxHash
	}
	if err = s.EndBlock(); err != nil {
		return nil, nil, nil, err
	}
	lastValidatorSet, err := s.LoadValSet(s.Height() - 1)
	if err != nil {
		return nil, nil, nil, err
	}
	nextValidatorSet, err := s.LoadValSet(s.Height())
	if err != nil {
		return nil, nil, nil, err
	}
	lastValidatorRoot, err := lastValidatorSet.ValidatorSet.Root()
	if err != nil {
		return nil, nil, nil, err
	}
	nextValidatorRoot, err := nextValidatorSet.ValidatorSet.Root()
	if err != nil {
		return nil, nil, nil, err
	}
	stateRoot, err := store.Root()
	if err != nil {
		return nil, nil, nil, err
	}
	lastBlock, err := s.LoadBlock(s.height - 1)
	if err != nil {
		return nil, nil, nil, err
	}
	header := lib.BlockHeader{
		Height:                s.Height(),
		Hash:                  nil,
		NetworkId:             s.NetworkID,
		Time:                  b.BlockHeader.Time,
		NumTxs:                uint64(numTxs),
		TotalTxs:              lastBlock.BlockHeader.TotalTxs + uint64(numTxs),
		LastBlockHash:         lastBlock.BlockHeader.Hash,
		StateRoot:             stateRoot,
		TransactionRoot:       txRoot,
		ValidatorRoot:         lastValidatorRoot,
		NextValidatorRoot:     nextValidatorRoot,
		ProposerAddress:       b.BlockHeader.ProposerAddress,
		LastQuorumCertificate: b.BlockHeader.LastQuorumCertificate,
	}
	if _, err = header.SetHash(); err != nil {
		return nil, nil, nil, err
	}
	return &header, txResults, nextValidatorSet.ValidatorSet, nil
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
	heightStore, err := store.NewReadOnly(height)
	if err != nil {
		return nil, err
	}
	return New(s.Config, heightStore, s.log)
}

func (s *StateMachine) LoadCommittee(committeeID uint64, height uint64) (lib.ValidatorSet, lib.ErrorI) {
	if height <= 1 {
		height = 1 // 1 is first non-genesis height
	} else {
		height -= 1 // end block state is begin block of next height
	}
	fsm, err := s.TimeMachine(height)
	if err != nil {
		return lib.ValidatorSet{}, err
	}
	vs, err := fsm.GetCommittee(committeeID)
	if err != nil {
		return lib.ValidatorSet{}, err
	}
	return vs, nil
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

func (s *StateMachine) LoadCertificateWithProposal(height uint64) (*lib.QuorumCertificate, lib.ErrorI) {
	store, ok := s.store.(lib.StoreI)
	if !ok {
		return nil, types.ErrWrongStoreType()
	}
	return store.GetQCByHeight(height)
}

func (s *StateMachine) LoadBlock(height uint64) (*lib.BlockResult, lib.ErrorI) {
	store, ok := s.store.(lib.StoreI)
	if !ok {
		return nil, types.ErrWrongStoreType()
	}
	return store.GetBlockByHeight(height)
}

func (s *StateMachine) LoadCertificateWOProposal(height uint64) (*lib.QuorumCertificate, lib.ErrorI) {
	qc, err := s.LoadCertificateWithProposal(height)
	if err != nil {
		return nil, err
	}
	qc.Proposal = nil
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
		ProtocolVersion:   s.ProtocolVersion,
		NetworkID:         s.NetworkID,
		height:            s.height,
		proposeVoteConfig: s.proposeVoteConfig,
		store:             storeCopy,
		log:               s.log,
	}, nil
}
func (s *StateMachine) Store() lib.RWStoreI                                 { return s.store }
func (s *StateMachine) SetStore(store lib.RWStoreI)                         { s.store = store }
func (s *StateMachine) Height() uint64                                      { return s.height }
func (s *StateMachine) Reset()                                              { s.store.(lib.StoreI).Reset() }
func (s *StateMachine) SetProposalVoteConfig(c types.GovProposalVoteConfig) { s.proposeVoteConfig = c }
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
