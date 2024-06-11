package consensus

import (
	"bytes"
	"fmt"
	"github.com/ginchuco/ginchu/fsm"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"github.com/ginchuco/ginchu/p2p"
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

func New(c lib.Config, valKey crypto.PrivateKeyI, db lib.StoreI, l lib.LoggerI) (*Consensus, lib.ErrorI) {
	sm, err := fsm.New(c, db, l)
	if err != nil {
		return nil, err
	}
	height := sm.Height()
	valSet, err := sm.LoadValSet(height)
	if err != nil {
		return nil, err
	}
	lastVS, err := sm.LoadValSet(height - 1)
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
		PublicKey:        valKey.PublicKey().Bytes(),
		PrivateKey:       valKey,
		P2P:              p2p.New(valKey, maxVals, c, l),
		Config:           c,
		log:              l,
		FSM:              sm,
		Mempool:          mempool,
		RWMutex:          sync.RWMutex{},
		newBlock:         make(chan time.Duration, 1),
		syncDone:         make(chan struct{}),
		syncing:          atomic.Bool{},
	}, nil
}

func (c *Consensus) Start() {
	c.P2P.Start()
	c.StartListeners()
	go c.Sync()
	go c.StartBFT()
}

func (c *Consensus) Stop() {
	c.Lock()
	defer c.Unlock()
	if err := c.FSM.Store().(lib.StoreI).Close(); err != nil {
		c.log.Error(err.Error())
	}
	c.P2P.Stop()
}

func (c *Consensus) StartBFT() {
	timer := time.NewTimer(time.Hour)
	for {
		select {
		case <-timer.C:
			if c.syncing.Load() {
				c.log.Info("Paused BFT loop as currently syncing")
				timer.Stop()
				continue
			}
			if !c.SelfIsValidator() {
				c.log.Info("Not currently a validator, waiting for a new block")
				timer.Stop()
				continue
			}
			startTime := time.Now()
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
			c.SetTimerForNextPhase(timer, c.Phase, time.Since(startTime))
		case processTime := <-c.newBlock:
			c.log.Info("Resetting BFT timer after receiving a new block")
			c.SetTimerForNextPhase(timer, CommitProcess, processTime)
		case <-c.syncDone:
			c.log.Info("Syncing done ✅ ")
			c.SetTimerForNextPhase(timer, CommitProcess, c.TimeSinceLastBlock())
		}
	}
}

func (c *Consensus) StartElectionPhase() {
	c.Lock()
	defer c.Unlock()
	c.log.Infof(c.View.ToString())
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
	c.log.Info(c.View.ToString())
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
	c.log.Info(c.View.ToString())
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
			dse,
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
		Header: c.View.Copy(),
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
	c.log.Info(c.View.ToString())
	msg := c.Proposals.GetProposal(c.View.Copy())
	if msg == nil {
		c.log.Warn("No valid message received from Proposer")
		c.RoundInterrupt()
		return
	}
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
	c.log.Info(c.View.ToString())
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
	c.log.Info(c.View.ToString())
	msg := c.Proposals.GetProposal(c.View.Copy())
	if msg == nil {
		c.log.Warn("No valid message received from Proposer")
		c.RoundInterrupt()
		return
	}
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
	c.log.Info(c.View.ToString())
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
			Header:    vote.Qc.Header,           // vote view
			BlockHash: c.Block.BlockHeader.Hash, // vote block
			Signature: as,
		},
	})
}

func (c *Consensus) StartCommitProcessPhase() {
	c.Lock()
	defer c.Unlock()
	c.log.Info(c.View.ToString())
	msg := c.Proposals.GetProposal(c.View.Copy())
	if msg == nil {
		c.log.Warn("No valid message received from Proposer")
		c.RoundInterrupt()
		return
	}
	// CONFIRM PROPOSER & BLOCK
	if interrupt := c.CheckProposerAndBlock(msg); interrupt {
		c.RoundInterrupt()
		return
	}
	msg.Qc.Block = c.Block
	c.gossipBlock(msg.Qc)
	c.ByzantineEvidence = c.Proposals.GetByzantineEvidence(c.View.Copy(), c.ValidatorSet, c.FSM.LoadValSet,
		c.FSM.GetMinimumEvidenceHeight, c.FSM.LoadEvidence, c.FSM.LoadCertificate, c.ProposerKey, &c.Votes, c.SelfIsProposer())
}

func (c *Consensus) RoundInterrupt() {
	c.log.Warn(c.View.ToString())
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
		minEvidenceHeight, _ := c.FSM.GetMinimumEvidenceHeight()
		height, vs, proposer := c.Height, c.ValidatorSet, c.ProposerKey
		c.RUnlock()
		switch {
		case msg.IsReplicaMessage() || msg.IsPacemakerMessage():
			if err := msg.CheckReplicaMessage(height, blockHash); err != nil {
				c.log.Errorf("Received invalid vote from %s", lib.BytesToString(msg.Signature.PublicKey))
				return err
			}
			c.log.Debugf("Received %s message from replica: %s", msg.Qc.Header.ToString(), lib.BzToTruncStr(msg.Signature.PublicKey))
			return c.Votes.AddVote(proposer, height, msg, vs, c.LoadValSet, minEvidenceHeight, c.GetEvidenceByHeight)
		case msg.IsProposerMessage():
			partialQC, err := msg.CheckProposerMessage(proposer, blockHash, height, c.LoadValSet)
			if err != nil {
				c.log.Errorf("Received invalid proposal from %s", lib.BytesToString(msg.Signature.PublicKey))
				return err
			}
			c.log.Debugf("Received %s message from proposer: %s", msg.Header.ToString(), lib.BzToTruncStr(msg.Signature.PublicKey))
			if partialQC {
				return c.Proposals.AddPartialQC(msg)
			}
			return c.Proposals.AddProposal(msg, vs)
		}
	}
	return ErrUnknownConsensusMsg(message)
}

