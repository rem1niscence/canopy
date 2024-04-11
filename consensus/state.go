package consensus

import (
	"bytes"
	"github.com/ginchuco/ginchu/crypto"
	lib "github.com/ginchuco/ginchu/types"
	"google.golang.org/protobuf/proto"
	"math"
	"sync"
	"time"
)

type ConsensusState struct {
	lib.View

	LeaderPublicKey crypto.PublicKeyI

	ValidatorSet        lib.ValidatorSetWrapper
	Votes               HeightVoteSet
	LeaderMessages      HeightLeaderMessages
	HighQC              *QuorumCertificate
	Locked              bool
	Block               *lib.Block
	ProducersPublicKeys [][]byte

	BadProposers [][]byte

	PublicKey  crypto.PublicKeyI
	PrivateKey crypto.PrivateKeyI

	P2P    lib.P2P
	App    lib.App
	Config Config
	log    lib.LoggerI
}

func NewConsensusState(height uint64, producersKeys [][]byte, v *lib.ValidatorSet, pk crypto.PrivateKeyI,
	p lib.P2P, a lib.App, c Config, l lib.LoggerI) (*ConsensusState, lib.ErrorI) {
	validatorSet, err := lib.NewValidatorSet(v)
	if err != nil {
		return nil, err
	}
	return &ConsensusState{
		View:         lib.View{Height: height},
		ValidatorSet: validatorSet,
		Votes: HeightVoteSet{
			Mutex:          sync.Mutex{},
			roundVoteSets:  make(map[uint64]RoundVoteSet),
			pacemakerVotes: make([]VoteSet, maxRounds),
		},
		LeaderMessages: HeightLeaderMessages{
			Mutex:               sync.Mutex{},
			roundLeaderMessages: make(map[uint64]RoundLeaderMessages),
		},
		ProducersPublicKeys: producersKeys,
		PublicKey:           pk.PublicKey(),
		PrivateKey:          pk,
		P2P:                 p,
		App:                 a,
		Config:              c,
		log:                 l,
	}, nil
}

func (cs *ConsensusState) Start() {
	for {
		now := time.Now()
		timer := &now
		cs.StartElectionPhase()
		cs.ResetAndSleep(timer)
		cs.StartElectionVotePhase()
		cs.ResetAndSleep(timer)
		cs.StartProposePhase()
		cs.ResetAndSleep(timer)
		if interrupt := cs.StartProposeVotePhase(); interrupt {
			cs.RoundInterrupt(timer)
			continue
		}
		cs.ResetAndSleep(timer)
		cs.StartPrecommitPhase()
		cs.ResetAndSleep(timer)
		if interrupt := cs.StartPrecommitVotePhase(); interrupt {
			cs.RoundInterrupt(timer)
			continue
		}
		cs.ResetAndSleep(timer)
		cs.StartCommitPhase()
		cs.ResetAndSleep(timer)
		if interrupt := cs.StartCommitProcessPhase(); interrupt {
			cs.RoundInterrupt(timer)
			continue
		}
	}
}

func (cs *ConsensusState) StartElectionPhase() {
	cs.Phase = lib.Phase_ELECTION
	cs.LeaderPublicKey = nil
	selfValidator, err := cs.ValidatorSet.GetValidator(cs.PublicKey.Bytes())
	if err != nil {
		cs.log.Error(err.Error())
		return
	}
	// SORTITION (CDF + VRF)
	_, vrf, isCandidate := Sortition(&SortitionParams{
		SortitionData: SortitionData{
			LastProducersPublicKeys: cs.ProducersPublicKeys,
			Height:                  cs.Height,
			Round:                   cs.Round,
			TotalValidators:         cs.ValidatorSet.NumValidators,
			VotingPower:             selfValidator.VotingPower,
			TotalPower:              cs.ValidatorSet.TotalPower,
		},
		PrivateKey: cs.PrivateKey,
	})
	if isCandidate {
		cs.SendToReplicas(&Message{
			Header: cs.View.Copy(),
			Vrf:    vrf,
		})
	}
}

func (cs *ConsensusState) StartElectionVotePhase() {
	cs.Phase = lib.Phase_ELECTION_VOTE
	sortitionData := SortitionData{
		LastProducersPublicKeys: cs.ProducersPublicKeys,
		Height:                  cs.Height,
		Round:                   cs.Round,
		TotalValidators:         cs.ValidatorSet.NumValidators,
		TotalPower:              cs.ValidatorSet.TotalPower,
	}
	electionMessages := cs.LeaderMessages.GetElectionMessages(cs.View.Round)

	// LEADER SELECTION
	candidates, err := electionMessages.GetCandidates(cs.ValidatorSet, sortitionData)
	if err != nil {
		cs.log.Error(err.Error())
	}
	cs.LeaderPublicKey = SelectLeaderFromCandidates(candidates, sortitionData, cs.ValidatorSet.ValidatorSet)

	// SEND VOTE TO LEADER
	cs.SendToLeader(&Message{
		Qc: &QuorumCertificate{
			Header:          cs.View.Copy(),
			LeaderPublicKey: cs.LeaderPublicKey.Bytes(),
		},
		HighQc: cs.HighQC,
	})
}

