package bft

import (
	"bytes"
	"fmt"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"sort"
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
	Locked        bool // TODO walk through the resubmission of the lock cause not sure
	Proposal      []byte
	SortitionData *SortitionData

	ByzantineEvidence *ByzantineEvidence
	PartialQCs        PartialQCs
	PacemakerMessages PacemakerMessages
	LastValidatorSet  ValSet

	PublicKey  []byte
	PrivateKey crypto.PrivateKeyI
	Config     lib.Config
	log        lib.LoggerI

	Controller
	App
	resetBFT chan time.Duration
	syncing  *atomic.Bool
}

func New(c lib.Config, valKey crypto.PrivateKeyI, height uint64, vs, lastVS ValSet,
	con Controller, app App, l lib.LoggerI) (*Consensus, lib.ErrorI) {
	return &Consensus{
		View:         &lib.View{Height: height},
		Votes:        make(VotesForHeight),
		Proposals:    make(ProposalsForHeight),
		ValidatorSet: vs,
		ByzantineEvidence: &ByzantineEvidence{
			DSE: DoubleSignEvidences{},
			BPE: BadProposerEvidences{},
		},
		PartialQCs:        make(PartialQCs),
		PacemakerMessages: make(PacemakerMessages),
		LastValidatorSet:  lastVS,
		PublicKey:         valKey.PublicKey().Bytes(),
		PrivateKey:        valKey,
		Config:            c,
		log:               l,
		Controller:        con,
		App:               app,
		resetBFT:          make(chan time.Duration, 1),
		syncing:           con.Syncing(),
	}, nil
}

func (c *Consensus) Start() {
	phaseTimeout, optimisticTimeout := c.newTimer(), c.newTimer()
	for {
		select {
		case <-phaseTimeout.C: // BFT TIMER TO HANDLE EACH PHASE
		case <-optimisticTimeout.C: // OPTIMISTIC AFTER R.10 TO ALLOW MAX SPEED DURING HALTS
			if !c.PhaseHas23Maj() {
				continue
			}
		case processTime := <-c.resetBFT: // CONTROLLER TRIGGER (NEW BLOCK FROM CANOPY OR APP)
			c.log.Info("Resetting BFT timers after receiving a new block")
			c.SetTimerForNextPhase(phaseTimeout, optimisticTimeout, c.Round, CommitProcess, processTime)
			continue
		}
		c.HandlePhase(phaseTimeout, optimisticTimeout)
	}
}

func (c *Consensus) HandlePhase(phaseTimeout, optimisticTimeout *time.Timer) {
	c.Controller.Lock()
	defer c.Controller.Unlock()
	if c.syncing.Load() {
		c.log.Info("Paused BFT loop as currently syncing")
		phaseTimeout.Stop()
		return
	}
	if !c.SelfIsValidator() {
		c.log.Info("Not currently a validator, waiting for a new block")
		phaseTimeout.Stop()
		return
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
	}
	c.SetTimerForNextPhase(phaseTimeout, optimisticTimeout, c.Round, c.Phase, time.Since(startTime))
	return
}

