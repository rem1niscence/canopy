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

type DSE = lib.DoubleSignEvidences
type BPE = lib.BadProposerEvidences
type BE = lib.ByzantineEvidence

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

	ByzantineEvidence     *BE
	NextByzantineEvidence *BE

	PublicKey  crypto.PublicKeyI
	PrivateKey crypto.PrivateKeyI

	P2P    lib.P2P
	App    lib.App
	Config Config
	log    lib.LoggerI
}

func NewConsensusState(height uint64, v *lib.ValidatorSet, pk crypto.PrivateKeyI,
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
			partialQCs:          make(map[string]*QuorumCertificate),
		},
		PublicKey:  pk.PublicKey(),
		PrivateKey: pk,
		P2P:        p,
		App:        a,
		Config:     c,
		log:        l,
	}, nil
}

func (cs *ConsensusState) Start() {
	cs.NewHeight()
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
	cs.ProducersPublicKeys = cs.App.GetProducerPubKeys()
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

	// CHECK BYZANTINE EVIDENCE
	bpe, dse := BPE(nil), DSE(nil)
	if cs.ByzantineEvidence != nil {
		bpe = cs.ByzantineEvidence.BPE
		dse = cs.ByzantineEvidence.DSE
	}

	// SEND VOTE TO LEADER
	cs.SendToLeader(&Message{
		Qc: &QuorumCertificate{
			Header:          cs.View.Copy(),
			LeaderPublicKey: cs.LeaderPublicKey.Bytes(),
		},
		HighQc:                 cs.HighQC,
		LastDoubleSignEvidence: dse,
		BadProposerEvidence:    bpe,
	})
}

func (cs *ConsensusState) StartProposePhase() {
	cs.Phase = lib.Phase_PROPOSE
	vote, as, bitmap, err := cs.Votes.GetMaj23(cs.View.Copy())
	if err != nil {
		cs.log.Error(err.Error())
		return
	}
	// GATHER BYZANTINE EVIDENCE
	dse, bpe := DSE(vote.LastDoubleSignEvidence), BPE(vote.BadProposerEvidence)
	// PRODUCE BLOCK OR USE HQC BLOCK
	if vote.HighQc == nil {
		var lastDoubleSigners, badProposers [][]byte
		lastDoubleSigners, err = dse.GetDoubleSigners(cs.App)
		if err != nil {
			cs.log.Error(err.Error())
			return
		}
		badProposers, err = bpe.GetBadProposers(vote.Qc.LeaderPublicKey, cs.Height, cs.ValidatorSet)
		if err != nil {
			cs.log.Error(err.Error())
			return
		}
		cs.Block, err = cs.App.ProduceCandidateBlock(badProposers, lastDoubleSigners)
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
		Qc: &QuorumCertificate{
			Header:          vote.Qc.Header,
			Block:           cs.Block,
			LeaderPublicKey: vote.Qc.LeaderPublicKey,
			Signature: &lib.AggregateSignature{
				Signature: as,
				Bitmap:    bitmap,
			},
		},
		HighQc:                 cs.HighQC,
		LastDoubleSignEvidence: dse,
		BadProposerEvidence:    bpe,
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
	byzantineEvidence := BE{
		DSE: msg.LastDoubleSignEvidence,
		BPE: msg.BadProposerEvidence,
	}
	// CHECK CANDIDATE BLOCK AGAINST STATE MACHINE
	if err := cs.App.CheckCandidateBlock(msg.Qc.Block, &byzantineEvidence); err != nil {
		cs.log.Error(err.Error())
		return true
	}
	cs.Block = msg.Qc.Block
	cs.ByzantineEvidence = &byzantineEvidence
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
	cs.ByzantineEvidence = cs.LeaderMessages.GetByzantineEvidence(cs.App, cs.View.Copy(),
		cs.ValidatorSet, cs.LeaderPublicKey.Bytes(), cs.Votes, cs.SelfIsLeader())
	cs.NewHeight()
	return
}

func (cs *ConsensusState) CheckLeaderAndBlock(msg *Message) (interrupt bool) {
	// CONFIRM IS LEADER
	if !cs.IsLeader(msg.Signature.PublicKey) {
		cs.log.Error(lib.ErrInvalidLeaderPublicKey().Error())
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
		switch {
		case msg.IsReplicaMessage() || msg.IsPacemakerMessage():
			isPartialQC, err := msg.Check(cs.View.Copy(), cs.ValidatorSet)
			if err != nil {
				return err
			}
			if isPartialQC {
				return lib.ErrNoMaj23()
			}
			return cs.Votes.AddVote(cs.App, cs.LeaderPublicKey.Bytes(), cs.View.Copy(), msg, cs.ValidatorSet)
		case msg.IsLeaderMessage():
			partialQC, err := msg.Check(cs.View.Copy(), cs.ValidatorSet)
			if err != nil {
				return err
			}
			if partialQC {
				return cs.LeaderMessages.AddIncompleteQC(msg)
			}
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
	cs.NewRound()
	cs.Pacemaker()
}

func (cs *ConsensusState) NewRound() {
	cs.Round++
	cs.Phase = lib.Phase_ELECTION
	cs.Votes.NewRound(cs.Round)
	cs.LeaderMessages.NewRound(cs.Round, cs.ValidatorSet.Key)
}
func (cs *ConsensusState) NewHeight() {
	cs.Height++
	cs.Round = -1
	cs.HighQC = nil
	cs.LeaderPublicKey = nil
	cs.Block = nil
	cs.Votes.NewHeight()
	cs.LeaderMessages.NewHeight()
	cs.NewRound()
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

func (cs *ConsensusState) SendToReplicas(msg lib.Signable) {
	if err := msg.Sign(cs.PrivateKey); err != nil {
		cs.log.Error(err.Error())
		return
	}
	if err := cs.P2P.SendToValidators(msg); err != nil {
		cs.log.Error(err.Error())
		return
	}
}

func (cs *ConsensusState) SendToLeader(msg lib.Signable) {
	if err := msg.Sign(cs.PrivateKey); err != nil {
		cs.log.Error(err.Error())
		return
	}
	if err := cs.P2P.SendToOne(cs.LeaderPublicKey.Bytes(), msg); err != nil {
		cs.log.Error(err.Error())
		return
	}
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
