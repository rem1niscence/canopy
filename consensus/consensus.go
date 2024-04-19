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
	*lib.View
	Votes         VotesForHeight
	Proposals     ProposalsForHeight
	ProposerKey   []byte
	ValidatorSet  ValSet
	HighQC        *QC
	Locked        bool
	Block         *lib.Block
	SortitionData *SortitionData

	ByzantineEvidence *BE
	LastValidatorSet  ValSet

	PublicKey  []byte
	PrivateKey crypto.PrivateKeyI
	Config     lib.Config
	log        lib.LoggerI

	P2P     *p2p.P2P
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
		View:             &lib.View{Height: height},
		LastValidatorSet: lastVS,
		ValidatorSet:     valSet,
		Votes:            NewVotesForHeight(),
		Proposals:        NewProposalsForHeight(),
		PublicKey:        pk.PublicKey().Bytes(),
		PrivateKey:       pk,
		P2P:              p2p.New(pk, maxVals, c, l),
		Config:           c,
		log:              l,
		FSM:              sm,
		Mempool:          NewMempool(fsmCopy, c.MempoolConfig, l),
		Mutex:            sync.Mutex{},
	}, nil
}

func (c *Consensus) Start() {
	c.P2P.Start()
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
	c.Lock()
	defer c.Unlock()
	c.Phase = Election
	c.ProposerKey = nil
	selfValidator, err := c.ValidatorSet.GetValidator(c.PublicKey)
	if err != nil {
		c.log.Error(err.Error())
		return
	}
	c.SortitionData = &SortitionData{
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
	c.Lock()
	defer c.Unlock()
	c.Phase = ElectionVote
	// PROPOSER SELECTION
	candidates, err := c.Proposals.GetElectionCandidates(c.View.Round, c.ValidatorSet, c.SortitionData)
	if err != nil {
		c.log.Error(err.Error())
	}
	c.ProposerKey = SelectProposerFromCandidates(candidates, c.SortitionData, c.ValidatorSet.ValidatorSet)

	// CHECK BYZANTINE EVIDENCE
	bpe, dse := BPE(nil), DSE(nil)
	if c.ByzantineEvidence != nil {
		bpe = c.ByzantineEvidence.BPE
		dse = c.ByzantineEvidence.DSE
	}

	// SEND VOTE TO PROPOSER
	c.SendToProposer(&Message{
		Qc: &QC{
			Header:      c.View.Copy(),
			ProposerKey: c.ProposerKey,
		},
		HighQc:                 c.HighQC,
		LastDoubleSignEvidence: dse,
		BadProposerEvidence:    bpe,
	})
}

func (c *Consensus) StartProposePhase() {
	c.Lock()
	defer c.Unlock()
	c.Phase = Propose
	vote, as, err := c.Votes.GetMaj23(c.View)
	if err != nil {
		c.log.Error(err.Error())
		return
	}
	// GATHER BYZANTINE EVIDENCE
	dse, bpe := lib.NewDSE(vote.LastDoubleSignEvidence), lib.NewBPE(vote.BadProposerEvidence)
	// PRODUCE BLOCK OR USE HQC BLOCK
	if vote.HighQc == nil {
		c.Block, err = c.ProduceCandidateBlock(
			bpe.GetBadProposers(),
			dse.GetDoubleSigners(c.Height, c.ValidatorSet, c.LastValidatorSet),
		)
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
			Header:      vote.Qc.Header,
			Block:       c.Block,
			ProposerKey: vote.Qc.ProposerKey,
			Signature:   as,
		},
		HighQc:                 c.HighQC,
		LastDoubleSignEvidence: dse.DSE,
		BadProposerEvidence:    bpe.BPE,
	})
}

func (c *Consensus) StartProposeVotePhase() (interrupt bool) {
	c.Lock()
	defer c.Unlock()
	c.Phase = ProposeVote
	msg := c.Proposals.GetProposal(c.View.Copy())
	c.ProposerKey = msg.Signature.PublicKey
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
	// SEND VOTE TO PROPOSER
	c.SendToProposer(&Message{
		Qc: &QC{
			Header: c.View.Copy(),
			Block:  c.Block},
	})
	return
}

func (c *Consensus) StartPrecommitPhase() {
	c.Lock()
	defer c.Unlock()
	c.Phase = Precommit
	if !c.SelfIsProposer() {
		return
	}
	vote, as, err := c.Votes.GetMaj23(c.View)
	if err != nil {
		c.log.Error(err.Error())
		return
	}
	c.SendToReplicas(&Message{
		Header: c.Copy(),
		Qc: &QC{
			Header:    vote.Qc.Header,
			Block:     vote.Qc.Block,
			Signature: as,
		},
	})
}

func (c *Consensus) StartPrecommitVotePhase() (interrupt bool) {
	c.Lock()
	defer c.Unlock()
	c.Phase = PrecommitVote
	msg := c.Proposals.GetProposal(c.View.Copy())
	if interrupt = c.CheckProposerAndBlock(msg); interrupt {
		return
	}
	// LOCK AND SET HIGH-QC TO PROTECT THOSE WHO MAY COMMIT
	c.HighQC = msg.Qc
	c.Locked = true
	// SEND VOTE TO PROPOSER
	c.SendToProposer(&Message{
		Qc: &QC{
			Header: c.View.Copy(),
			Block:  c.Block,
		},
	})
	return
}

