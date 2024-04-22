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
	"sync/atomic"
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
	sync.RWMutex
	newBlock chan time.Duration
	syncDone chan struct{}
	syncing  atomic.Bool
}

func New(pk crypto.PrivateKeyI, c lib.Config, l lib.LoggerI) (*Consensus, lib.ErrorI) {
	db, err := store.New(c, l)
	if err != nil {
		return nil, err
	}
	sm, err := fsm.New(c, db)
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
	mempool, err := NewMempool(sm, c.MempoolConfig, l)
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
		Mempool:          mempool,
		RWMutex:          sync.RWMutex{},
		newBlock:         make(chan time.Duration),
		syncDone:         make(chan struct{}),
		syncing:          atomic.Bool{},
	}, nil
}

func (c *Consensus) Start() {
	c.P2P.Start()
	go c.Sync()
	go c.StartBFT()
	go c.StartListeners()
}

func (c *Consensus) StartBFT() {
	startTime, timer := time.Now(), time.NewTimer(time.Hour)
	for {
		select {
		case <-timer.C:
			if !c.SelfIsValidator() || c.syncing.Load() {
				timer.Stop()
				continue
			}
			switch c.Phase {
			case Election:
				c.StartElectionPhase()
			case ElectionVote:
				c.StartElectionVotePhase()
			case Propose:
				c.StartProposePhase()
			case ProposeVote:
				c.StartProposeVotePhase()
			case Precommit:
				c.StartPrecommitPhase()
			case PrecommitVote:
				c.StartPrecommitVotePhase()
			case Commit:
				c.StartCommitPhase()
			case CommitProcess:
				c.StartCommitProcessPhase()
			case Pacemaker:
				c.Pacemaker()
				continue
			}
			c.SetTimerForNextPhase(&startTime, timer, c.Phase, time.Since(startTime))
		case processTime := <-c.newBlock:
			c.stopTimer(timer)
			c.SetTimerForNextPhase(&startTime, timer, CommitProcess, processTime)
		case <-c.syncDone:
			c.stopTimer(timer)
			c.SetTimerForNextPhase(&startTime, timer, CommitProcess, c.TimeSinceLastBlock())
		}
	}
}

func (c *Consensus) StartElectionPhase() {
	c.Lock()
	defer c.Unlock()
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
	c.ProposerKey = nil
}

func (c *Consensus) StartProposePhase() {
	c.Lock()
	defer c.Unlock()
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

func (c *Consensus) StartProposeVotePhase() {
	c.Lock()
	defer c.Unlock()
	msg := c.Proposals.GetProposal(c.View.Copy())
	c.ProposerKey = msg.Signature.PublicKey
	// IF LOCKED, CONFIRM SAFE TO UNLOCK
	if c.Locked {
		if err := c.SafeNode(msg.HighQc, msg.Qc.Block); err != nil {
			c.log.Error(err.Error())
			c.RoundInterrupt()
			return
		}
	}
	byzantineEvidence := BE{
		DSE: msg.LastDoubleSignEvidence,
		BPE: msg.BadProposerEvidence,
	}
	// CHECK CANDIDATE BLOCK AGAINST STATE MACHINE
	if err := c.CheckCandidateBlock(msg.Qc.Block, &byzantineEvidence); err != nil {
		c.log.Error(err.Error())
		c.RoundInterrupt()
		return
	}
	c.Block = msg.Qc.Block
	c.ByzantineEvidence = &byzantineEvidence
	// SEND VOTE TO PROPOSER
	c.SendToProposer(&Message{
		Qc: &QC{
			Header:    c.View.Copy(),
			BlockHash: c.Block.BlockHeader.Hash,
		},
	})
}

func (c *Consensus) StartPrecommitPhase() {
	c.Lock()
	defer c.Unlock()
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
			BlockHash: c.Block.BlockHeader.Hash,
			Signature: as,
		},
	})
}

func (c *Consensus) StartPrecommitVotePhase() {
	c.Lock()
	defer c.Unlock()
	msg := c.Proposals.GetProposal(c.View.Copy())
	if interrupt := c.CheckProposerAndBlock(msg); interrupt {
		c.RoundInterrupt()
		return
	}
	// LOCK AND SET HIGH-QC TO PROTECT THOSE WHO MAY COMMIT
	c.HighQC = msg.Qc
	c.Locked = true
	// SEND VOTE TO PROPOSER
	c.SendToProposer(&Message{
		Qc: &QC{
			Header:    c.View.Copy(),
			BlockHash: c.Block.BlockHeader.Hash,
		},
	})
}

func (c *Consensus) StartCommitPhase() {
	c.Lock()
	defer c.Unlock()
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
			Header:    vote.Header,              // vote view
			BlockHash: c.Block.BlockHeader.Hash, // vote block
			Signature: as,
		},
	})
}