func (c *Consensus) StartElectionPhase() {
	c.log.Infof(c.View.ToString())
	selfValidator, err := c.ValidatorSet.GetValidator(c.PublicKey)
	if err != nil {
		c.log.Error(err.Error())
		return
	}
	c.SortitionData = &SortitionData{
		LastProposersPublicKeys: c.LoadProposerKeys().ProposerKeys,
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
	c.log.Info(c.View.ToString())
	// SELECT PROPOSER (set is required for self-send)
	c.ProposerKey = SelectProposerFromCandidates(c.GetElectionCandidates(), c.SortitionData, c.ValidatorSet.ValidatorSet)
	defer func() { c.ProposerKey = nil }()
	// SEND VOTE TO PROPOSER
	c.SendToProposer(&Message{
		Qc: &QC{
			Header:      c.View.Copy(),
			ProposerKey: c.ProposerKey,
		},
		HighQc:                 c.HighQC,
		LastDoubleSignEvidence: c.ByzantineEvidence.DSE.Evidence,
		BadProposerEvidence:    c.ByzantineEvidence.BPE.Evidence,
	})
}

func (c *Consensus) StartProposePhase() {
	c.log.Info(c.View.ToString())
	vote, as, err := c.GetMajorityVote()
	if err != nil {
		c.log.Error(err.Error())
		return
	}
	// PRODUCE BLOCK OR USE HQC BLOCK
	if c.HighQC == nil {
		c.Proposal, err = c.ProduceProposal(c.ByzantineEvidence)
		if err != nil {
			c.log.Error(err.Error())
			return
		}
	}
	// SEND MSG TO REPLICAS
	c.SendToReplicas(&Message{
		Header: c.View.Copy(),
		Qc: &QC{
			Header:      vote.Qc.Header,
			Proposal:    c.Proposal,
			ProposerKey: vote.Qc.ProposerKey,
			Signature:   as,
		},
		HighQc:                 c.HighQC,
		LastDoubleSignEvidence: c.ByzantineEvidence.DSE.Evidence,
		BadProposerEvidence:    c.ByzantineEvidence.BPE.Evidence,
	})
}

func (c *Consensus) StartProposeVotePhase() {
	c.log.Info(c.View.ToString())
	msg := c.GetProposal()
	if msg == nil {
		c.log.Warn("No valid message received from Proposer")
		c.RoundInterrupt()
		return
	}
	c.ProposerKey = msg.Signature.PublicKey
	// IF LOCKED, CONFIRM SAFE TO UNLOCK
	if c.Locked {
		if err := c.SafeNode(msg.HighQc, msg.Qc.Proposal); err != nil {
			c.log.Error(err.Error())
			c.RoundInterrupt()
			return
		}
	}
	byzantineEvidence := &ByzantineEvidence{
		DSE: NewDSE(msg.LastDoubleSignEvidence),
		BPE: NewBPE(msg.BadProposerEvidence),
	}
	// CHECK CANDIDATE BLOCK AGAINST STATE MACHINE
	if err := c.ValidateProposal(msg.Qc.Proposal, byzantineEvidence); err != nil {
		c.log.Error(err.Error())
		c.RoundInterrupt()
		return
	}
	c.Proposal = msg.Qc.Proposal
	c.ByzantineEvidence = byzantineEvidence // BE stored in case of round interrupt and replicas locked on a proposal with BE
	// SEND VOTE TO PROPOSER
	c.SendToProposer(&Message{
		Qc: &QC{
			Header:       c.View.Copy(),
			ProposalHash: c.HashProposal(c.Proposal),
		},
	})
}

func (c *Consensus) StartPrecommitPhase() {
	c.log.Info(c.View.ToString())
	if !c.SelfIsProposer() {
		return
	}
	vote, as, err := c.GetMajorityVote()
	if err != nil {
		c.log.Error(err.Error())
		return
	}
	c.SendToReplicas(&Message{
		Header: c.Copy(),
		Qc: &QC{
			Header:       vote.Qc.Header,
			ProposalHash: c.HashProposal(c.Proposal),
			Signature:    as,
		},
	})
}

func (c *Consensus) StartPrecommitVotePhase() {
	c.log.Info(c.View.ToString())
	msg := c.GetProposal()
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
	c.HighQC.Proposal = c.Proposal
	c.Locked = true
	// SEND VOTE TO PROPOSER
	c.SendToProposer(&Message{
		Qc: &QC{
			Header:       c.View.Copy(),
			ProposalHash: c.HashProposal(c.Proposal),
		},
	})
}

func (c *Consensus) StartCommitPhase() {
	c.log.Info(c.View.ToString())
	if !c.SelfIsProposer() {
		return
	}
	vote, as, err := c.GetMajorityVote()
	if err != nil {
		c.log.Error(err.Error())
		return
	}
	// SEND MSG TO REPLICAS
	c.SendToReplicas(&Message{
		Header: c.Copy(), // header
		Qc: &QC{
			Header:       vote.Qc.Header,             // vote view
			ProposalHash: c.HashProposal(c.Proposal), // vote block
			Signature:    as,
		},
	})
}

func (c *Consensus) StartCommitProcessPhase() {
	c.log.Info(c.View.ToString())
	msg := c.GetProposal()
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
	msg.Qc.Proposal = c.Proposal
	// OVERWRITE ANY SAVED BE WITH NEW SINCE ROUND IS COMPLETE
	c.ByzantineEvidence = &ByzantineEvidence{
		DSE: c.GetDSE(),
		BPE: c.GetBPE(),
	}
	// GOSSIP COMMITTED BLOCK MESSAGE TO PEERS AND SELF
	c.GossipCertificate(msg.Qc)
}

func (c *Consensus) RoundInterrupt() {
	c.log.Warn(c.View.ToString())
	c.Phase = RoundInterrupt
	// send pacemaker message
	c.SendToReplicas(&Message{
		Qc: &lib.QuorumCertificate{
			Header: c.View.Copy(),
		},
	})
}

// Pacemaker sets the highest round that 2/3rds majority have seen
func (c *Consensus) Pacemaker() {
	c.log.Info(c.View.ToString())
	c.NewRound(false)
	var sortedVotes []*Message
	for _, vote := range c.PacemakerMessages {
		sortedVotes = append(sortedVotes, vote)
	}
	sort.Slice(sortedVotes, func(i, j int) bool {
		return sortedVotes[i].Qc.Header.Round >= sortedVotes[j].Qc.Header.Round
	})
	totalVotedPower, minPowerFor23, pacemakerRound := uint64(0), c.ValidatorSet.MinimumMaj23, uint64(0)
	for _, vote := range sortedVotes {
		round := vote.Qc.Header.Round
		validator, err := c.ValidatorSet.GetValidator(vote.Signature.PublicKey)
		if err != nil {
			continue
		}
		totalVotedPower += validator.VotingPower
		if totalVotedPower >= minPowerFor23 {
			pacemakerRound = round
			break
		}
	}
	if pacemakerRound > c.Round {
		c.log.Infof("Pacemaker peers set round: %d", pacemakerRound)
		c.Round = pacemakerRound
	}
}

type PacemakerMessages map[string]*Message // in map to de-duplicate the pacemaker votes for the latest

func (c *Consensus) AddPacemakerMessage(msg *Message) (err lib.ErrorI) {
	c.PacemakerMessages[lib.BytesToString(msg.Signature.PublicKey)] = msg
	return
}

func (c *Consensus) PhaseHas23Maj() bool {
	c.Controller.Lock()
	defer c.Controller.Unlock()
	switch c.Phase {
	case ElectionVote, ProposeVote, PrecommitVote, CommitProcess:
		return c.GetProposal() != nil
	case Propose, Precommit, Commit:
		_, _, err := c.GetMajorityVote()
		return err == nil
	}
	return false
}

func (c *Consensus) CheckProposerAndBlock(msg *Message) (interrupt bool) {
	// CONFIRM IS PROPOSER
	if !c.IsProposer(msg.Signature.PublicKey) {
		c.log.Error(lib.ErrInvalidProposerPubKey().Error())
		return true
	}

	// CONFIRM BLOCK
	if !bytes.Equal(c.HashProposal(c.Proposal), msg.Qc.ProposalHash) {
		c.log.Error(ErrMismatchedProposals().Error())
		return true
	}
	return
}

func (c *Consensus) NewRound(newHeight bool) {
	if !newHeight {
		c.Round++
	}
	c.Votes.NewRound(c.Round)
	c.Proposals[c.Round] = make(map[string][]*Message)
}

func (c *Consensus) NewHeight() {
	c.Round = 0
	c.Votes = make(VotesForHeight)
	c.Proposals = make(ProposalsForHeight)
	c.PartialQCs = make(PartialQCs)
	c.PacemakerMessages = make(PacemakerMessages)
	c.HighQC = nil
	c.ProposerKey = nil
	c.Proposal = nil
	c.Locked = false
	c.SortitionData = nil
	c.NewRound(true)
	c.Phase = Election
}

// SafeNode is the codified Hotstuff SafeNodePredicate:
// - Protects replicas who may have committed to a previous value by locking on that value when signing a Precommit Message
// - May unlock if new proposer:
//   - SAFETY: uses the same value the replica is locked on (safe because it will match the value that may have been committed by others)
//   - LIVENESS: uses a lock with a higher round (safe because replica is convinced no other replica committed to their locked value as +2/3rds locked on a higher round)
func (c *Consensus) SafeNode(proposerHQC *QC, msgProposal []byte) lib.ErrorI {
	hqcCandidate, hqcView := proposerHQC.ProposalHash, proposerHQC.Header
	if !bytes.Equal(c.HashProposal(msgProposal), hqcCandidate) {
		return ErrMismatchedProposals() // PROPOSAL IN MSG MUST BE JUSTIFIED WITH HIGH-QC
	}
	lockedCandidate, lockedView := c.HighQC.Proposal, c.HighQC.Header
	if bytes.Equal(c.HashProposal(lockedCandidate), hqcCandidate) {
		return nil // SAFETY (SAME PROPOSAL AS LOCKED)
	}
	if hqcView.Round > lockedView.Round {
		return nil // LIVENESS (HIGHER ROUND THAN LOCKED)
	}
	return ErrFailedSafeNodePredicate()
}

func (c *Consensus) SetTimerForNextPhase(phaseTimeout, optimisticTimeout *time.Timer, round uint64, phase lib.Phase, processTime time.Duration) {
	waitTime := c.GetPhaseWaitTime(phase, round)
	switch phase {
	default:
		c.Phase++
	case CommitProcess:
		c.NewHeight()
	case Pacemaker:
		c.Phase = Election
	}
	c.SetTimerWait(phaseTimeout, optimisticTimeout, waitTime, processTime)
}

func (c *Consensus) GetPhaseWaitTime(phase Phase, round uint64) (waitTime time.Duration) {
	switch phase {
	case Election:
		waitTime = c.WaitTime(c.Config.ElectionTimeoutMS, round)
	case ElectionVote:
		waitTime = c.WaitTime(c.Config.ElectionVoteTimeoutMS, round)
	case Propose:
		waitTime = c.WaitTime(c.Config.ProposeTimeoutMS, round)
	case ProposeVote:
		waitTime = c.WaitTime(c.Config.ProposeVoteTimeoutMS, round)
	case Precommit:
		waitTime = c.WaitTime(c.Config.PrecommitTimeoutMS, round)
	case PrecommitVote:
		waitTime = c.WaitTime(c.Config.PrecommitVoteTimeoutMS, round)
	case Commit:
		waitTime = c.WaitTime(c.Config.CommitTimeoutMS, round)
	case CommitProcess:
		waitTime = c.WaitTime(c.Config.CommitProcessMS, round)
	case RoundInterrupt:
		waitTime = c.WaitTime(c.Config.RoundInterruptTimeoutMS, round)
	case Pacemaker:
		waitTime = c.WaitTime(c.Config.CommitProcessMS, round)
	}
	return
}

func (c *Consensus) SetTimerWait(timer, optimistic *time.Timer, waitTime, processTime time.Duration) {
	var timerDuration time.Duration
	if waitTime > processTime {
		timerDuration = waitTime - processTime
	} else {
		timerDuration = 0
	}
	c.log.Debugf("Setting consensus timer: %s", timerDuration.Round(time.Second))
	c.resetTimer(timer, timerDuration)
	c.resetTimer(optimistic, c.GetPhaseWaitTime(c.Phase, 10))
}

func (c *Consensus) WaitTime(sleepTimeMS int, round uint64) time.Duration {
	return time.Duration(uint64(sleepTimeMS)*(round+1)) * time.Millisecond
}

func (c *Consensus) SelfIsProposer() bool      { return c.IsProposer(c.PublicKey) }
func (c *Consensus) IsProposer(id []byte) bool { return bytes.Equal(id, c.ProposerKey) }
func (c *Consensus) SelfIsValidator() bool {
	selfValidator, _ := c.ValidatorSet.GetValidator(c.PublicKey)
	return selfValidator != nil
}

func (c *Consensus) newTimer() (timer *time.Timer) {
	timer = time.NewTimer(0)
	<-timer.C
	return
}

func (c *Consensus) resetTimer(t *time.Timer, duration time.Duration) {
	c.stopTimer(t)
	t.Reset(duration)
}

func (c *Consensus) stopTimer(t *time.Timer) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
}