func (c *Consensus) StartCommitPhase() {
	c.Lock()
	defer c.Unlock()
	c.Phase = Commit
	if !c.SelfIsProposer() {
		return
	}
	vote, as, err := c.Votes.GetMaj23(c.View)
	if err != nil {
		c.log.Error(err.Error())
		return
	}
	// SEND MSG TO REPLICAS
	c.SendToReplicas(&Message{
		Header: c.Copy(), // header
		Qc: &QC{
			Header:    vote.Header,   // vote view
			Block:     vote.Qc.Block, // vote block
			Signature: as,
		},
	})
}

func (c *Consensus) StartCommitProcessPhase() (interrupt bool) {
	c.Lock()
	defer c.Unlock()
	c.Phase = CommitProcess
	msg := c.Proposals.GetProposal(c.View.Copy())
	// CONFIRM PROPOSER & BLOCK
	if interrupt = c.CheckProposerAndBlock(msg); interrupt {
		return
	}
	if err := c.CommitBlock(msg.Qc); err != nil {
		return true
	}
	c.ByzantineEvidence = c.Proposals.GetByzantineEvidence(c.View.Copy(),
		c.ValidatorSet, c.LastValidatorSet, c.ProposerKey, &c.Votes, c.SelfIsProposer())
	c.NewHeight()
	return
}

func (c *Consensus) CheckProposerAndBlock(msg *Message) (interrupt bool) {
	// CONFIRM IS PROPOSER
	if !c.IsProposer(msg.Signature.PublicKey) {
		c.log.Error(lib.ErrInvalidProposerPubKey().Error())
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
		c.Lock()
		height, vals, lastVals, proposer := c.Height, c.ValidatorSet, c.LastValidatorSet, c.ProposerKey
		c.Unlock()
		switch {
		case msg.IsReplicaMessage() || msg.IsPacemakerMessage():
			if err := msg.CheckReplicaMessage(height, vals); err != nil {
				return err
			}
			return c.Votes.AddVote(proposer, height, msg, vals, lastVals)
		case msg.IsProposerMessage():
			partialQC, err := msg.CheckProposerMessage(proposer, height, vals)
			if err != nil {
				return err
			}
			if partialQC {
				return c.Proposals.AddPartialQC(msg)
			}
			return c.Proposals.AddProposal(msg)
		}
	}
	return ErrUnknownConsensusMsg(message)
}

func (c *Consensus) RoundInterrupt(timer *time.Time) {
	c.Lock()
	defer c.Unlock()
	c.Phase = RoundInterrupt
	// send pacemaker message
	c.SendToProposer(&Message{
		Header: c.View.Copy(),
	})
	c.ResetAndSleep(timer)
	c.NewRound(false)
	if pacerRound := c.Votes.Pacemaker(); pacerRound > c.Round {
		c.Round = pacerRound
	}
}

func (c *Consensus) NewRound(newHeight bool) {
	if !newHeight {
		c.Round++
	}
	c.Phase = Election
	c.Votes.NewRound(c.Round)
	c.Proposals.NewRound(c.Round)
}
func (c *Consensus) NewHeight() {
	c.Height++
	c.Round = 0
	c.HighQC = nil
	c.ProposerKey = nil
	c.Block = nil
	c.Locked = false
	c.SortitionData = nil
	c.Votes.NewHeight()
	c.Proposals.NewHeight()
	c.NewRound(true)
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
	case Election:
		sleepTime = c.SleepTime(c.Config.ElectionTimeoutMS)
	case ElectionVote:
		sleepTime = c.SleepTime(c.Config.ElectionVoteTimeoutMS)
	case Propose:
		sleepTime = c.SleepTime(c.Config.ProposeTimeoutMS)
	case ProposeVote:
		sleepTime = c.SleepTime(c.Config.ProposeVoteTimeoutMS)
	case Precommit:
		sleepTime = c.SleepTime(c.Config.PrecommitTimeoutMS)
	case PrecommitVote:
		sleepTime = c.SleepTime(c.Config.PrecommitVoteTimeoutMS)
	case Commit:
		sleepTime = c.SleepTime(c.Config.CommitTimeoutMS)
	case CommitProcess:
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

func (c *Consensus) SelfIsProposer() bool {
	return c.IsProposer(c.PublicKey)
}

func (c *Consensus) IsProposer(sender []byte) bool {
	return bytes.Equal(sender, c.ProposerKey)
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

func (c *Consensus) SendToProposer(msg lib.Signable) {
	if err := msg.Sign(c.PrivateKey); err != nil {
		c.log.Error(err.Error())
		return
	}
	if err := c.P2P.SendTo(c.ProposerKey, lib.Topic_CONSENSUS, msg); err != nil {
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

type (
	DSE    = []*lib.DoubleSignEvidence
	BPE    = []*lib.BadProposerEvidence
	BE     = lib.ByzantineEvidence
	QC     = lib.QuorumCertificate
	Phase  = lib.Phase
	ValSet = lib.ValidatorSet
)

const (
	Election       = lib.Phase_ELECTION
	ElectionVote   = lib.Phase_ELECTION_VOTE
	Propose        = lib.Phase_PROPOSE
	ProposeVote    = lib.Phase_PROPOSE_VOTE
	Precommit      = lib.Phase_PRECOMMIT
	PrecommitVote  = lib.Phase_PRECOMMIT_VOTE
	Commit         = lib.Phase_COMMIT
	CommitProcess  = lib.Phase_COMMIT_PROCESS
	RoundInterrupt = lib.Phase_ROUND_INTERRUPT
)
