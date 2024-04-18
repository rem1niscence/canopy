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
	"sync"
	"time"
)

type Consensus struct {
	lib.View

	LeaderPublicKey crypto.PublicKeyI

	LastValidatorSet lib.ValidatorSet
	ValidatorSet     lib.ValidatorSet
	Votes            HeightVoteSet
	LeaderMessages   HeightLeaderMessages
	HighQC           *QC
	Locked           bool
	Block            *lib.Block
	SortitionData    SortitionData

	ByzantineEvidence     *BE
	NextByzantineEvidence *BE

	PublicKey  crypto.PublicKeyI
	PrivateKey crypto.PrivateKeyI

	P2P    lib.P2P
	Config lib.ConsensusConfig
	log    lib.LoggerI

	FSM     *fsm.StateMachine
	Mempool *Mempool
	sync.Mutex
}

func New(pk crypto.PrivateKeyI, c lib.Config, l lib.LoggerI) (*Consensus, lib.ErrorI) {
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
	return &Consensus{
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
		Mutex:            sync.Mutex{},
	}, nil
}

func (c *Consensus) Start() {
	c.P2P.Start()
	c.StartListeners()
	c.Sync()
	c.NewHeight()
	for {
		now := time.Now()
		timer := &now
		c.StartElectionPhase()
		c.ResetAndSleep(timer)
		c.StartElectionVotePhase()
		c.ResetAndSleep(timer)
		c.StartProposePhase()
		c.ResetAndSleep(timer)
		if interrupt := c.StartProposeVotePhase(); interrupt {
			c.RoundInterrupt(timer)
			continue
		}
		c.ResetAndSleep(timer)
		c.StartPrecommitPhase()
		c.ResetAndSleep(timer)
		if interrupt := c.StartPrecommitVotePhase(); interrupt {
			c.RoundInterrupt(timer)
			continue
		}
		c.ResetAndSleep(timer)
		c.StartCommitPhase()
		c.ResetAndSleep(timer)
		if interrupt := c.StartCommitProcessPhase(); interrupt {
			c.RoundInterrupt(timer)
			continue
		}
	}
}

func (c *Consensus) StartElectionPhase() {
	c.Phase = lib.Phase_ELECTION
	c.LeaderPublicKey = nil
	selfValidator, err := c.ValidatorSet.GetValidator(c.PublicKey.Bytes())
	if err != nil {
		c.log.Error(err.Error())
		return
	}
	c.SortitionData = SortitionData{
		LastProducersPublicKeys: c.getProducerKeys(),
		Height:                  c.Height,
		Round:                   c.Round,
		TotalValidators:         c.ValidatorSet.NumValidators,
		VotingPower:             selfValidator.VotingPower,
		TotalPower:              c.ValidatorSet.TotalPower,
	}
	// SORTITION (CDF + VRF)
	_, vrf, isCandidate := Sortition(&SortitionParams{
		SortitionData: c.SortitionData,
		PrivateKey:    c.PrivateKey,
	})
	if isCandidate {
		c.SendToReplicas(&Message{
			Header: c.View.Copy(),
			Vrf:    vrf,
		})
	}
}

func (c *Consensus) StartElectionVotePhase() {
	c.Phase = lib.Phase_ELECTION_VOTE
	electionMessages := c.LeaderMessages.GetElectionMessages(c.View.Round)

	// LEADER SELECTION
	candidates, err := electionMessages.GetCandidates(c.ValidatorSet, c.SortitionData)
	if err != nil {
		c.log.Error(err.Error())
	}
	c.LeaderPublicKey = SelectLeaderFromCandidates(candidates, c.SortitionData, c.ValidatorSet.ValidatorSet)

	// CHECK BYZANTINE EVIDENCE
	bpe, dse := BPE(nil), DSE(nil)
	if c.ByzantineEvidence != nil {
		bpe = c.ByzantineEvidence.BPE
		dse = c.ByzantineEvidence.DSE
	}

	// SEND VOTE TO LEADER
	c.SendToLeader(&Message{
		Qc: &QC{
			Header:          c.View.Copy(),
			LeaderPublicKey: c.LeaderPublicKey.Bytes(),
		},
		HighQc:                 c.HighQC,
		LastDoubleSignEvidence: dse,
		BadProposerEvidence:    bpe,
	})
}