func (c *Consensus) StartCommitProcessPhase() {
	c.Lock()
	defer c.Unlock()
	msg := c.Proposals.GetProposal(c.View.Copy())
	// CONFIRM PROPOSER & BLOCK
	if interrupt := c.CheckProposerAndBlock(msg); interrupt {
		c.RoundInterrupt()
		return
	}
	msg.Qc.Block = c.Block
	c.gossipBlock(msg.Qc)
	c.ByzantineEvidence = c.Proposals.GetByzantineEvidence(c.View.Copy(),
		c.ValidatorSet, c.LastValidatorSet, c.ProposerKey, &c.Votes, c.SelfIsProposer())
}

func (c *Consensus) CheckProposerAndBlock(msg *Message) (interrupt bool) {
	// CONFIRM IS PROPOSER
	if !c.IsProposer(msg.Signature.PublicKey) {
		c.log.Error(lib.ErrInvalidProposerPubKey().Error())
		return true
	}

	// CONFIRM BLOCK
	if !bytes.Equal(c.Block.BlockHeader.Hash, msg.Qc.BlockHash) {
		c.log.Error(ErrMismatchBlocks().Error())
		return true
	}
	return
}

func (c *Consensus) HandleMessage(message proto.Message) lib.ErrorI {
	switch msg := message.(type) {
	case *Message:
		var blockHash []byte
		c.RLock()
		if c.Block != nil {
			blockHash = c.Block.BlockHeader.Hash
		}
		height, vals, lastVals, proposer := c.Height, c.ValidatorSet, c.LastValidatorSet, c.ProposerKey
		c.RUnlock()
		switch {
		case msg.IsReplicaMessage() || msg.IsPacemakerMessage():
			if err := msg.CheckReplicaMessage(height, blockHash, vals); err != nil {
				return err
			}
			return c.Votes.AddVote(proposer, height, msg, vals, lastVals)
		case msg.IsProposerMessage():
			partialQC, err := msg.CheckProposerMessage(proposer, blockHash, height, vals)
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

func (c *Consensus) RoundInterrupt() {
	c.Lock()
	defer c.Unlock()
	c.Phase = RoundInterrupt
	// send pacemaker message
	c.SendToReplicas(&Message{
		Header: c.View.Copy(),
	})
}

func (c *Consensus) Pacemaker() {
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

func (c *Consensus) SetTimerForNextPhase(startTime *time.Time, timer *time.Timer, phase lib.Phase, processTime time.Duration) {
	var waitTime time.Duration
	switch phase {
	case Election:
		waitTime = c.WaitTime(c.Config.ElectionTimeoutMS)
	case ElectionVote:
		waitTime = c.WaitTime(c.Config.ElectionVoteTimeoutMS)
	case Propose:
		waitTime = c.WaitTime(c.Config.ProposeTimeoutMS)
	case ProposeVote:
		waitTime = c.WaitTime(c.Config.ProposeVoteTimeoutMS)
	case Precommit:
		waitTime = c.WaitTime(c.Config.PrecommitTimeoutMS)
	case PrecommitVote:
		waitTime = c.WaitTime(c.Config.PrecommitVoteTimeoutMS)
	case Commit:
		waitTime = c.WaitTime(c.Config.CommitTimeoutMS)
	case CommitProcess:
		waitTime = c.WaitTime(c.Config.CommitProcessMS)
	case RoundInterrupt:
		waitTime = c.WaitTime(c.Config.RoundInterruptTimeoutMS)
	}
	c.Lock()
	if phase == CommitProcess {
		c.NewHeight()
	} else {
		c.Phase++
	}
	c.Unlock()
	c.SetTimerWait(startTime, timer, waitTime, processTime)
}

func (c *Consensus) SetTimerWait(startTime *time.Time, timer *time.Timer, waitTime, processTime time.Duration) {
	if waitTime > processTime {
		timer.Reset(waitTime - processTime)
	} else {
		timer.Reset(0)
	}
	*startTime = time.Now()
}

func (c *Consensus) WaitTime(sleepTimeMS int) time.Duration {
	return time.Duration(math.Pow(float64(sleepTimeMS), float64(c.Round+1)) * float64(time.Millisecond))
}

func (c *Consensus) TimeSinceLastBlock() time.Duration {
	c.RLock()
	lastBlockTime := c.FSM.BeginBlockParams.BlockHeader.Time.AsTime()
	c.RUnlock()
	return time.Since(lastBlockTime)
}

func (c *Consensus) SelfIsProposer() bool          { return c.IsProposer(c.PublicKey) }
func (c *Consensus) IsProposer(sender []byte) bool { return bytes.Equal(sender, c.ProposerKey) }
func (c *Consensus) SelfIsValidator() bool {
	c.RLock()
	defer c.RUnlock()
	selfValidator, _ := c.ValidatorSet.GetValidator(c.PublicKey)
	return selfValidator != nil
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

func (c *Consensus) stopTimer(t *time.Timer) {
	if !t.Stop() {
		<-t.C
	}
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
	Pacemaker      = lib.Phase_PACEMAKER
)
