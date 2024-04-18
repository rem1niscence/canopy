package fsm

import (
	"bytes"
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"time"
)

type StateMachine struct {
	BeginBlockParams *lib.BeginBlockParams
	ProtocolVersion  int
	MaxBlockBytes    uint64 // TODO parameterize
	NetworkID        uint32
	height           uint64
	store            lib.RWStoreI
}

func New(protocolVersion int, networkID uint32, store lib.StoreI) (*StateMachine, lib.ErrorI) {
	// TODO handle genesis state
	latestHeight := store.Version()
	lastHeightStore, err := store.NewReadOnly(latestHeight - 1)
	if err != nil {
		return nil, err
	}
	latestHeightFSM := StateMachine{store: lastHeightStore}
	validatorSet, err := latestHeightFSM.GetConsensusValidators()
	if err != nil {
		return nil, err
	}
	lastBlock, err := store.GetBlockByHeight(latestHeight - 1)
	if err != nil {
		return nil, err
	}
	return &StateMachine{
		BeginBlockParams: &lib.BeginBlockParams{
			BlockHeader:  lastBlock.BlockHeader,
			ValidatorSet: validatorSet,
		},
		ProtocolVersion: protocolVersion,
		NetworkID:       networkID,
		height:          latestHeight,
		store:           store,
	}, nil
}

func NewWithBeginBlock(bb *lib.BeginBlockParams, protocolVersion int, networkID uint32, height uint64, store lib.RWStoreI) *StateMachine {
	return &StateMachine{
		BeginBlockParams: bb,
		ProtocolVersion:  protocolVersion,
		NetworkID:        networkID,
		height:           height,
		store:            store,
	}
}

func (s *StateMachine) ApplyBlock(b *lib.Block) (*lib.BlockHeader, []*lib.TxResult, *lib.ValidatorSet, lib.ErrorI) {
	store, ok := s.Store().(lib.StoreI)
	if !ok {
		return nil, nil, nil, types.ErrWrongStoreType()
	}
	if err := s.BeginBlock(s.BeginBlockParams); err != nil {
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
		Height:                s.Height() + 1,
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
		ProposerAddress:       b.BlockHeader.ProposerAddress,
		LastDoubleSigners:     b.BlockHeader.LastDoubleSigners,
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
	lastVS, err := s.GetHistoricalValidatorSet(header.Height - 1)
	if err != nil {
		return err
	}
	vs, err := lib.NewValidatorSet(s.BeginBlockParams.ValidatorSet)
	if err != nil {
		return err
	}
	isPartialQC, err := header.LastQuorumCertificate.Check(&lib.View{Height: header.Height - 1}, vs)
	if err != nil {
		return err
	}
	if isPartialQC {
		return lib.ErrNoMaj23()
	}
	if err = s.validateBlockTime(header); err != nil {
		return err
	}
	if isCandidateBlock {
		if err = header.ValidateByzantineEvidence(vs, lastVS, evidence); err != nil {
			return err
		}
	}
	return nil
}

func (s *StateMachine) ApplyAndValidateBlock(b *lib.Block, evidence *lib.ByzantineEvidence, isCandidateBlock bool) (*lib.BlockResult, *lib.ValidatorSet, lib.ErrorI) {
	header, txResults, valSet, err := s.ApplyBlock(b)
	if err != nil {
		return nil, nil, err
	}
	if err = s.ValidateBlock(b.BlockHeader, header, evidence, isCandidateBlock); err != nil {
		return nil, nil, err
	}
	return &lib.BlockResult{
		BlockHeader:  b.BlockHeader,
		Transactions: txResults,
	}, valSet, nil
}

func (s *StateMachine) ApplyTransactions(block *lib.Block) (results []*lib.TxResult, root []byte, n int, er lib.ErrorI) {
	var items [][]byte
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
		items = append(items, bz)
		n++
	}
	root, _, err := lib.MerkleTree(items)
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
	newStore, err := store.NewReadOnly(height)
	if err != nil {
		return nil, err
	}
	return New(s.ProtocolVersion, s.NetworkID, newStore)
}

func (s *StateMachine) GetHistoricalValidatorSet(height uint64) (lib.ValidatorSetWrapper, lib.ErrorI) {
	fsm, err := s.TimeMachine(height - 1) // end block state is begin block of next height
	if err != nil {
		return lib.ValidatorSetWrapper{}, err
	}
	vs, err := fsm.GetConsensusValidators()
	if err != nil {
		return lib.ValidatorSetWrapper{}, err
	}
	return lib.NewValidatorSet(vs)
}

func (s *StateMachine) GetMaxValidators() (uint64, lib.ErrorI) {
	valParams, err := s.GetParamsVal()
	if err != nil {
		return 0, err
	}
	return valParams.ValidatorMaxCount.Value, nil
}

func (s *StateMachine) GetBlockAndCertificate(height uint64) (*lib.QuorumCertificate, lib.ErrorI) {
	store, ok := s.store.(lib.StoreI)
	if !ok {
		return nil, types.ErrWrongStoreType()
	}
	return store.GetQCByHeight(height)
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
		BeginBlockParams: s.BeginBlockParams,
		ProtocolVersion:  s.ProtocolVersion,
		NetworkID:        s.NetworkID,
		height:           s.height,
		store:            storeCopy,
	}, nil
}

func (s *StateMachine) LastBlockHeader() *lib.BlockHeader { return s.BeginBlockParams.BlockHeader }
func (s *StateMachine) Store() lib.RWStoreI               { return s.store }
func (s *StateMachine) SetStore(store lib.RWStoreI)       { s.store = store }
func (s *StateMachine) Height() uint64                    { return s.height }
func (s *StateMachine) SetHeight(height uint64)           { s.height = height }
func (s *StateMachine) Reset()                            { s.store.(lib.StoreI).Reset() }
func (s *StateMachine) validateBlockTime(header *lib.BlockHeader) lib.ErrorI {
	now := time.Now()
	t := header.Time.AsTime()
	minTime := now.Add(30 * time.Minute)
	maxTime := now.Add(30 * time.Minute)
	if minTime.Compare(t) > 0 || maxTime.Compare(t) < 0 {
		return lib.ErrInvalidBlockTime()
	}
	return nil
}
func (s *StateMachine) ResetToBeginBlock() {
	s.Reset()
	if err := s.BeginBlock(s.BeginBlockParams); err != nil {
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
