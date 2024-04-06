package consensus

import (
	"bytes"
	"github.com/ginchuco/ginchu/consensus/leader_election"
	"github.com/ginchuco/ginchu/crypto"
	lib "github.com/ginchuco/ginchu/types"
	"google.golang.org/protobuf/proto"
	"math"
	"time"
)

type ConsensusState struct {
	View

	LeaderPublicKey crypto.PublicKeyI

	ValidatorSet   ValidatorSet
	Votes          HeightVoteSet
	LeaderMessages HeightLeaderMessages
	HighQC         *QuorumCertificate
	Locked         bool
	Block          *lib.Block

	PublicKey  crypto.PublicKeyI
	PrivateKey crypto.PrivateKeyI

	P2P    lib.P2P
	App    lib.App
	Config Config
	log    lib.Logger
}

type ElectionMessages struct {
	messages map[uint64][]*ElectionMessage // round -> messages
}

type Config struct {
	ElectionTimeoutMS      int
	ElectionVoteTimeoutMS  int
	ProposeTimeoutMS       int
	ProposeVoteTimeoutMS   int
	PrecommitTimeoutMS     int
	PrecommitVoteTimeoutMS int
	CommitTimeoutMS        int
	CommitProcessMS        int // the majority of block time should be here
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
			cs.RoundInterrupt()
			continue
		}
		cs.ResetAndSleep(timer)
		cs.StartPrecommitPhase()
		cs.ResetAndSleep(timer)
		if interrupt := cs.StartPrecommitVotePhase(); interrupt {
			cs.RoundInterrupt()
			continue
		}
		cs.ResetAndSleep(timer)
		cs.StartCommitPhase()
		cs.ResetAndSleep(timer)
		if interrupt := cs.StartCommitProcessPhase(); interrupt {
			cs.RoundInterrupt()
			continue
		}
	}
}

func (cs *ConsensusState) RoundInterrupt() {
	cs.Round++
	cs.Pacemaker()
}

func (cs *ConsensusState) Pacemaker() {
	highestQuorumRoundFromPeers := cs.Votes.Pacemaker()
	if highestQuorumRoundFromPeers > cs.Round {
		cs.Round = highestQuorumRoundFromPeers
	}
}

func (cs *ConsensusState) StartElectionPhase() {
	cs.Phase = Phase_ELECTION
	selfValidator, err := cs.ValidatorSet.GetValidator(cs.PublicKey.Bytes())
	if err != nil {
		cs.log.Error(err.Error())
		return
	}
	_, vrf, isCandidate := leader_election.Sortition(&leader_election.SortitionParams{
		SortitionData: leader_election.SortitionData{
			LastNLeaderPubKeys: nil, // TODO
			Height:             cs.Height,
			Round:              cs.Round,
			TotalValidators:    cs.ValidatorSet.numValidators,
			VotingPower:        selfValidator.VotingPower,
			TotalPower:         cs.ValidatorSet.totalPower,
		},
		PrivateKey: cs.PrivateKey,
	})
	if isCandidate {
		cs.SendLeaderMessage(&ElectionMessage{
			Header: &View{
				Height: cs.Height,
				Round:  cs.Round,
				Phase:  cs.Phase,
			},
			Vrf: vrf,
		})
	}
}