func (c *Consensus) StartProposePhase() {
	c.Phase = lib.Phase_PROPOSE
	vote, as, bitmap, err := c.Votes.GetMaj23(c.View.Copy())
	if err != nil {
		c.log.Error(err.Error())
		return
	}
	// GATHER BYZANTINE EVIDENCE
	dse, bpe := DSE(vote.LastDoubleSignEvidence), BPE(vote.BadProposerEvidence)
	// PRODUCE BLOCK OR USE HQC BLOCK
	if vote.HighQc == nil {
		var lastDoubleSigners, badProposers [][]byte
		lastDoubleSigners, err = dse.GetDoubleSigners(c.Height, c.ValidatorSet, c.LastValidatorSet)
		if err != nil {
			c.log.Error(err.Error())
			return
		}
		badProposers, err = bpe.GetBadProposers(vote.Qc.LeaderPublicKey, c.Height, c.ValidatorSet)
		if err != nil {
			c.log.Error(err.Error())
			return
		}
		c.Block, err = c.ProduceCandidateBlock(badProposers, lastDoubleSigners)
		if err != nil {
			c.log.Error(err.Error())
			return
		}
	} else {
		c.HighQC = vote.HighQc
		c.Block = c.HighQC.Block
	}
	// SEND MSG TO REPLICAS
	c.SendToReplicas(&Message{
		Header: c.Copy(),
		Qc: &QC{
			Header:          vote.Qc.Header,
			Block:           c.Block,
			LeaderPublicKey: vote.Qc.LeaderPublicKey,
			Signature: &lib.AggregateSignature{
				Signature: as,
				Bitmap:    bitmap,
			},
		},
		HighQc:                 c.HighQC,
		LastDoubleSignEvidence: dse,
		BadProposerEvidence:    bpe,
	})
}

func (c *Consensus) StartProposeVotePhase() (interrupt bool) {
	c.Phase = lib.Phase_PROPOSE_VOTE
	msg := c.LeaderMessages.GetLeaderMessage(c.View.Copy())
	c.LeaderPublicKey, _ = lib.PublicKeyFromBytes(msg.Signature.PublicKey)
	// IF LOCKED, CONFIRM SAFE TO UNLOCK
	if c.Locked {
		if err := c.SafeNode(msg.HighQc, msg.Qc.Block); err != nil {
			c.log.Error(err.Error())
			return true
		}
	}
	byzantineEvidence := BE{
		DSE: msg.LastDoubleSignEvidence,
		BPE: msg.BadProposerEvidence,
	}
	// CHECK CANDIDATE BLOCK AGAINST STATE MACHINE
	if err := c.CheckCandidateBlock(msg.Qc.Block, &byzantineEvidence); err != nil {
		c.log.Error(err.Error())
		return true
	}
	c.Block = msg.Qc.Block
	c.ByzantineEvidence = &byzantineEvidence
	// SEND VOTE TO LEADER
	c.SendToLeader(&Message{
		Qc: &QC{
			Header: c.View.Copy(),
			Block:  c.Block},
	})
	return
}

func (c *Consensus) StartPrecommitPhase() {
	c.Phase = lib.Phase_PRECOMMIT
	if !c.SelfIsLeader() {
		return
	}
	vote, as, bitmap, err := c.Votes.GetMaj23(c.View.Copy())
	if err != nil {
		c.log.Error(err.Error())
		return
	}
	c.SendToReplicas(&Message{
		Header: c.Copy(),
		Qc: &QC{
			Header: vote.Qc.Header,
			Block:  vote.Qc.Block,
			Signature: &lib.AggregateSignature{
				Signature: as,
				Bitmap:    bitmap,
			},
		},
	})
}

