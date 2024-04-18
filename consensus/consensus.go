package consensus

import (
	"bytes"
	"github.com/ginchuco/ginchu/fsm"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"github.com/ginchuco/ginchu/p2p"
	"github.com/ginchuco/ginchu/store"
	"google.golang.org/protobuf/proto"
	"math"
	"time"
)

type DSE = lib.DoubleSignEvidences
type BPE = lib.BadProposerEvidences
type BE = lib.ByzantineEvidence

type State struct {
	lib.View

	LeaderPublicKey crypto.PublicKeyI

	LastValidatorSet    lib.ValidatorSetWrapper // TODO repopulate
	ValidatorSet        lib.ValidatorSetWrapper
	Votes               HeightVoteSet
	LeaderMessages      HeightLeaderMessages
	HighQC              *QuorumCertificate
	Locked              bool
	Block               *lib.Block
	ProducersPublicKeys [][]byte

	ByzantineEvidence     *BE
	NextByzantineEvidence *BE

	PublicKey  crypto.PublicKeyI
	PrivateKey crypto.PrivateKeyI

	P2P    lib.P2P
	Config lib.ConsensusConfig
	log    lib.LoggerI

	FSM     *fsm.StateMachine
	Mempool *Mempool
}

func New(pk crypto.PrivateKeyI, c lib.Config, l lib.LoggerI) (*State, lib.ErrorI) {
	db, err := store.New(c, l)
	if err != nil {
		return nil, err
	}
	sm, err := fsm.New(c.ProtocolVersion, c.NetworkID, db)
	if err != nil {
		return nil, err
	}
	fsmCopy, err := sm.Copy()
	if err != nil {
		return nil, err
	}
	height := sm.Height()
	valSet, err := sm.GetHistoricalValidatorSet(height)
	if err != nil {
		return nil, err
	}
	lastVS, err := sm.GetHistoricalValidatorSet(height - 1)
	if err != nil {
		return nil, err
	}
	maxVals, err := sm.GetMaxValidators()
	if err != nil {
		return nil, err
	}
	return &State{
		View:             lib.View{Height: height},
		LastValidatorSet: lastVS,
		ValidatorSet:     valSet,
		Votes:            NewHeightVoteSet(),
		LeaderMessages:   NewHeightLeaderMessages(),
		PublicKey:        pk.PublicKey(),
		PrivateKey:       pk,
		P2P:              p2p.New(pk, maxVals, c, l),
		Config:           c.ConsensusConfig,
		log:              l,
		FSM:              sm,
		Mempool:          NewMempool(fsmCopy, c.MempoolConfig, l),
	}, nil
}

func (s *State) Start() {
	s.NewHeight()
	for {
		now := time.Now()
		timer := &now
		s.StartElectionPhase()
		s.ResetAndSleep(timer)
		s.StartElectionVotePhase()
		s.ResetAndSleep(timer)
		s.StartProposePhase()
		s.ResetAndSleep(timer)
		if interrupt := s.StartProposeVotePhase(); interrupt {
			s.RoundInterrupt(timer)
			continue
		}
		s.ResetAndSleep(timer)
		s.StartPrecommitPhase()
		s.ResetAndSleep(timer)
		if interrupt := s.StartPrecommitVotePhase(); interrupt {
			s.RoundInterrupt(timer)
			continue
		}
		s.ResetAndSleep(timer)
		s.StartCommitPhase()
		s.ResetAndSleep(timer)
		if interrupt := s.StartCommitProcessPhase(); interrupt {
			s.RoundInterrupt(timer)
			continue
		}
	}
}

func (s *State) StartElectionPhase() {
	s.Phase = lib.Phase_ELECTION
	s.LeaderPublicKey = nil
	selfValidator, err := s.ValidatorSet.GetValidator(s.PublicKey.Bytes())
	if err != nil {
		s.log.Error(err.Error())
		return
	}
	s.ProducersPublicKeys = s.getProducerKeys()
	// SORTITION (CDF + VRF)
	_, vrf, isCandidate := Sortition(&SortitionParams{
		SortitionData: SortitionData{
			LastProducersPublicKeys: s.ProducersPublicKeys,
			Height:                  s.Height,
			Round:                   s.Round,
			TotalValidators:         s.ValidatorSet.NumValidators,
			VotingPower:             selfValidator.VotingPower,
			TotalPower:              s.ValidatorSet.TotalPower,
		},
		PrivateKey: s.PrivateKey,
	})
	if isCandidate {
		s.SendToReplicas(&Message{
			Header: s.View.Copy(),
			Vrf:    vrf,
		})
	}
}