func (cs *ConsensusState) StartElectionVotePhase() {
	cs.Phase = Phase_ELECTION_VOTE
	var outs [][]byte
	var candidates []crypto.PublicKeyI
	var leader crypto.PublicKeyI
	sortitionData := leader_election.SortitionData{
		LastNLeaderPubKeys: nil, // TODO
		Height:             cs.Height,
		Round:              cs.Round,
		TotalValidators:    cs.ValidatorSet.numValidators,
		TotalPower:         cs.ValidatorSet.totalPower,
	}
	cs.LeaderMessages.Lock()
	roundLeaderMessages := cs.LeaderMessages.GetRound(cs.Round)
	electionMessages := roundLeaderMessages.GetElectionMessages()
	cs.LeaderMessages.Unlock()
	for _, msg := range electionMessages {
		v, err := cs.ValidatorSet.GetValidator(msg.Vrf.PublicKey)
		if err != nil {
			cs.log.Error(err.Error())
			continue
		}
		publicKey, er := crypto.NewBLSPublicKeyFromBytes(msg.Vrf.PublicKey)
		if er != nil {
			cs.log.Error(ErrPubKeyFromBytes(er).Error())
			continue
		}
		sortitionData.VotingPower = v.VotingPower
		out, isCandidate := leader_election.VerifyCandidate(&leader_election.SortitionVerifyParams{
			SortitionData: sortitionData,
			Signature:     msg.Vrf.Signature,
			PublicKey:     publicKey,
		})
		if isCandidate {
			outs = append(outs, out)
			candidates = append(candidates, publicKey)
		}
	}
	if candidates != nil {
		leader = candidates[leader_election.SelectLeaderFromCandidates(outs)]
	} else {
		publicKey := leader_election.WeightedRoundRobin(&leader_election.RoundRobinParams{
			SortitionData: sortitionData,
			ValidatorSet:  cs.ValidatorSet.validatorSet,
		})
		var err error
		leader, err = crypto.NewBLSPublicKeyFromBytes(publicKey)
		if err != nil {
			cs.log.Error(ErrPubKeyFromBytes(err).Error())
			return
		}
	}
	cs.LeaderPublicKey = leader
	cs.SendReplicaMessage(&ReplicaMessage{
		Header: &View{
			Height: cs.Height,
			Round:  cs.Round,
			Phase:  cs.Phase,
		},
		Payload: &ReplicaMessage_LeaderPublicKey{cs.LeaderPublicKey.Bytes()},
		HighQC:  cs.HighQC,
	})
}

func (cs *ConsensusState) StartProposePhase() {
	cs.Phase = Phase_PROPOSE
	cs.Votes.Lock()
	round := cs.Votes.GetRound(cs.Round)
	payload, multiKey := round.GetQuorumForElection()
	cs.Votes.Unlock()
	leaderPublicKey, err := publicKeyFromBytes(payload.GetLeaderPublicKey())
	if err != nil {
		cs.log.Error(err.Error())
		return
	}
	if !cs.PublicKey.Equals(leaderPublicKey) {
		cs.log.Error(ErrMismatchPublicKeys().Error())
		return
	}
	cs.LeaderPublicKey = leaderPublicKey
	if payload.HighQC != nil {
		cs.HighQC = payload.HighQC
		cs.Block = cs.HighQC.Payload.GetBlock()
	} else {
		cs.Block, err = cs.App.ProduceCandidateBlock()
		if err != nil {
			cs.log.Error(err.Error())
			return
		}
	}
	signature, er := multiKey.AggregateSignatures()
	if er != nil {
		cs.log.Error(ErrAggregateSignature(er).Error())
		return
	}
	cs.SendLeaderMessage(&LeaderMessage{
		Header:  cs.Copy(),
		HighQc:  cs.HighQC,
		Payload: &LeaderMessage_Block{Block: cs.Block},
		Qc: &QuorumCertificate{
			Header:  payload.Header,
			Payload: payload,
			Signature: &AggregateSignature{
				Signature: signature,
				Bitmap:    multiKey.Bitmap(),
			},
		},
	})
}

func (cs *ConsensusState) StartProposeVotePhase() (interrupt bool) {
	cs.Phase = Phase_PROPOSE_VOTE
	cs.LeaderMessages.Lock()
	roundLeaderMessages := cs.LeaderMessages.GetRound(cs.Round)
	msg := roundLeaderMessages.GetProposeMessage()
	cs.LeaderMessages.Unlock()
	if err := msg.Check(cs.View.Copy(), cs.ValidatorSet.key.Copy(), true); err != nil {
		cs.log.Error(err.Error())
		return true
	}
	if !bytes.Equal(msg.LeaderSignature.PublicKey, msg.Qc.Payload.GetLeaderPublicKey()) {
		cs.log.Error(ErrMismatchPublicKeys().Error())
		return true
	}
	if cs.Locked {
		if err := msg.HighQc.Check(nil, cs.ValidatorSet.key.Copy(), true); err != nil {
			cs.log.Error(err.Error())
			return true
		}
		if !cs.SafeNode(msg.HighQc, msg.GetBlock()) {
			return true
		}
	}
	if err := cs.App.CheckCandidateBlock(msg.GetBlock()); err != nil {
		cs.log.Error(err.Error())
		return true
	}
	cs.Block = msg.GetBlock()
	cs.SendReplicaMessage(&ReplicaMessage{
		Header:  cs.View.Copy(),
		Payload: &ReplicaMessage_Block{cs.Block},
	})
	return false
}