func (cs *ConsensusState) StartProposePhase() {
	cs.Phase = lib.Phase_PROPOSE
	vote, as, bitmap, err := cs.Votes.GetMaj23(cs.View.Copy())
	if err != nil {
		cs.log.Error(err.Error())
		return
	}
	// PRODUCE BLOCK OR USE HQC BLOCK
	if vote.HighQc == nil {
		var doubleSigners [][]byte
		doubleSigners, err = lib.Evidence(doubleSignEvidence).GetDoubleSigners(cs.App)
		if err != nil {
			cs.log.Error(err.Error())
			return
		}
		cs.Block, err = cs.App.ProduceCandidateBlock(cs.BadProposers, doubleSigners)
		if err != nil {
			cs.log.Error(err.Error())
			return
		}
	} else {
		cs.HighQC = vote.HighQc
		cs.Block = cs.HighQC.Block
	}
	// SEND MSG TO REPLICAS
	cs.SendToReplicas(&Message{
		Header: cs.Copy(),
		HighQc: cs.HighQC,
		Qc: &QuorumCertificate{
			Header:          vote.Qc.Header,
			Block:           cs.Block,
			LeaderPublicKey: vote.Qc.LeaderPublicKey,
			Signature: &lib.AggregateSignature{
				Signature: as,
				Bitmap:    bitmap,
			},
		},
	})
}

func (cs *ConsensusState) StartProposeVotePhase() (interrupt bool) {
	cs.Phase = lib.Phase_PROPOSE_VOTE
	msg := cs.LeaderMessages.GetLeaderMessage(cs.View.Copy())
	cs.LeaderPublicKey, _ = lib.PublicKeyFromBytes(msg.Signature.PublicKey)
	// IF LOCKED, CONFIRM SAFE TO UNLOCK
	if cs.Locked {
		if err := cs.SafeNode(msg.HighQc, msg.Qc.Block); err != nil {
			cs.log.Error(err.Error())
			return true
		}
	}
	// CHECK CANDIDATE BLOCK AGAINST STATE MACHINE
	if err := cs.App.CheckCandidateBlock(msg.Qc.Block); err != nil {
		cs.log.Error(err.Error())
		return true
	}
	cs.Block = msg.Qc.Block
	// SEND VOTE TO LEADER
	cs.SendToLeader(&Message{
		Qc: &QuorumCertificate{
			Header: cs.View.Copy(),
			Block:  cs.Block},
	})
	return
}