func (c *Consensus) ResetBFTChan() chan time.Duration { return c.resetBFT }

func phaseToString(p Phase) string {
	return fmt.Sprintf("%d_%s", p, lib.Phase_name[int32(p)])
}

type (
	QC     = lib.QuorumCertificate
	Phase  = lib.Phase
	ValSet = lib.ValidatorSet

	App interface {
		ProduceProposal(be *ByzantineEvidence) (proposal []byte, err lib.ErrorI)
		HashProposal(proposal []byte) []byte
		CheckProposal(proposal []byte) lib.ErrorI
		ValidateProposal(proposal []byte, evidence *ByzantineEvidence) lib.ErrorI
	}
	Controller interface {
		Lock()
		Unlock()
		LoadValSet(height uint64) (lib.ValidatorSet, lib.ErrorI)
		LoadCertificate(height uint64) (*lib.QuorumCertificate, lib.ErrorI)
		LoadProposerKeys() *lib.ProposerKeys
		LoadMinimumEvidenceHeight() (uint64, lib.ErrorI)
		IsValidDoubleSigner(height uint64, address []byte) bool
		GossipCertificate(certificate *lib.QuorumCertificate)
		SendToReplicas(msg lib.Signable)
		SendToProposer(msg lib.Signable)
		Syncing() *atomic.Bool
	}
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