func (c *Consensus) StartPrecommitVotePhase() (interrupt bool) {
	c.Phase = lib.Phase_PRECOMMIT_VOTE
	msg := c.LeaderMessages.GetLeaderMessage(c.View.Copy())
	if interrupt = c.CheckLeaderAndBlock(msg); interrupt {
		return
	}
	// LOCK AND SET HIGH-QC TO PROTECT THOSE WHO MAY COMMIT
	c.HighQC = msg.Qc
	c.Locked = true
	// SEND VOTE TO LEADER
	c.SendToLeader(&Message{
		Qc: &QC{
			Header: c.View.Copy(),
			Block:  c.Block,
		},
	})
	return
}

func (c *Consensus) StartCommitPhase() {
	c.Phase = lib.Phase_COMMIT
	if !c.SelfIsLeader() {
		return
	}
	vote, as, bitmap, err := c.Votes.GetMaj23(c.View.Copy())
	if err != nil {
		c.log.Error(err.Error())
		return
	}
	// SEND MSG TO REPLICAS
	c.SendToReplicas(&Message{
		Header: c.Copy(), // header
		Qc: &QC{
			Header: vote.Header,   // vote view
			Block:  vote.Qc.Block, // vote block
			Signature: &lib.AggregateSignature{
				Signature: as,
				Bitmap:    bitmap,
			},
		},
	})
}

func (c *Consensus) StartCommitProcessPhase() (interrupt bool) {
	c.Phase = lib.Phase_COMMIT_PROCESS
	msg := c.LeaderMessages.GetLeaderMessage(c.View.Copy())
	// CONFIRM LEADER & BLOCK
	if interrupt = c.CheckLeaderAndBlock(msg); interrupt {
		return
	}
	if err := c.CommitBlock(msg.Qc); err != nil {
		return true
	}
	c.ByzantineEvidence = c.LeaderMessages.GetByzantineEvidence(c.View.Copy(),
		c.ValidatorSet, c.LastValidatorSet, c.LeaderPublicKey.Bytes(), &c.Votes, c.SelfIsLeader())
	c.NewHeight()
	return
}

func (c *Consensus) CheckLeaderAndBlock(msg *Message) (interrupt bool) {
	// CONFIRM IS LEADER
	if !c.IsLeader(msg.Signature.PublicKey) {
		c.log.Error(lib.ErrInvalidLeaderPublicKey().Error())
		return true
	}

	// CONFIRM BLOCK
	if !c.Block.Equals(msg.Qc.Block) {
		c.log.Error(ErrMismatchBlocks().Error())
		return true
	}
	return
}

func (c *Consensus) HandleMessage(message proto.Message) lib.ErrorI {
	switch msg := message.(type) {
	case *Message:
		switch {
		case msg.IsReplicaMessage() || msg.IsPacemakerMessage():
			isPartialQC, err := msg.Check(c.View.Copy(), c.ValidatorSet)
			if err != nil {
				return err
			}
			if isPartialQC {
				return lib.ErrNoMaj23()
			}
			return c.Votes.AddVote(c.LeaderPublicKey.Bytes(), c.View.Copy(), msg, c.ValidatorSet, c.LastValidatorSet)
		case msg.IsLeaderMessage():
			partialQC, err := msg.Check(c.View.Copy(), c.ValidatorSet)
			if err != nil {
				return err
			}
			if partialQC {
				return c.LeaderMessages.AddIncompleteQC(msg)
			}
			return c.LeaderMessages.AddLeaderMessage(msg)
		}
	}
	return ErrUnknownConsensusMsg(message)
}

func (c *Consensus) RoundInterrupt(timer *time.Time) {
	c.Phase = lib.Phase_ROUND_INTERRUPT
	// send pacemaker message
	c.SendToLeader(&Message{
		Header: c.View.Copy(),
	})
	c.ResetAndSleep(timer)
	c.NewRound(false)
	c.Pacemaker()
}

func (c *Consensus) NewRound(newHeight bool) {
	if !newHeight {
		c.Round++
	}
	c.Phase = lib.Phase_ELECTION
	c.Votes.NewRound(c.Round)
	c.LeaderMessages.NewRound(c.Round, c.ValidatorSet.Key)
}
func (c *Consensus) NewHeight() {
	c.Height++
	c.Round = 0
	c.HighQC = nil
	c.LeaderPublicKey = nil
	c.Block = nil
	c.Votes.NewHeight()
	c.LeaderMessages.NewHeight()
	c.NewRound(true)
}