func (c *Consensus) LoadValSet(height uint64) (lib.ValidatorSet, lib.ErrorI) {
	c.Lock()
	defer c.Unlock()
	return c.FSM.LoadValSet(height)
}

func (c *Consensus) GetEvidenceByHeight(height uint64) (*lib.DoubleSigners, lib.ErrorI) {
	c.Lock()
	defer c.Unlock()
	return c.FSM.LoadEvidence(height)
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

func (c *Consensus) SetTimerForNextPhase(timer *time.Timer, phase lib.Phase, processTime time.Duration) {
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
	c.SetTimerWait(timer, waitTime, processTime)
}

func (c *Consensus) SetTimerWait(timer *time.Timer, waitTime, processTime time.Duration) {
	var timerDuration time.Duration
	if waitTime > processTime {
		timerDuration = waitTime - processTime
	} else {
		timerDuration = 0
	}
	c.log.Debugf("Setting consensus timer: %s", timerDuration.Round(time.Second))
	c.stopTimer(timer)
	timer.Reset(timerDuration)
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
	if err := c.P2P.SelfSend(c.PublicKey, Cons, msg); err != nil {
		c.log.Error(err.Error())
		return
	}
	c.log.Debugf("done sending to replica")
}

func (c *Consensus) SendToProposer(msg lib.Signable) {
	if err := msg.Sign(c.PrivateKey); err != nil {
		c.log.Error(err.Error())
		return
	}
	if c.SelfIsProposer() {
		if err := c.P2P.SelfSend(c.PublicKey, Cons, msg); err != nil {
			c.log.Error(err.Error())
		}
		return
	}
	if err := c.P2P.SendTo(c.ProposerKey, Cons, msg); err != nil {
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
		select {
		case <-t.C:
		default:
		}
	}
}

func (c *Consensus) JSONSummary() ([]byte, lib.ErrorI) {
	c.RLock()
	defer c.RUnlock()
	c.Votes.Lock()
	defer c.Votes.Unlock()
	c.Proposals.Lock()
	defer c.Proposals.Unlock()
	hash := lib.HexBytes{}
	if c.Block != nil && c.Block.BlockHeader != nil {
		hash = c.Block.BlockHeader.Hash
	}
	selfKey, err := crypto.NewPublicKeyFromBytes(c.PublicKey)
	if err != nil {
		return nil, ErrInvalidPublicKey()
	}
	var propAddress lib.HexBytes
	if c.ProposerKey != nil {
		propKey, _ := crypto.NewPublicKeyFromBytes(c.ProposerKey)
		propAddress = propKey.Address().Bytes()
	}
	status := ""
	switch c.View.Phase {
	case Election, Propose, Precommit, Commit:
		proposal := c.Proposals.getProposal(c.View)
		if proposal == nil {
			status = fmt.Sprintf("waiting for proposal")
		} else {
			status = fmt.Sprintf("received proposal")
		}
	default:
		if bytes.Equal(c.ProposerKey, c.PublicKey) {
			_, _, votedPercentage := c.Votes.getLeadingVote(c.View, c.ValidatorSet.TotalPower)
			status = fmt.Sprintf("received %d%% of votes", votedPercentage)
		} else {
			status = fmt.Sprintf("voting on proposal")
		}
	}
	return lib.MarshalJSONIndent(Summary{
		Syncing:         c.syncing.Load(),
		View:            *c.View,
		BlockHash:       hash,
		Locked:          c.Locked,
		Address:         selfKey.Address().Bytes(),
		PublicKey:       c.PublicKey,
		ProposerAddress: propAddress,
		Proposer:        c.ProposerKey,
		Proposals: proposalsForHeight{
			ProposalsByRound: c.Proposals.ProposalsByRound,
			PartialQCs:       c.Proposals.PartialQCs,
		},
		Votes: votesByHeight{
			VotesByRound:          c.Votes.VotesByRound,
			PacemakerVotesByRound: c.Votes.PacemakerVotesByRound,
		},
		Status: status,
	})
}

func phaseString(p Phase) string {
	return fmt.Sprintf("%d_%s", p, lib.Phase_name[int32(p)])
}

type Summary struct {
	Syncing         bool               `json:"syncing"`
	View            lib.View           `json:"view"`
	BlockHash       lib.HexBytes       `json:"blockHash"`
	Locked          bool               `json:"locked"`
	Address         lib.HexBytes       `json:"address"`
	PublicKey       lib.HexBytes       `json:"publicKey"`
	ProposerAddress lib.HexBytes       `json:"proposerAddress"`
	Proposer        lib.HexBytes       `json:"proposer"`
	Proposals       proposalsForHeight `json:"proposals"`
	Votes           votesByHeight      `json:"votes"`
	Status          string             `json:"status"`
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