func (s *State) StartElectionVotePhase() {
	s.Phase = lib.Phase_ELECTION_VOTE
	sortitionData := SortitionData{
		LastProducersPublicKeys: s.ProducersPublicKeys,
		Height:                  s.Height,
		Round:                   s.Round,
		TotalValidators:         s.ValidatorSet.NumValidators,
		TotalPower:              s.ValidatorSet.TotalPower,
	}
	electionMessages := s.LeaderMessages.GetElectionMessages(s.View.Round)

	// LEADER SELECTION
	candidates, err := electionMessages.GetCandidates(s.ValidatorSet, sortitionData)
	if err != nil {
		s.log.Error(err.Error())
	}
	s.LeaderPublicKey = SelectLeaderFromCandidates(candidates, sortitionData, s.ValidatorSet.ValidatorSet)

	// CHECK BYZANTINE EVIDENCE
	bpe, dse := BPE(nil), DSE(nil)
	if s.ByzantineEvidence != nil {
		bpe = s.ByzantineEvidence.BPE
		dse = s.ByzantineEvidence.DSE
	}

	// SEND VOTE TO LEADER
	s.SendToLeader(&Message{
		Qc: &QuorumCertificate{
			Header:          s.View.Copy(),
			LeaderPublicKey: s.LeaderPublicKey.Bytes(),
		},
		HighQc:                 s.HighQC,
		LastDoubleSignEvidence: dse,
		BadProposerEvidence:    bpe,
	})
}

func (s *State) StartProposePhase() {
	s.Phase = lib.Phase_PROPOSE
	vote, as, bitmap, err := s.Votes.GetMaj23(s.View.Copy())
	if err != nil {
		s.log.Error(err.Error())
		return
	}
	// GATHER BYZANTINE EVIDENCE
	dse, bpe := DSE(vote.LastDoubleSignEvidence), BPE(vote.BadProposerEvidence)
	// PRODUCE BLOCK OR USE HQC BLOCK
	if vote.HighQc == nil {
		var lastDoubleSigners, badProposers [][]byte
		lastDoubleSigners, err = dse.GetDoubleSigners(s.Height, s.ValidatorSet, s.LastValidatorSet)
		if err != nil {
			s.log.Error(err.Error())
			return
		}
		badProposers, err = bpe.GetBadProposers(vote.Qc.LeaderPublicKey, s.Height, s.ValidatorSet)
		if err != nil {
			s.log.Error(err.Error())
			return
		}
		s.Block, err = s.ProduceCandidateBlock(badProposers, lastDoubleSigners)
		if err != nil {
			s.log.Error(err.Error())
			return
		}
	} else {
		s.HighQC = vote.HighQc
		s.Block = s.HighQC.Block
	}
	// SEND MSG TO REPLICAS
	s.SendToReplicas(&Message{
		Header: s.Copy(),
		Qc: &QuorumCertificate{
			Header:          vote.Qc.Header,
			Block:           s.Block,
			LeaderPublicKey: vote.Qc.LeaderPublicKey,
			Signature: &lib.AggregateSignature{
				Signature: as,
				Bitmap:    bitmap,
			},
		},
		HighQc:                 s.HighQC,
		LastDoubleSignEvidence: dse,
		BadProposerEvidence:    bpe,
	})
}

func (s *State) StartProposeVotePhase() (interrupt bool) {
	s.Phase = lib.Phase_PROPOSE_VOTE
	msg := s.LeaderMessages.GetLeaderMessage(s.View.Copy())
	s.LeaderPublicKey, _ = lib.PublicKeyFromBytes(msg.Signature.PublicKey)
	// IF LOCKED, CONFIRM SAFE TO UNLOCK
	if s.Locked {
		if err := s.SafeNode(msg.HighQc, msg.Qc.Block); err != nil {
			s.log.Error(err.Error())
			return true
		}
	}
	byzantineEvidence := BE{
		DSE: msg.LastDoubleSignEvidence,
		BPE: msg.BadProposerEvidence,
	}
	// CHECK CANDIDATE BLOCK AGAINST STATE MACHINE
	if err := s.CheckCandidateBlock(msg.Qc.Block, &byzantineEvidence); err != nil {
		s.log.Error(err.Error())
		return true
	}
	s.Block = msg.Qc.Block
	s.ByzantineEvidence = &byzantineEvidence
	// SEND VOTE TO LEADER
	s.SendToLeader(&Message{
		Qc: &QuorumCertificate{
			Header: s.View.Copy(),
			Block:  s.Block},
	})
	return
}