func (c *Consensus) Pacemaker() {
	highestQuorumRoundFromPeers := c.Votes.Pacemaker()
	if highestQuorumRoundFromPeers > c.Round {
		c.Round = highestQuorumRoundFromPeers
	}
}

func (c *Consensus) SafeNode(qc *QC, b *lib.Block) lib.ErrorI {
	block, view := qc.Block, qc.Header
	if bytes.Equal(b.BlockHeader.Hash, block.BlockHeader.Hash) {
		return ErrMismatchBlocks()
	}
	lockedBlock, locked := c.HighQC.Block, c.HighQC.Header
	if bytes.Equal(lockedBlock.BlockHeader.Hash, block.BlockHeader.Hash) {
		return nil // SAFETY
	}
	if view.Round > locked.Round {
		return nil // LIVENESS
	}
	return ErrFailedSafeNodePredicate()
}

func (c *Consensus) ResetAndSleep(startTime *time.Time) {
	processingTime := time.Since(*startTime)
	var sleepTime time.Duration
	switch c.Phase {
	case lib.Phase_ELECTION:
		sleepTime = c.SleepTime(c.Config.ElectionTimeoutMS)
	case lib.Phase_ELECTION_VOTE:
		sleepTime = c.SleepTime(c.Config.ElectionVoteTimeoutMS)
	case lib.Phase_PROPOSE:
		sleepTime = c.SleepTime(c.Config.ProposeTimeoutMS)
	case lib.Phase_PROPOSE_VOTE:
		sleepTime = c.SleepTime(c.Config.ProposeVoteTimeoutMS)
	case lib.Phase_PRECOMMIT:
		sleepTime = c.SleepTime(c.Config.PrecommitTimeoutMS)
	case lib.Phase_PRECOMMIT_VOTE:
		sleepTime = c.SleepTime(c.Config.PrecommitVoteTimeoutMS)
	case lib.Phase_COMMIT:
		sleepTime = c.SleepTime(c.Config.CommitTimeoutMS)
	case lib.Phase_COMMIT_PROCESS:
		sleepTime = c.SleepTime(c.Config.CommitProcessMS)
	}
	if sleepTime > processingTime {
		time.Sleep(sleepTime - processingTime)
	}
	*startTime = time.Now()
}

func (c *Consensus) SleepTime(sleepTimeMS int) time.Duration {
	return time.Duration(math.Pow(float64(sleepTimeMS), float64(c.Round+1)) * float64(time.Millisecond))
}

func (c *Consensus) SelfIsLeader() bool {
	return c.IsLeader(c.PublicKey.Bytes())
}

func (c *Consensus) IsLeader(sender []byte) bool {
	return bytes.Equal(sender, c.LeaderPublicKey.Bytes())
}

func (c *Consensus) SendToReplicas(msg lib.Signable) {
	if err := msg.Sign(c.PrivateKey); err != nil {
		c.log.Error(err.Error())
		return
	}
	if err := c.P2P.SendToValidators(msg); err != nil {
		c.log.Error(err.Error())
		return
	}
}

func (c *Consensus) SendToLeader(msg lib.Signable) {
	if err := msg.Sign(c.PrivateKey); err != nil {
		c.log.Error(err.Error())
		return
	}
	if err := c.P2P.SendTo(c.LeaderPublicKey.Bytes(), lib.Topic_CONSENSUS, msg); err != nil {
		c.log.Error(err.Error())
		return
	}
}

func (c *Consensus) getProducerKeys() [][]byte {
	keys, err := c.FSM.GetProducerKeys()
	if err != nil {
		return nil
	}
	return keys.ProducerKeys
}

type DSE = lib.DoubleSignEvidences
type BPE = lib.BadProposerEvidences
type BE = lib.ByzantineEvidence
type QC = lib.QuorumCertificate