func (cs *ConsensusState) StartPrecommitPhase() {
	cs.Phase = Phase_PRECOMMIT
	cs.Votes.Lock()
	round := cs.Votes.GetRound(cs.Round)
	payload, multiKey := round.GetQuorumForPropose()
	cs.Votes.Unlock()
	if err := payload.Check(cs.View.Copy(), true, true); err != nil {
		cs.log.Error(err.Error())
		return
	}
	if !cs.Block.Equals(payload.GetBlock()) {
		cs.log.Error(ErrMismatchBlocks().Error())
		return
	}
	signature, er := multiKey.AggregateSignatures()
	if er != nil {
		cs.log.Error(ErrAggregateSignature(er).Error())
		return
	}
	cs.SendLeaderMessage(&LeaderMessage{
		Header:  cs.Copy(),
		Payload: &LeaderMessage_Block{Block: cs.Block},
		Qc: &QuorumCertificate{
			Header:  payload.Header,
			Payload: payload,
			Signature: &AggregateSignature{
				Signature: signature,
				Bitmap:    multiKey.Bitmap(),
			},
		},
	})
}

func (cs *ConsensusState) StartPrecommitVotePhase() (interrupt bool) {
	cs.Phase = Phase_PRECOMMIT_VOTE
	cs.LeaderMessages.Lock()
	roundLeaderMessages := cs.LeaderMessages.GetRound(cs.Round)
	msg := roundLeaderMessages.GetPrecommitMessage()
	cs.LeaderMessages.Unlock()
	if err := msg.Check(cs.View.Copy(), cs.ValidatorSet.key.Copy(), true); err != nil {
		cs.log.Error(err.Error())
		return true
	}
	if !bytes.Equal(msg.LeaderSignature.PublicKey, msg.Qc.Payload.GetLeaderPublicKey()) {
		cs.log.Error(ErrMismatchPublicKeys().Error())
		return true
	}
	if !cs.Block.Equals(msg.GetBlock()) {
		cs.log.Error(ErrMismatchBlocks().Error())
		return true
	}
	cs.HighQC = msg.Qc
	cs.Locked = true
	cs.SendReplicaMessage(&ReplicaMessage{
		Header:  cs.View.Copy(),
		Payload: &ReplicaMessage_Block{cs.Block},
	})
	return false
}

func (cs *ConsensusState) StartCommitPhase() {
	cs.Phase = Phase_COMMIT
	cs.Votes.Lock()
	round := cs.Votes.GetRound(cs.Round)
	payload, multiKey := round.GetQuorumForPrecommit()
	cs.Votes.Unlock()
	if err := payload.Check(cs.View.Copy(), true, true); err != nil {
		cs.log.Error(err.Error())
		return
	}
	if !cs.Block.Equals(payload.GetBlock()) {
		cs.log.Error(ErrMismatchBlocks().Error())
		return
	}
	signature, er := multiKey.AggregateSignatures()
	if er != nil {
		cs.log.Error(ErrAggregateSignature(er).Error())
		return
	}
	cs.SendLeaderMessage(&LeaderMessage{
		Header:  cs.Copy(),
		Payload: &LeaderMessage_Block{Block: cs.Block},
		Qc: &QuorumCertificate{
			Header:  payload.Header,
			Payload: payload,
			Signature: &AggregateSignature{
				Signature: signature,
				Bitmap:    multiKey.Bitmap(),
			},
		},
	})
}

func (cs *ConsensusState) StartCommitProcessPhase() (interrupt bool) {
	cs.Phase = Phase_COMMIT_PROCESS
	cs.LeaderMessages.Lock()
	roundLeaderMessages := cs.LeaderMessages.GetRound(cs.Round)
	msg := roundLeaderMessages.GetCommitMessage()
	cs.LeaderMessages.Unlock()
	if err := msg.Check(cs.View.Copy(), cs.ValidatorSet.key.Copy(), true); err != nil {
		cs.log.Error(err.Error())
		return true
	}
	if !bytes.Equal(msg.LeaderSignature.PublicKey, msg.Qc.Payload.GetLeaderPublicKey()) {
		cs.log.Error(ErrMismatchPublicKeys().Error())
		return true
	}
	if !cs.Block.Equals(msg.GetBlock()) {
		cs.log.Error(ErrMismatchBlocks().Error())
		return true
	}
	if err := cs.App.CommitBlock(cs.Block, &lib.BeginBlockParams{
		ProposerAddress: cs.Block.BlockHeader.ProposerAddress,
		BadProposers:    nil, // TODO
		NonSigners:      nil, // TODO
		FaultySigners:   nil, // TODO
		DoubleSigners:   nil, // TODO
		ValidatorSet:    cs.ValidatorSet.validatorSet,
	}); err != nil {
		return true
	}
	return false
}