func (cs *ConsensusState) StartPrecommitPhase() {
	cs.Phase = lib.Phase_PRECOMMIT
	if !cs.SelfIsLeader() {
		return
	}
	vote, as, bitmap, err := cs.Votes.GetMaj23(cs.View.Copy())
	if err != nil {
		cs.log.Error(err.Error())
		return
	}
	cs.SendToReplicas(&Message{
		Header: cs.Copy(),
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

func (cs *ConsensusState) StartPrecommitVotePhase() (interrupt bool) {
	cs.Phase = lib.Phase_PRECOMMIT_VOTE
	msg := cs.LeaderMessages.GetLeaderMessage(cs.View.Copy())
	if interrupt = cs.CheckLeaderAndBlock(msg); interrupt {
		return
	}
	// LOCK AND SET HIGH-QC TO PROTECT THOSE WHO MAY COMMIT
	cs.HighQC = msg.Qc
	cs.Locked = true
	// SEND VOTE TO LEADER
	cs.SendToLeader(&Message{
		Qc: &QuorumCertificate{
			Header: cs.View.Copy(),
			Block:  cs.Block,
		},
	})
	return
}

func (cs *ConsensusState) StartCommitPhase() {
	cs.Phase = lib.Phase_COMMIT
	if !cs.SelfIsLeader() {
		return
	}
	vote, as, bitmap, err := cs.Votes.GetMaj23(cs.View.Copy())
	if err != nil {
		cs.log.Error(err.Error())
		return
	}
	// SEND MSG TO REPLICAS
	cs.SendToReplicas(&Message{
		Header: cs.Copy(), // header
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

func (cs *ConsensusState) StartCommitProcessPhase() (interrupt bool) {
	cs.Phase = lib.Phase_COMMIT_PROCESS
	msg := cs.LeaderMessages.GetLeaderMessage(cs.View.Copy())
	// CONFIRM LEADER & BLOCK
	if interrupt = cs.CheckLeaderAndBlock(msg); interrupt {
		return
	}
	if err := cs.App.CommitBlock(msg.Qc); err != nil {
		return true
	}
	return
}

func (cs *ConsensusState) CheckLeaderAndBlock(msg *Message) (interrupt bool) {
	// CONFIRM IS LEADER
	if !cs.IsLeader(msg.Signature.PublicKey) {
		cs.log.Error(ErrInvalidLeaderPublicKey().Error())
		return true
	}

	// CONFIRM BLOCK
	if !cs.Block.Equals(msg.Qc.Block) {
		cs.log.Error(ErrMismatchBlocks().Error())
		return true
	}
	return
}

func (cs *ConsensusState) HandleMessage(message proto.Message) lib.ErrorI {
	switch msg := message.(type) {
	case *Message:
		if err := msg.Check(cs.View.Copy(), cs.ValidatorSet); err != nil {
			return err
		}
		switch {
		case msg.IsReplicaMessage() || msg.IsPacemakerMessage():
			return cs.Votes.AddVote(cs.App, cs.View.Copy(), msg, cs.ValidatorSet)
		case msg.IsLeaderMessage():
			return cs.LeaderMessages.AddLeaderMessage(msg)
		}
	}
	return ErrUnknownConsensusMsg(message)
}

func (cs *ConsensusState) RoundInterrupt(timer *time.Time) {
	cs.Phase = lib.Phase_ROUND_INTERRUPT
	// send pacemaker message
	cs.SendToLeader(&Message{
		Header: cs.View.Copy(),
	})
	cs.ResetAndSleep(timer)
	cs.Round++
	cs.Phase = lib.Phase_ELECTION
	cs.Pacemaker()
}

func (cs *ConsensusState) Pacemaker() {
	highestQuorumRoundFromPeers := cs.Votes.Pacemaker()
	if highestQuorumRoundFromPeers > cs.Round {
		cs.Round = highestQuorumRoundFromPeers
	}
}

func (cs *ConsensusState) SafeNode(qc *QuorumCertificate, b *lib.Block) lib.ErrorI {
	block, view := qc.Block, qc.Header
	if bytes.Equal(b.BlockHeader.Hash, block.BlockHeader.Hash) {
		return ErrMismatchBlocks()
	}
	lockedBlock, locked := cs.HighQC.Block, cs.HighQC.Header
	if bytes.Equal(lockedBlock.BlockHeader.Hash, block.BlockHeader.Hash) {
		return nil // SAFETY
	}
	if view.Round > locked.Round {
		return nil // LIVENESS
	}
	return ErrFailedSafeNodePredicate()
}

func (cs *ConsensusState) ResetAndSleep(startTime *time.Time) {
	processingTime := time.Since(*startTime)
	var sleepTime time.Duration
	switch cs.Phase {
	case lib.Phase_ELECTION:
		sleepTime = cs.SleepTime(cs.Config.ElectionTimeoutMS)
	case lib.Phase_ELECTION_VOTE:
		sleepTime = cs.SleepTime(cs.Config.ElectionVoteTimeoutMS)
	case lib.Phase_PROPOSE:
		sleepTime = cs.SleepTime(cs.Config.ProposeTimeoutMS)
	case lib.Phase_PROPOSE_VOTE:
		sleepTime = cs.SleepTime(cs.Config.ProposeVoteTimeoutMS)
	case lib.Phase_PRECOMMIT:
		sleepTime = cs.SleepTime(cs.Config.PrecommitTimeoutMS)
	case lib.Phase_PRECOMMIT_VOTE:
		sleepTime = cs.SleepTime(cs.Config.PrecommitVoteTimeoutMS)
	case lib.Phase_COMMIT:
		sleepTime = cs.SleepTime(cs.Config.CommitTimeoutMS)
	case lib.Phase_COMMIT_PROCESS:
		sleepTime = cs.SleepTime(cs.Config.CommitProcessMS)
	}
	if sleepTime > processingTime {
		time.Sleep(sleepTime - processingTime)
	}
	*startTime = time.Now()
}

func (cs *ConsensusState) SleepTime(sleepTimeMS int) time.Duration {
	return time.Duration(math.Pow(float64(sleepTimeMS), float64(cs.Round+1)) * float64(time.Millisecond))
}

func (cs *ConsensusState) SelfIsLeader() bool {
	return cs.IsLeader(cs.PublicKey.Bytes())
}

func (cs *ConsensusState) IsLeader(sender []byte) bool {
	return bytes.Equal(sender, cs.LeaderPublicKey.Bytes())
}

type Config struct {
	ElectionTimeoutMS       int
	ElectionVoteTimeoutMS   int
	ProposeTimeoutMS        int
	ProposeVoteTimeoutMS    int
	PrecommitTimeoutMS      int
	PrecommitVoteTimeoutMS  int
	CommitTimeoutMS         int
	CommitProcessMS         int // majority of block time
	RoundInterruptTimeoutMS int
}