func (s *State) StartPrecommitPhase() {
	s.Phase = lib.Phase_PRECOMMIT
	if !s.SelfIsLeader() {
		return
	}
	vote, as, bitmap, err := s.Votes.GetMaj23(s.View.Copy())
	if err != nil {
		s.log.Error(err.Error())
		return
	}
	s.SendToReplicas(&Message{
		Header: s.Copy(),
		Qc: &QuorumCertificate{
			Header: vote.Qc.Header,
			Block:  vote.Qc.Block,
			Signature: &lib.AggregateSignature{
				Signature: as,
				Bitmap:    bitmap,
			},
		},
	})
}

func (s *State) StartPrecommitVotePhase() (interrupt bool) {
	s.Phase = lib.Phase_PRECOMMIT_VOTE
	msg := s.LeaderMessages.GetLeaderMessage(s.View.Copy())
	if interrupt = s.CheckLeaderAndBlock(msg); interrupt {
		return
	}
	// LOCK AND SET HIGH-QC TO PROTECT THOSE WHO MAY COMMIT
	s.HighQC = msg.Qc
	s.Locked = true
	// SEND VOTE TO LEADER
	s.SendToLeader(&Message{
		Qc: &QuorumCertificate{
			Header: s.View.Copy(),
			Block:  s.Block,
		},
	})
	return
}

func (s *State) StartCommitPhase() {
	s.Phase = lib.Phase_COMMIT
	if !s.SelfIsLeader() {
		return
	}
	vote, as, bitmap, err := s.Votes.GetMaj23(s.View.Copy())
	if err != nil {
		s.log.Error(err.Error())
		return
	}
	// SEND MSG TO REPLICAS
	s.SendToReplicas(&Message{
		Header: s.Copy(), // header
		Qc: &QuorumCertificate{
			Header: vote.Header,   // vote view
			Block:  vote.Qc.Block, // vote block
			Signature: &lib.AggregateSignature{
				Signature: as,
				Bitmap:    bitmap,
			},
		},
	})
}

func (s *State) StartCommitProcessPhase() (interrupt bool) {
	s.Phase = lib.Phase_COMMIT_PROCESS
	msg := s.LeaderMessages.GetLeaderMessage(s.View.Copy())
	// CONFIRM LEADER & BLOCK
	if interrupt = s.CheckLeaderAndBlock(msg); interrupt {
		return
	}
	if err := s.CommitBlock(msg.Qc); err != nil {
		return true
	}
	s.ByzantineEvidence = s.LeaderMessages.GetByzantineEvidence(s.View.Copy(),
		s.ValidatorSet, s.LastValidatorSet, s.LeaderPublicKey.Bytes(), &s.Votes, s.SelfIsLeader())
	s.NewHeight()
	return
}

func (s *State) CheckLeaderAndBlock(msg *Message) (interrupt bool) {
	// CONFIRM IS LEADER
	if !s.IsLeader(msg.Signature.PublicKey) {
		s.log.Error(lib.ErrInvalidLeaderPublicKey().Error())
		return true
	}

	// CONFIRM BLOCK
	if !s.Block.Equals(msg.Qc.Block) {
		s.log.Error(ErrMismatchBlocks().Error())
		return true
	}
	return
}

func (s *State) HandleMessage(message proto.Message) lib.ErrorI {
	switch msg := message.(type) {
	case *Message:
		switch {
		case msg.IsReplicaMessage() || msg.IsPacemakerMessage():
			isPartialQC, err := msg.Check(s.View.Copy(), s.ValidatorSet)
			if err != nil {
				return err
			}
			if isPartialQC {
				return lib.ErrNoMaj23()
			}
			return s.Votes.AddVote(s.LeaderPublicKey.Bytes(), s.View.Copy(), msg, s.ValidatorSet, s.LastValidatorSet)
		case msg.IsLeaderMessage():
			partialQC, err := msg.Check(s.View.Copy(), s.ValidatorSet)
			if err != nil {
				return err
			}
			if partialQC {
				return s.LeaderMessages.AddIncompleteQC(msg)
			}
			return s.LeaderMessages.AddLeaderMessage(msg)
		}
	}
	return ErrUnknownConsensusMsg(message)
}