func (cs *ConsensusState) HandleMessage(message proto.Message) lib.ErrorI {
	switch msg := message.(type) {
	case *ReplicaMessage:
		switch msg.Header.Phase {
		case Phase_ELECTION_VOTE, Phase_PROPOSE_VOTE, Phase_PRECOMMIT_VOTE:
			if err := msg.Check(cs.View.Copy(), false, true); err != nil {
				return err
			}
			return cs.Votes.Add(msg, cs.ValidatorSet)
		}
	case *LeaderMessage:
		switch msg.Header.Phase {
		case Phase_PROPOSE, Phase_PRECOMMIT, Phase_COMMIT:
			if err := msg.Check(cs.View.Copy(), cs.ValidatorSet.key.Copy(), false); err != nil {
				return err
			}
			return cs.LeaderMessages.AddLeaderMessage(msg)
		}
	case *ElectionMessage:
		if msg.Header.Phase == Phase_ELECTION {
			if err := msg.Check(cs.View.Copy(), true); err != nil {
				return err
			}
			return cs.LeaderMessages.AddElectionMessage(msg)
		}
	case *PacemakerMessage:
		if err := msg.Check(cs.View.Copy()); err != nil {
			return err
		}
		return cs.Votes.AddPacemakerMessage(msg, cs.ValidatorSet)
	}
	return ErrUnknownConsensusMsg(message)
}

func (cs *ConsensusState) SafeNode(qc *QuorumCertificate, b *lib.Block) bool {
	block, view := qc.GetPayload().GetBlock(), qc.GetPayload().Header
	if block == nil || block.BlockHeader == nil || b == nil || b.BlockHeader == nil {
		cs.log.Error(ErrEmptyBlock().Error())
		return false
	}
	if bytes.Equal(b.BlockHeader.Hash, block.BlockHeader.Hash) {
		cs.log.Error(ErrMismatchBlocks().Error())
		return false
	}
	lockedBlock, lockedView := cs.HighQC.Payload.GetBlock(), cs.HighQC.Payload.Header
	if bytes.Equal(lockedBlock.BlockHeader.Hash, block.BlockHeader.Hash) {
		cs.log.Info("unlocking due to safety rule")
		return true
	}
	if view.Round > lockedView.Round {
		cs.log.Info("unlocking due to liveness rule")
		return true
	}
	cs.log.Error(ErrFailedSafeNodePredicate().Error())
	return false
}

func (cs *ConsensusState) ResetAndSleep(startTime *time.Time) {
	processingTime := time.Since(*startTime)
	var sleepTime time.Duration
	switch cs.Phase {
	case Phase_ELECTION:
		sleepTime = cs.SleepTime(cs.Config.ElectionTimeoutMS)
	case Phase_ELECTION_VOTE:
		sleepTime = cs.SleepTime(cs.Config.ElectionVoteTimeoutMS)
	case Phase_PROPOSE:
		sleepTime = cs.SleepTime(cs.Config.ProposeTimeoutMS)
	case Phase_PROPOSE_VOTE:
		sleepTime = cs.SleepTime(cs.Config.ProposeVoteTimeoutMS)
	case Phase_PRECOMMIT:
		sleepTime = cs.SleepTime(cs.Config.PrecommitTimeoutMS)
	case Phase_PRECOMMIT_VOTE:
		sleepTime = cs.SleepTime(cs.Config.PrecommitVoteTimeoutMS)
	case Phase_COMMIT:
		sleepTime = cs.SleepTime(cs.Config.CommitTimeoutMS)
	case Phase_COMMIT_PROCESS:
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

type SignedMessage interface {
	proto.Message
	Sign(privateKey crypto.PrivateKeyI) lib.ErrorI
}

func (cs *ConsensusState) SendLeaderMessage(msg SignedMessage) {
	if err := msg.Sign(cs.PrivateKey); err != nil {
		cs.log.Error(err.Error())
		return
	}
	if err := cs.P2P.SendToAll(msg); err != nil {
		cs.log.Error(err.Error())
		return
	}
}

func (cs *ConsensusState) SendReplicaMessage(msg SignedMessage) {
	if err := msg.Sign(cs.PrivateKey); err != nil {
		cs.log.Error(err.Error())
		return
	}
	if err := cs.P2P.SendToOne(cs.LeaderPublicKey.Bytes(), msg); err != nil {
		cs.log.Error(err.Error())
		return
	}
}