func (s *State) RoundInterrupt(timer *time.Time) {
	s.Phase = lib.Phase_ROUND_INTERRUPT
	// send pacemaker message
	s.SendToLeader(&Message{
		Header: s.View.Copy(),
	})
	s.ResetAndSleep(timer)
	s.NewRound(false)
	s.Pacemaker()
}

func (s *State) NewRound(newHeight bool) {
	if !newHeight {
		s.Round++
	}
	s.Phase = lib.Phase_ELECTION
	s.Votes.NewRound(s.Round)
	s.LeaderMessages.NewRound(s.Round, s.ValidatorSet.Key)
}
func (s *State) NewHeight() {
	s.Height++
	s.Round = 0
	s.HighQC = nil
	s.LeaderPublicKey = nil
	s.Block = nil
	s.Votes.NewHeight()
	s.LeaderMessages.NewHeight()
	s.NewRound(true)
}

func (s *State) Pacemaker() {
	highestQuorumRoundFromPeers := s.Votes.Pacemaker()
	if highestQuorumRoundFromPeers > s.Round {
		s.Round = highestQuorumRoundFromPeers
	}
}

func (s *State) SafeNode(qc *QuorumCertificate, b *lib.Block) lib.ErrorI {
	block, view := qc.Block, qc.Header
	if bytes.Equal(b.BlockHeader.Hash, block.BlockHeader.Hash) {
		return ErrMismatchBlocks()
	}
	lockedBlock, locked := s.HighQC.Block, s.HighQC.Header
	if bytes.Equal(lockedBlock.BlockHeader.Hash, block.BlockHeader.Hash) {
		return nil // SAFETY
	}
	if view.Round > locked.Round {
		return nil // LIVENESS
	}
	return ErrFailedSafeNodePredicate()
}

func (s *State) ResetAndSleep(startTime *time.Time) {
	processingTime := time.Since(*startTime)
	var sleepTime time.Duration
	switch s.Phase {
	case lib.Phase_ELECTION:
		sleepTime = s.SleepTime(s.Config.ElectionTimeoutMS)
	case lib.Phase_ELECTION_VOTE:
		sleepTime = s.SleepTime(s.Config.ElectionVoteTimeoutMS)
	case lib.Phase_PROPOSE:
		sleepTime = s.SleepTime(s.Config.ProposeTimeoutMS)
	case lib.Phase_PROPOSE_VOTE:
		sleepTime = s.SleepTime(s.Config.ProposeVoteTimeoutMS)
	case lib.Phase_PRECOMMIT:
		sleepTime = s.SleepTime(s.Config.PrecommitTimeoutMS)
	case lib.Phase_PRECOMMIT_VOTE:
		sleepTime = s.SleepTime(s.Config.PrecommitVoteTimeoutMS)
	case lib.Phase_COMMIT:
		sleepTime = s.SleepTime(s.Config.CommitTimeoutMS)
	case lib.Phase_COMMIT_PROCESS:
		sleepTime = s.SleepTime(s.Config.CommitProcessMS)
	}
	if sleepTime > processingTime {
		time.Sleep(sleepTime - processingTime)
	}
	*startTime = time.Now()
}

func (s *State) SleepTime(sleepTimeMS int) time.Duration {
	return time.Duration(math.Pow(float64(sleepTimeMS), float64(s.Round+1)) * float64(time.Millisecond))
}

func (s *State) SelfIsLeader() bool {
	return s.IsLeader(s.PublicKey.Bytes())
}

func (s *State) IsLeader(sender []byte) bool {
	return bytes.Equal(sender, s.LeaderPublicKey.Bytes())
}

func (s *State) SendToReplicas(msg lib.Signable) {
	if err := msg.Sign(s.PrivateKey); err != nil {
		s.log.Error(err.Error())
		return
	}
	if err := s.P2P.SendToValidators(msg); err != nil {
		s.log.Error(err.Error())
		return
	}
}

func (s *State) SendToLeader(msg lib.Signable) {
	if err := msg.Sign(s.PrivateKey); err != nil {
		s.log.Error(err.Error())
		return
	}
	if err := s.P2P.SendTo(s.LeaderPublicKey.Bytes(), lib.Topic_CONSENSUS, msg); err != nil {
		s.log.Error(err.Error())
		return
	}
}

func (s *State) getProducerKeys() [][]byte {
	keys, err := s.FSM.GetProducerKeys()
	if err != nil {
		return nil
	}
	return keys.ProducerKeys
}
