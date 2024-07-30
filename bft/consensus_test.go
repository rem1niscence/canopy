package bft

import (
	"fmt"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"github.com/stretchr/testify/require"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	testTimeout = 500 * time.Millisecond
)

func TestStartElectionPhase(t *testing.T) {
	tests := []struct {
		name            string
		detail          string
		selfIsValidator bool
		numValidators   int
		expectsMessage  bool
	}{
		{
			name:            "self is leader",
			detail:          `deterministic key set ensures 'self' is an election candidate with a set of 3 Validators`,
			selfIsValidator: true,
			numValidators:   3,
			expectsMessage:  true,
		},
		{
			name:            "self is not leader",
			detail:          `deterministic key set ensures 'self' is an not election candidate with a set of 4 Validators`,
			selfIsValidator: true,
			numValidators:   4,
			expectsMessage:  false,
		},
		{
			name:            "self is not a validator",
			detail:          `self is not a validator within the deterministic key`,
			selfIsValidator: false,
			numValidators:   3,
			expectsMessage:  false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// setup
			c := newTestConsensus(t, Election, test.numValidators)
			pub, private := c.valKeys[0].PublicKey(), c.valKeys[0]
			if !test.selfIsValidator {
				pk, err := crypto.NewBLSPrivateKey()
				require.NoError(t, err)
				c.bft.PublicKey = pk.PublicKey().Bytes()
			}
			// deterministic key set ensures 'self' is not an election candidate with a set of 4 Validators
			go c.bft.StartElectionPhase()
			if !test.expectsMessage {
				select {
				case <-c.c.sendToReplicasChan:
					t.Fatal("unexpected message")
				case <-time.After(100 * time.Millisecond):
					return
				}
			}
			expectedView := lib.View{
				Height: 1,
				Round:  0,
				Phase:  Election,
			}
			select {
			case <-time.After(testTimeout):
				t.Fatal("timeout")
			case m := <-c.c.sendToReplicasChan:
				msg, ok := m.(*Message)
				require.True(t, ok)
				require.Equal(t, expectedView, *msg.Header)
				require.Equal(t, pub.Bytes(), msg.Vrf.PublicKey)
				require.Equal(t, private.Sign(formatInput(c.c.proposerKeys.ProposerKeys, expectedView.Height, expectedView.Round)), msg.Vrf.Signature)
			}
		})
	}
}

func TestStartElectionVotePhase(t *testing.T) {
	tests := []struct {
		name                 string
		detail               string
		numValidators        int
		isElectionCandidate  bool
		noElectionCandidates bool
		hasBE                bool
	}{
		{
			name:                "self is election candidate",
			detail:              `deterministic key set ensures 'self' is an election candidate with a set of 3 Validators`,
			numValidators:       3,
			isElectionCandidate: true,
		},
		{
			name:                "self is not an election candidate",
			detail:              `deterministic key set ensures 'self' is not an election candidate with a set of 4 Validators`,
			numValidators:       4,
			isElectionCandidate: false,
		},
		{
			name:                 "pseudorandom",
			detail:               `didn't 'handle messages' from any candidate, fallback to pseudorandom`,
			numValidators:        4,
			noElectionCandidates: true,
		},
		{
			name:          "byzantine evidence not nil",
			detail:        `testing sending the byzantine evidence to the leader with the leader vote`,
			numValidators: 4,
			hasBE:         true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// setup
			c := newTestConsensus(t, ElectionVote, test.numValidators)
			switch {
			case test.noElectionCandidates:
				c.setSortitionData(t)
			default:
				c.simElectionPhase(t)
			}
			expectedDoubleSignEvidence, expectedBadProposerEvidence := DoubleSignEvidences{}, BadProposerEvidences{}
			if test.hasBE {
				expectedDoubleSignEvidence.Evidence, expectedBadProposerEvidence.Evidence = c.newTestDoubleSignEvidence(t), c.newTestBadProposerEvidence(t)
				c.bft.ByzantineEvidence = &ByzantineEvidence{
					DSE: expectedDoubleSignEvidence,
					BPE: expectedBadProposerEvidence,
				}
			}
			pub, _, expectedView := c.valKeys[0].PublicKey(), c.valKeys[0], lib.View{
				Height: 1,
				Round:  0,
				Phase:  ElectionVote,
			}
			go c.bft.StartElectionVotePhase()
			select {
			case <-time.After(testTimeout):
				t.Fatal("timeout")
			case m := <-c.c.sendToProposerChan:
				msg, ok := m.(*Message)
				require.True(t, ok)
				require.NotNil(t, msg.Qc)
				require.Equal(t, *msg.Qc.Header, expectedView)
				if !test.isElectionCandidate || test.noElectionCandidates {
					require.Equal(t, msg.Qc.ProposerKey, c.valKeys[2].PublicKey().Bytes())
				} else {
					require.Equal(t, msg.Qc.ProposerKey, pub.Bytes())
				}
				if test.hasBE {
					require.NotNil(t, msg.BadProposerEvidence)
					require.Equal(t, expectedBadProposerEvidence.Evidence, msg.BadProposerEvidence)
					require.NotNil(t, msg.LastDoubleSignEvidence)
					require.Equal(t, expectedDoubleSignEvidence.Evidence, msg.LastDoubleSignEvidence)
				}
			}
		})
	}
}

func TestStartProposePhase(t *testing.T) {
	tests := []struct {
		name           string
		detail         string
		receiveEVQC    bool
		hasBE          bool
		hasLivenessHQC bool
		hasSafetyHQC   bool
	}{
		{
			name:   "self is not proposer",
			detail: `no election vote quorum certificate received`,
		},
		{
			name:        "self is proposer",
			detail:      `election vote quorum certificate received`,
			receiveEVQC: true,
		},
		{
			name:        "self is proposer with byzantine evidence",
			detail:      `election vote quorum certificate received and there was byzantine evidence attached to the QC`,
			receiveEVQC: true,
			hasBE:       true,
		},
		{
			name:           "self is proposer with existing liveness highQC", // liveness vs safety doesn't matter for the proposer, a valid precommit qc from the same height is a lock
			detail:         `election vote quorum certificate received and there was a highQC (passes liveness rule) attached to the QC`,
			receiveEVQC:    true,
			hasLivenessHQC: true,
		},
		{
			name:         "self is proposer with existing safety highQC", // liveness vs safety doesn't matter for the proposer, a valid precommit qc from the same height is a lock
			detail:       `election vote quorum certificate received and there was a highQC (passes safety rule) attached to the QC`,
			receiveEVQC:  true,
			hasSafetyHQC: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// setup
			c := newTestConsensus(t, Propose, 3)
			var multiKey crypto.MultiPublicKeyI
			if test.receiveEVQC {
				multiKey = c.simElectionVotePhase(t, 0, test.hasBE, test.hasLivenessHQC, test.hasSafetyHQC, 0)
			}
			_, _, expectedView, expectedQCView := c.valKeys[0].PublicKey(), c.valKeys[0], lib.View{
				Height: 1,
				Round:  0,
				Phase:  Propose,
			}, lib.View{
				Height: 1,
				Round:  0,
				Phase:  ElectionVote,
			}
			go c.bft.StartProposePhase()
			select {
			case <-time.After(testTimeout):
				if test.receiveEVQC {
					t.Fatal("timeout")
				}
			case m := <-c.c.sendToReplicasChan:
				if !test.receiveEVQC {
					t.Fatal("unexpected message")
				}
				msg, ok := m.(*Message)
				require.True(t, ok)
				require.Equal(t, expectedView, *msg.Header)
				require.NotNil(t, msg.Qc)
				require.Equal(t, expectedQCView.Phase, msg.Qc.Header.Phase)
				expectedProposal, _ := c.c.ProduceProposal(nil)
				require.Equal(t, expectedProposal, msg.Qc.Proposal)
				expectedAggSig, err := multiKey.AggregateSignatures()
				require.NoError(t, err)
				require.Equal(t, expectedAggSig, msg.Qc.Signature.Signature)
				if test.hasBE {
					expectedBPE, expectedDSE := c.newTestBadProposerEvidence(t), c.newTestDoubleSignEvidence(t)
					require.True(t, expectedBPE[0].ElectionVoteQc.Equals(msg.BadProposerEvidence[0].ElectionVoteQc))
					require.True(t, expectedDSE[0].VoteA.Equals(msg.LastDoubleSignEvidence[0].VoteA))
					require.True(t, expectedDSE[0].VoteB.Equals(msg.LastDoubleSignEvidence[0].VoteB))
				}
				if test.hasLivenessHQC || test.hasSafetyHQC {
					require.Equal(t, msg.Qc.Proposal, c.newHighQC(t, test.hasLivenessHQC, false).Proposal)
				}
			}
		})
	}
}

func TestStartProposeVotePhase(t *testing.T) {
	tests := []struct {
		name           string
		detail         string
		safetyLocked   bool
		livenessLocked bool
		invalidHighQC  bool
		validProposal  bool
		hasBE          bool
	}{
		{
			name:          "proposal is valid",
			detail:        `replica received a valid proposal from the leader`,
			validProposal: true,
		},
		{
			name:   "proposal is invalid",
			detail: `replica received an invalid proposal from the leader`,
		},
		{
			name:          "proposal is valid with BE",
			detail:        `proposal is valid and there's byzantine evidence attached to the message`,
			validProposal: true,
			hasBE:         true,
		},
		{
			name:          "proposal is valid and replica is locked",
			detail:        `a locked replica received a valid proposal from the leader, lock is bypassed using safety`,
			validProposal: true,
			safetyLocked:  true,
		},
		{
			name:           "proposal is valid and replica is locked",
			detail:         `a locked replica received a valid proposal from the leader, lock is bypassed using liveness`,
			validProposal:  true,
			livenessLocked: true,
		},
		{
			name:          "replica is locked and highQC doesn't pass safety or liveness",
			detail:        `a locked replica received a valid proposal with an invalid highQC justification, lock is not bypassed`,
			invalidHighQC: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// setup
			c := newTestConsensus(t, ProposeVote, 3)
			highQC, dse, bpe := (*QC)(nil), DoubleSignEvidences{}, BadProposerEvidences{}
			if test.safetyLocked || test.livenessLocked || test.invalidHighQC {
				highQC = c.newHighQC(t, test.livenessLocked, test.invalidHighQC)
			}
			if test.hasBE {
				dse.Evidence, bpe.Evidence = c.newTestDoubleSignEvidence(t), c.newTestBadProposerEvidence(t)
			}
			proposal := c.simProposePhase(t, 0, test.validProposal, dse, bpe, highQC, 0)
			expectedView := lib.View{
				Height: 1,
				Round:  0,
				Phase:  ProposeVote,
			}
			// valid proposal
			go c.bft.StartProposeVotePhase()
			select {
			case <-time.After(testTimeout):
				if test.validProposal {
					t.Fatal("timeout")
				}
			case m := <-c.c.sendToProposerChan:
				if !test.validProposal || test.invalidHighQC {
					t.Fatal("unexpected message")
				}
				msg, ok := m.(*Message)
				require.True(t, ok)
				require.NotNil(t, msg.Qc)
				require.Equal(t, *msg.Qc.Header, expectedView)
				require.Equal(t, c.c.HashProposal(proposal), msg.Qc.ProposalHash)
				require.Equal(t, c.bft.Proposal, proposal)
				if test.hasBE {
					require.NotNil(t, dse)
					require.NotNil(t, bpe)
				}
				require.Equal(t, c.bft.ByzantineEvidence.DSE.Evidence, dse.Evidence)
				require.Equal(t, c.bft.ByzantineEvidence.BPE.Evidence, bpe.Evidence)
			}
		})
	}
}

func TestStartPrecommitPhase(t *testing.T) {
	tests := []struct {
		name             string
		detail           string
		has23MajPropVote bool
		isProposer       bool
	}{
		{
			name:   "not proposer",
			detail: `self is not the proposer`,
		},
		{
			name:       "didn't received +2/3 prop vote",
			detail:     `did not receive +2/3 quorum on the propose votes from replicas`,
			isProposer: true,
		},
		{
			name:             "received +2/3 prop vote",
			detail:           `received +2/3 quorum on the propose votes from replicas`,
			isProposer:       true,
			has23MajPropVote: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// setup
			c, multiKey, proposalHash := newTestConsensus(t, Precommit, 3), crypto.MultiPublicKeyI(nil), []byte(nil)
			expectedView, expectedQCView := lib.View{
				Height: 1,
				Round:  0,
				Phase:  Precommit,
			}, lib.View{
				Height: 1,
				Round:  0,
				Phase:  ProposeVote,
			}
			if test.has23MajPropVote {
				multiKey, proposalHash = c.simProposeVotePhase(t, test.isProposer, true, 0)
			}
			go c.bft.StartPrecommitPhase()
			select {
			case <-time.After(testTimeout):
				if test.has23MajPropVote {
					t.Fatal("timeout")
				}
			case m := <-c.c.sendToReplicasChan:
				if !test.has23MajPropVote {
					t.Fatal("unexpected message")
				}
				msg, ok := m.(*Message)
				require.True(t, ok)
				require.NotNil(t, msg.Qc)
				require.Equal(t, msg.Qc.Header.Phase, expectedQCView.Phase)
				require.Equal(t, *msg.Header, expectedView)
				require.Equal(t, proposalHash, msg.Qc.ProposalHash)
				expectedAggSig, err := multiKey.AggregateSignatures()
				require.NoError(t, err)
				require.Equal(t, expectedAggSig, msg.Qc.Signature.Signature)
			}
		})
	}
}

func TestStartPrecommitVotePhase(t *testing.T) {
	tests := []struct {
		name             string
		detail           string
		proposalReceived bool
		validProposal    bool
		isProposer       bool
	}{
		{
			name:   "no proposal received",
			detail: `no proposal was received`,
		},
		{
			name:             "sender not proposer",
			detail:           `sender is not the set proposer`,
			proposalReceived: true,
		},
		{
			name:             "proposer sent invalid proposal",
			detail:           `the proposer sent a proposal that did not correspond with the block set in the propose phase`,
			proposalReceived: true,
			isProposer:       true,
		},
		{
			name:             "received +2/3 prop vote",
			detail:           `received +2/3 quorum on the propose votes from replicas`,
			proposalReceived: true,
			isProposer:       true,
			validProposal:    true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// setup
			c := newTestConsensus(t, PrecommitVote, 3)
			var proposal []byte
			if test.proposalReceived {
				proposal = c.simPrecommitPhase(t, 0)
			}
			if !test.validProposal {
				c.bft.Proposal = []byte("some other proposal") // mismatched proposals
			}
			if !test.isProposer {
				c.bft.ProposerKey = []byte("some other proposer")
			}
			expectedView := lib.View{
				Height: 1,
				Round:  0,
				Phase:  PrecommitVote,
			}
			go c.bft.StartPrecommitVotePhase()
			select {
			case <-time.After(testTimeout):
				if test.validProposal {
					t.Fatal("timeout")
				}
			case m := <-c.c.sendToProposerChan:
				if !test.validProposal {
					t.Fatal("unexpected message received")
				}
				msg, ok := m.(*Message)
				require.True(t, ok)
				require.NotNil(t, msg.Qc)
				require.Equal(t, *msg.Qc.Header, expectedView)
				require.Equal(t, c.c.HashProposal(proposal), msg.Qc.ProposalHash)
				require.True(t, c.bft.Locked)
				require.Equal(t, c.bft.HighQC.ProposalHash, msg.Qc.ProposalHash)
				require.Equal(t, c.bft.HighQC.Proposal, proposal)
			}
		})
	}
}

func TestStartCommitPhase(t *testing.T) {
	tests := []struct {
		name             string
		detail           string
		has23MajPropVote bool
		isProposer       bool
	}{
		{
			name:   "not proposer",
			detail: `self is not the proposer`,
		},
		{
			name:       "didn't received +2/3 prop vote",
			detail:     `did not receive +2/3 quorum on the propose votes from replicas`,
			isProposer: true,
		},
		{
			name:             "received +2/3 prop vote",
			detail:           `received +2/3 quorum on the propose votes from replicas`,
			isProposer:       true,
			has23MajPropVote: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// setup
			c := newTestConsensus(t, Commit, 3)
			multiKey, proposalHash := crypto.MultiPublicKeyI(nil), []byte(nil)
			expectedView, expectedQCView := lib.View{
				Height: 1,
				Round:  0,
				Phase:  Commit,
			}, lib.View{
				Height: 1,
				Round:  0,
				Phase:  PrecommitVote,
			}
			if !test.isProposer {
				c.bft.ProposerKey = []byte("some other proposer")
			}
			if test.has23MajPropVote {
				multiKey, proposalHash = c.simPrecommitVotePhase(t, 0)
			}
			go c.bft.StartCommitPhase()
			select {
			case <-time.After(testTimeout):
				if test.has23MajPropVote {
					t.Fatal("timeout")
				}
			case m := <-c.c.sendToReplicasChan:
				msg, ok := m.(*Message)
				require.True(t, ok)
				require.NotNil(t, msg.Qc)
				require.Equal(t, msg.Qc.Header.Phase, expectedQCView.Phase)
				require.Equal(t, *msg.Header, expectedView)
				require.Equal(t, proposalHash, msg.Qc.ProposalHash)
				expectedAggSig, err := multiKey.AggregateSignatures()
				require.NoError(t, err)
				require.Equal(t, expectedAggSig, msg.Qc.Signature.Signature)
			}
		})
	}
}

func TestStartCommitProcessPhase(t *testing.T) {
	tests := []struct {
		name             string
		detail           string
		proposalReceived bool
		validProposal    bool
		isProposer       bool
		hasBPE           bool
		hasPartialQCDSE  bool
		hasEVDSE         bool
	}{
		{
			name:   "no proposal received",
			detail: `no proposal was received`,
		},
		{
			name:             "sender not proposer",
			detail:           `sender is not the set proposer`,
			proposalReceived: true,
		},
		{
			name:             "proposer sent invalid proposal",
			detail:           `the proposer sent a proposal that did not correspond with the block set in the propose phase`,
			proposalReceived: true,
			isProposer:       true,
		},
		{
			name:             "received +2/3 prop vote",
			detail:           `received +2/3 quorum on the precommit votes from replicas`,
			proposalReceived: true,
			isProposer:       true,
			validProposal:    true,
		},
		{
			name:             "received +2/3 prop vote and has BPE stored",
			detail:           `received +2/3 quorum on the precommit votes from replicas and has bad proposer evidence stored from round 0`,
			proposalReceived: true,
			isProposer:       true,
			validProposal:    true,
			hasBPE:           true,
		},
		{
			name:             "received +2/3 prop vote and has partial qc DSE stored",
			detail:           `received +2/3 quorum on the precommit votes from replicas and has double sign evidence stored in the form of a conflicting partial qc`,
			proposalReceived: true,
			isProposer:       true,
			validProposal:    true,
			hasPartialQCDSE:  true,
		},
		{
			name:             "received +2/3 prop vote and has election vote DSE stored",
			detail:           `received +2/3 quorum on the precommit votes from replicas and has double sign evidence stored in the form of a conflicting election votes`,
			proposalReceived: true,
			isProposer:       true,
			validProposal:    true,
			hasEVDSE:         true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// setup
			c := newTestConsensus(t, CommitProcess, 3)
			if test.hasBPE {
				c.bft.ProposerKey = nil
				c.simProposePhase(t, 0, true, DoubleSignEvidences{}, BadProposerEvidences{}, nil, 0)
				c.bft.ProposerKey = c.valKeys[0].PublicKey().Bytes()
			}
			if test.hasPartialQCDSE {
				c.simPrecommitPhase(t, 1)
				c.newPartialQCDoubleSign(t, Precommit)
			}
			if test.hasEVDSE {
				c.simProposePhase(t, 1, true, DoubleSignEvidences{}, BadProposerEvidences{}, nil, 1)
				c.newElectionVoteDoubleSign(t)
			}
			c.bft.Round++
			multiKey, proposal := crypto.MultiPublicKeyI(nil), []byte(nil)
			if !test.isProposer {
				c.bft.ProposerKey = []byte("some other proposer")
			}
			if !test.validProposal {
				c.bft.Proposal = []byte("some other proposal")
			}
			if test.proposalReceived {
				multiKey, proposal = c.simCommitPhase(t, 1, 1)
			}
			expectedQCView := lib.View{
				Height: 1,
				Round:  1,
				Phase:  PrecommitVote,
			}
			if test.hasEVDSE {
				c.bft.ProposerKey = c.valKeys[1].PublicKey().Bytes()
			}
			go c.bft.StartCommitProcessPhase()
			select {
			case <-time.After(testTimeout):
				if test.validProposal {
					t.Fatal("timeout")
				}
			case qc := <-c.c.gossipCertChan:
				require.Equal(t, qc.Header.Phase, expectedQCView.Phase)
				require.Equal(t, qc.Proposal, proposal)
				require.Equal(t, c.c.HashProposal(proposal), qc.ProposalHash)
				require.NotNil(t, qc.Signature)
				expectedAggSig, err := multiKey.AggregateSignatures()
				require.NoError(t, err)
				require.Equal(t, expectedAggSig, qc.Signature.Signature)
				if test.hasBPE {
					require.Len(t, c.bft.ByzantineEvidence.BPE.Evidence, 1)
				}
				if test.hasPartialQCDSE || test.hasEVDSE {
					require.Len(t, c.bft.ByzantineEvidence.DSE.Evidence, 1)
				}
			}
		})
	}
}

func TestRoundInterrupt(t *testing.T) {
	// setup
	c := newTestConsensus(t, Propose, 3)
	go c.bft.RoundInterrupt()
	select {
	case <-time.After(testTimeout):
		t.Fatal("timeout")
	case m := <-c.c.sendToReplicasChan:
		msg, ok := m.(*Message)
		require.True(t, ok)
		require.EqualExportedValues(t, msg.Qc.Header, &lib.View{
			Height: 1,
			Round:  0,
			Phase:  RoundInterrupt,
		})
		require.Equal(t, c.bft.Phase, RoundInterrupt)
	}
}

func TestPacemaker(t *testing.T) {
	tests := []struct {
		name                   string
		detail                 string
		hasPeerPacemakerVotes  bool
		has23MajAtHigherRound  bool
		expectedPacemakerRound uint64
	}{
		{
			name:                   "no peer pacemaker votes",
			detail:                 "no peer pacemaker votes received, simply increment round",
			expectedPacemakerRound: 1,
		},
		{
			name:                   "received peer pacemaker votes",
			detail:                 "peer pacemaker votes received, highest +2/3 at round 1",
			hasPeerPacemakerVotes:  true,
			expectedPacemakerRound: 1,
		},
		{
			name:                   "received peer pacemaker votes which caused a round fast forward",
			detail:                 "peer pacemaker votes received, highest +2/3 at round 3",
			hasPeerPacemakerVotes:  true,
			has23MajAtHigherRound:  true,
			expectedPacemakerRound: 3,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// setup
			numValidators := 3
			if test.has23MajAtHigherRound {
				numValidators = 4
			}
			c := newTestConsensus(t, Propose, numValidators)
			if test.hasPeerPacemakerVotes {
				c.simPacemakerPhase(t)
			}
			c.bft.Pacemaker()
			require.Equal(t, uint64(1), c.bft.Height)
			require.Equal(t, test.expectedPacemakerRound, c.bft.Round)
		})
	}
}

func TestPhaseHas23Maj(t *testing.T) {
	c := newTestConsensus(t, Propose, 3)
	// 75% votes
	c.simElectionVotePhase(t, 0, false, false, false, 0)
	require.True(t, c.bft.PhaseHas23Maj())
	// 50% votes
	c.simProposeVotePhase(t, false, false, 0)
	c.bft.Phase = Precommit
	require.False(t, c.bft.PhaseHas23Maj())
	// 0% votes
	c.bft.Phase = Commit
	require.False(t, c.bft.PhaseHas23Maj())
}

func TestCheckProposerAndBlock(t *testing.T) {
	tests := []struct {
		name          string
		detail        string
		validProposer bool
		validProposal bool
	}{
		{
			name:          "valid proposer and proposal",
			detail:        "message has a proposer and proposal that corresponds to the local state",
			validProposer: true,
			validProposal: true,
		},
		{
			name:          "valid proposer and invalid proposal",
			detail:        "message has a proposer that corresponds to the local state but the proposal does not",
			validProposer: true,
		},
		{
			name:          "valid proposal and invalid proposer",
			detail:        "message has a proposal that corresponds to the local state but the proposer does not",
			validProposal: true,
		},
		{
			name:   "invalid proposal and invalid proposer",
			detail: "message doesn't have a proposal nor a proposer that corresponds to the local state",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := newTestConsensus(t, Election, 1)
			c.bft.Proposal, c.bft.ProposerKey = []byte("some proposal"), []byte("some proposer")
			messageProposal, messageProposer := []byte("some other proposal"), []byte("some other proposer")
			if test.validProposal {
				messageProposal = c.bft.Proposal
			}
			if test.validProposer {
				messageProposer = c.bft.ProposerKey
			}
			msg := &Message{
				Qc: &lib.QuorumCertificate{
					ProposalHash: crypto.Hash(messageProposal),
				},
				Signature: &lib.Signature{
					PublicKey: messageProposer,
				},
			}
			require.Equal(t, !(test.validProposer && test.validProposal), c.bft.CheckProposerAndBlock(msg))
		})
	}
}

func TestNewRound(t *testing.T) {
	tests := []struct {
		name      string
		detail    string
		newRound  bool
		newHeight bool
	}{
		{
			name:   "not new round",
			detail: "new round was not called",
		},
		{
			name:     "new round only",
			detail:   "new round was called, but not a new height",
			newRound: true,
		},
		{
			name:      "new round / height",
			detail:    "new height = new round 0",
			newHeight: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := newTestConsensus(t, Election, 4)
			c.simElectionPhase(t)
			c.simProposePhase(t, 0, true, DoubleSignEvidences{}, BadProposerEvidences{}, nil, 0)
			c.simPrecommitPhase(t, 0)
			c.simCommitPhase(t, 0, 0)
			c.simPacemakerPhase(t)
			eleLen, evVoteLen, propNil, propVoteLen, precNil, precVoteLen, comNil, expRound, paceLen := 4, 1, false, 1, false, 1, false, uint64(0), 3
			if test.newRound || test.newHeight {
				eleLen, evVoteLen, propNil, propVoteLen, precNil, precVoteLen, comNil, expRound = 0, 0, true, 0, true, 0, true, 1
			}
			if test.newRound {
				c.bft.NewRound(false)
			}
			if test.newHeight {
				expRound, paceLen = 0, 0
				c.bft.NewHeight()
				c.bft.NewRound(true)
			}
			require.Equal(t, c.bft.Round, expRound)
			require.Len(t, c.bft.Proposals[expRound][phaseToString(Election)], eleLen)
			require.Len(t, c.bft.Votes[expRound][phaseToString(ElectionVote)], evVoteLen)
			require.Equal(t, c.bft.Proposals[expRound][phaseToString(Propose)] == nil, propNil)
			require.Len(t, c.bft.Votes[expRound][phaseToString(ProposeVote)], propVoteLen)
			require.Equal(t, c.bft.Proposals[expRound][phaseToString(Precommit)] == nil, precNil)
			require.Len(t, c.bft.Votes[expRound][phaseToString(PrecommitVote)], precVoteLen)
			require.Equal(t, c.bft.Proposals[expRound][phaseToString(Commit)] == nil, comNil)
			require.Len(t, c.bft.PacemakerMessages, paceLen)
		})
	}
}

func TestGetPhaseWaitTime(t *testing.T) {
	tests := []struct {
		name             string
		detail           string
		phase            Phase
		round            uint64
		expectedWaitTime time.Duration
	}{
		{
			name:             "election phase wait time",
			detail:           "the wait time for election",
			phase:            Election,
			round:            0,
			expectedWaitTime: time.Duration(lib.DefaultConfig().ElectionTimeoutMS) * time.Millisecond,
		},
		{
			name:             "election vote phase wait time",
			detail:           "the wait time for election vote",
			phase:            ElectionVote,
			round:            0,
			expectedWaitTime: time.Duration(lib.DefaultConfig().ElectionVoteTimeoutMS) * time.Millisecond,
		},
		{
			name:             "propose phase wait time",
			detail:           "the wait time for proposal phase",
			phase:            Propose,
			round:            0,
			expectedWaitTime: time.Duration(lib.DefaultConfig().ProposeTimeoutMS) * time.Millisecond,
		},
		{
			name:             "propose vote phase wait time",
			detail:           "the wait time for proposal vote phase",
			phase:            ProposeVote,
			round:            0,
			expectedWaitTime: time.Duration(lib.DefaultConfig().ProposeVoteTimeoutMS) * time.Millisecond,
		},
		{
			name:             "precommit phase wait time",
			detail:           "the wait time for precommit phase",
			phase:            Precommit,
			round:            0,
			expectedWaitTime: time.Duration(lib.DefaultConfig().PrecommitTimeoutMS) * time.Millisecond,
		},
		{
			name:             "precommit vote phase wait time",
			detail:           "the wait time for precommit vote phase",
			phase:            PrecommitVote,
			round:            0,
			expectedWaitTime: time.Duration(lib.DefaultConfig().PrecommitVoteTimeoutMS) * time.Millisecond,
		},
		{
			name:             "commit phase wait time",
			detail:           "the wait time for commit phase",
			phase:            Commit,
			round:            0,
			expectedWaitTime: time.Duration(lib.DefaultConfig().CommitTimeoutMS) * time.Millisecond,
		},
		{
			name:             "commit process phase wait time",
			detail:           "the wait time for commit process phase",
			phase:            CommitProcess,
			round:            0,
			expectedWaitTime: time.Duration(lib.DefaultConfig().CommitProcessMS) * time.Millisecond,
		},
		{
			name:             "round interrupt phase wait time",
			detail:           "the wait time for round interrupt phase",
			phase:            RoundInterrupt,
			round:            0,
			expectedWaitTime: time.Duration(lib.DefaultConfig().RoundInterruptTimeoutMS) * time.Millisecond,
		},
		{
			name:             "propose phase wait time with round 3",
			detail:           "the wait time for round interrupt phase",
			phase:            Propose,
			round:            3,
			expectedWaitTime: time.Duration(lib.DefaultConfig().ProposeTimeoutMS) * time.Millisecond * (3 + 1),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := newTestConsensus(t, Election, 1)
			require.Equal(t, c.bft.GetPhaseWaitTime(test.phase, test.round), test.expectedWaitTime)
		})
	}
}

func TestSafeNode(t *testing.T) {
	tests := []struct {
		name             string
		detail           string
		samePropInMsg    bool
		unlockBySafety   bool
		unlockByLiveness bool
		err              lib.ErrorI
	}{
		{
			name:   "high qc doesn't justify the proposal",
			detail: "high qc contains a different proposal than the one in the message",
			err:    ErrMismatchedProposals(),
		},
		{
			name:          "high qc fails safe node predicate",
			detail:        "high qc does not satisfy safety nor liveness",
			samePropInMsg: true,
			err:           ErrFailedSafeNodePredicate(),
		},
		{
			name:           "high qc unlocks with safety",
			detail:         "high qc does satisfy the safety portion of the safe node predicate (same proposal)",
			samePropInMsg:  true,
			unlockBySafety: true,
		},
		{
			name:             "high qc unlocks with liveness",
			detail:           "high qc does satisfy the liveness portion of the safe node predicate (higher round)",
			samePropInMsg:    true,
			unlockByLiveness: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, c2 := newTestConsensus(t, PrecommitVote, 4), newTestConsensus(t, PrecommitVote, 4)
			c2.bft.Round++
			c2.simPrecommitPhase(t, 1) // higher lock
			go c2.bft.StartPrecommitVotePhase()
			<-c2.c.sendToProposerChan
			c.simPrecommitPhase(t, 0) // lock
			go c.bft.StartPrecommitVotePhase()
			<-c.c.sendToProposerChan
			var err lib.ErrorI
			switch {
			case test.unlockBySafety:
				err = c.bft.SafeNode(c.bft.HighQC, c.bft.HighQC.Proposal)
			case test.unlockByLiveness:
				err = c.bft.SafeNode(c2.bft.HighQC, c2.bft.HighQC.Proposal)
			default:
				msgProposal, highQCProposal := []byte("some proposal"), []byte("some other proposal")
				if test.samePropInMsg {
					highQCProposal = msgProposal
				}
				err = c.bft.SafeNode(&QC{
					Header:       c.bft.HighQC.Header,
					Proposal:     highQCProposal,
					ProposalHash: c.c.HashProposal(highQCProposal),
				}, msgProposal)
			}
			require.Equal(t, test.err, err)
		})
	}
}

func (tc *testConsensus) newPartialQCDoubleSign(t *testing.T, phase Phase) {
	// a partial QC and full QC for the same (H,R,P) sent to replica
	qc := &lib.QuorumCertificate{
		Header: &lib.View{
			Height: 1,
			Round:  1,
			Phase:  phase - 1,
		},
		ProposalHash: crypto.Hash([]byte("some proposal")),
	}
	sb := qc.SignBytes()
	mk := tc.valSet.Key.Copy()
	for i, pk := range tc.valKeys {
		if i == 1 || i == 2 {
			val, e := tc.valSet.GetValidator(pk.PublicKey().Bytes())
			require.NoError(t, e)
			require.NoError(t, mk.AddSigner(pk.Sign(sb), val.Index))
		}
	}
	as1, e := mk.AggregateSignatures()
	require.NoError(t, e)
	qc.Signature = &lib.AggregateSignature{
		Signature: as1,
		Bitmap:    mk.Bitmap(),
	}
	msg := &Message{
		Header: &lib.View{
			Height: 1,
			Round:  1,
			Phase:  phase,
		},
		Qc: qc,
	}
	require.NoError(t, msg.Sign(tc.valKeys[0]))
	require.NoError(t, tc.bft.HandleMessage(msg))
}

func (tc *testConsensus) newElectionVoteDoubleSign(t *testing.T) {
	// election candidate has conflicting signatures with the real proposers' election vote QC
	msg := &Message{
		Qc: &lib.QuorumCertificate{
			Header: &lib.View{
				Height: 1,
				Round:  1,
				Phase:  ElectionVote,
			},
			Proposal:     nil,
			ProposalHash: nil,
			ProposerKey:  tc.valKeys[0].PublicKey().Bytes(),
			Signature:    nil,
		},
		Signature: nil,
	}
	for i, pk := range tc.valKeys {
		if i == 1 || i == 2 {
			require.NoError(t, msg.Sign(pk))
			require.NoError(t, tc.bft.HandleMessage(msg))
		}
	}
}

func (tc *testConsensus) newTestDoubleSignEvidence(t *testing.T) []*DoubleSignEvidence {
	qcA := &lib.QuorumCertificate{
		Header: &lib.View{
			Height: 1,
			Round:  0,
			Phase:  0,
		},
		ProposalHash: crypto.Hash([]byte("some proposal")),
	}
	qcB := &lib.QuorumCertificate{
		Header: &lib.View{
			Height: 1,
			Round:  0,
			Phase:  0,
		},
		ProposalHash: crypto.Hash([]byte("some other proposal")),
	}
	sbA, sbB := qcA.SignBytes(), qcB.SignBytes()
	mk, mk2 := tc.valSet.Key.Copy(), tc.valSet.Key.Copy()
	for _, pk := range tc.valKeys {
		val, e := tc.valSet.GetValidator(pk.PublicKey().Bytes())
		require.NoError(t, e)
		require.NoError(t, mk.AddSigner(pk.Sign(sbA), val.Index))
	}
	for i, pk := range tc.valKeys {
		if i == 1 || i == 2 {
			val, e := tc.valSet.GetValidator(pk.PublicKey().Bytes())
			require.NoError(t, e)
			require.NoError(t, mk2.AddSigner(pk.Sign(sbB), val.Index))
		}
	}
	as1, e := mk.AggregateSignatures()
	require.NoError(t, e)
	qcA.Signature = &lib.AggregateSignature{
		Signature: as1,
		Bitmap:    mk.Bitmap(),
	}
	as2, e := mk2.AggregateSignatures()
	require.NoError(t, e)
	qcB.Signature = &lib.AggregateSignature{
		Signature: as2,
		Bitmap:    mk2.Bitmap(),
	}
	return []*DoubleSignEvidence{{
		VoteA: qcA,
		VoteB: qcB,
	}}
}

func (tc *testConsensus) newTestBadProposerEvidence(t *testing.T) []*BadProposerEvidence {
	qcA := &lib.QuorumCertificate{
		Header: &lib.View{
			Height: 1,
			Round:  0,
			Phase:  ElectionVote,
		},
		ProposerKey: tc.valKeys[1].PublicKey().Bytes(),
	}
	mk := tc.valSet.Key.Copy()
	for _, pk := range tc.valKeys {
		val, e := tc.valSet.GetValidator(pk.PublicKey().Bytes())
		require.NoError(t, e)
		require.NoError(t, mk.AddSigner(pk.Sign(qcA.SignBytes()), val.Index))
	}
	as1, e := mk.AggregateSignatures()
	require.NoError(t, e)
	qcA.Signature = &lib.AggregateSignature{
		Signature: as1,
		Bitmap:    mk.Bitmap(),
	}
	NewBPE()
	return []*BadProposerEvidence{{
		ElectionVoteQc: qcA,
	}}
}

func (tc *testConsensus) simElectionPhase(t *testing.T) {
	tc.setSortitionData(t)
	for _, k := range tc.valKeys {
		msg := &Message{
			Header: &lib.View{
				Height: 1,
				Round:  0,
				Phase:  Election,
			},
			Vrf: VRF(tc.c.proposerKeys.ProposerKeys, 1, 0, k),
		}
		require.NoError(t, msg.Sign(k))
		require.NoError(t, tc.bft.HandleMessage(msg))
	}
	return
}

func (tc *testConsensus) simElectionVotePhase(t *testing.T, proposerIdx int, hasBE, hasLivenessHQC, hasSafetyHQC bool, round uint64) crypto.MultiPublicKeyI {
	multiKey := tc.valSet.Key.Copy()
	for i, k := range tc.valKeys {
		if i == len(tc.valKeys)-1 {
			break // skip last signer
		}
		msg := &Message{
			Qc: &lib.QuorumCertificate{
				Header: &lib.View{
					Height: 1,
					Round:  round,
					Phase:  ElectionVote,
				},
				ProposerKey: tc.valKeys[proposerIdx].PublicKey().Bytes(),
			},
		}
		if hasBE && i == 1 {
			msg.LastDoubleSignEvidence = tc.newTestDoubleSignEvidence(t)
			msg.BadProposerEvidence = tc.newTestBadProposerEvidence(t)
		}
		if hasLivenessHQC || hasSafetyHQC && i == 1 {
			msg.HighQc = tc.newHighQC(t, hasLivenessHQC, false)
		}
		require.NoError(t, msg.Sign(k))
		require.NoError(t, multiKey.AddSigner(msg.Signature.Signature, i))
		require.NoError(t, tc.bft.HandleMessage(msg))
	}
	return multiKey
}

func (tc *testConsensus) newHighQC(t *testing.T, liveness, invalid bool) *QC {
	mockProposal, round, c := []byte("some proposal"), uint64(0), newTestConsensus(t, PrecommitVote, 3)
	tc.bft.Locked, tc.bft.HighQC, tc.bft.Proposal = true, &QC{Header: &lib.View{Height: 1, Round: 0}, Proposal: mockProposal}, mockProposal
	highQCProposal, err := tc.c.ProduceProposal(nil)
	require.NoError(t, err)
	if !invalid {
		if liveness {
			round = 1
		} else { // safety
			tc.bft.Proposal, tc.bft.HighQC.Proposal = highQCProposal, highQCProposal
		}
	}
	mk, _ := c.simPrecommitVotePhase(t, 0, round)
	aggSig, e := mk.AggregateSignatures()
	require.NoError(t, e)
	return &QC{
		Header: &lib.View{
			Height: 1,
			Round:  round,
			Phase:  PrecommitVote,
		},
		Proposal:     highQCProposal,
		ProposalHash: tc.c.HashProposal(highQCProposal),
		Signature: &lib.AggregateSignature{
			Signature: aggSig,
			Bitmap:    mk.Bitmap(),
		},
	}
}

func (tc *testConsensus) simProposePhase(t *testing.T, proposerIdx int, validProposal bool, dse DoubleSignEvidences, bpe BadProposerEvidences, highQC *QC, round uint64) (proposal []byte) {
	proposer := tc.valKeys[proposerIdx]
	proposal, err := tc.c.ProduceProposal(nil)
	require.NoError(t, err)
	if !validProposal {
		proposal = []byte("invalid")
	}
	multiKey := tc.simElectionVotePhase(t, proposerIdx, false, false, false, round)
	aggSig, e := multiKey.AggregateSignatures()
	require.NoError(t, e)
	msg := &Message{
		Header: &lib.View{
			Height: 1,
			Round:  round,
			Phase:  Propose,
		},
		Vrf: nil,
		Qc: &QC{
			Header: &lib.View{
				Height: 1,
				Round:  round,
				Phase:  ElectionVote,
			},
			Proposal:    proposal,
			ProposerKey: proposer.PublicKey().Bytes(),
			Signature: &lib.AggregateSignature{
				Signature: aggSig,
				Bitmap:    multiKey.Bitmap(),
			},
		},
		HighQc:                 highQC,
		LastDoubleSignEvidence: dse.Evidence,
		BadProposerEvidence:    bpe.Evidence,
		Signature:              nil,
	}
	require.NoError(t, msg.Sign(proposer))
	require.NoError(t, tc.bft.HandleMessage(msg))
	return
}

func (tc *testConsensus) simProposeVotePhase(t *testing.T, isProposer, maj23 bool, round uint64) (crypto.MultiPublicKeyI, []byte) {
	var err error
	if isProposer {
		tc.bft.ProposerKey = tc.bft.PublicKey
	}
	tc.bft.Proposal, err = tc.c.ProduceProposal(nil)
	require.NoError(t, err)
	proposalHash := tc.c.HashProposal(tc.bft.Proposal)
	multiKey := tc.valSet.Key.Copy()
	for i, k := range tc.valKeys {
		if !maj23 && i == 0 {
			continue // skip the first signer to ensure no 23 maj
		}
		if i == len(tc.valKeys)-1 {
			break // skip last signer
		}
		msg := &Message{
			Qc: &lib.QuorumCertificate{
				Header: &lib.View{
					Height: 1,
					Round:  round,
					Phase:  ProposeVote,
				},
				ProposalHash: proposalHash,
			},
		}
		require.NoError(t, msg.Sign(k))
		require.NoError(t, multiKey.AddSigner(msg.Signature.Signature, i))
		require.NoError(t, tc.bft.HandleMessage(msg))
	}
	return multiKey, proposalHash
}

func (tc *testConsensus) simPrecommitPhase(t *testing.T, round uint64) (proposal []byte) {
	proposer := tc.valKeys[0]
	proposal, err := tc.c.ProduceProposal(nil)
	require.NoError(t, err)
	multiKey, _ := tc.simProposeVotePhase(t, true, true, round)
	aggSig, e := multiKey.AggregateSignatures()
	require.NoError(t, e)
	msg := &Message{
		Header: &lib.View{
			Height: 1,
			Round:  round,
			Phase:  Precommit,
		},
		Qc: &QC{
			Header: &lib.View{
				Height: 1,
				Round:  round,
				Phase:  ProposeVote,
			},
			ProposalHash: tc.c.HashProposal(proposal),
			Signature: &lib.AggregateSignature{
				Signature: aggSig,
				Bitmap:    multiKey.Bitmap(),
			},
		},
	}
	require.NoError(t, msg.Sign(proposer))
	require.NoError(t, tc.bft.HandleMessage(msg))
	return
}

func (tc *testConsensus) simPrecommitVotePhase(t *testing.T, proposerIdx int, round ...uint64) (crypto.MultiPublicKeyI, []byte) {
	var err error
	tc.bft.ProposerKey = tc.valKeys[proposerIdx].PublicKey().Bytes()
	tc.bft.Proposal, err = tc.c.ProduceProposal(nil)
	require.NoError(t, err)
	proposalHash := tc.c.HashProposal(tc.bft.Proposal)
	multiKey := tc.valSet.Key.Copy()
	r := uint64(0)
	if round != nil {
		r = round[0]
	}
	for i, k := range tc.valKeys {
		if i == len(tc.valKeys)-1 {
			break // skip last signer
		}
		msg := &Message{
			Qc: &lib.QuorumCertificate{
				Header: &lib.View{
					Height: 1,
					Round:  r,
					Phase:  PrecommitVote,
				},
				ProposalHash: proposalHash,
			},
		}
		require.NoError(t, msg.Sign(k))
		require.NoError(t, multiKey.AddSigner(msg.Signature.Signature, i))
		require.NoError(t, tc.bft.HandleMessage(msg))
	}
	return multiKey, proposalHash
}

func (tc *testConsensus) simCommitPhase(t *testing.T, proposerIdx int, round uint64) (multiKey crypto.MultiPublicKeyI, proposal []byte) {
	proposer := tc.valKeys[proposerIdx]
	proposal, err := tc.c.ProduceProposal(nil)
	require.NoError(t, err)
	multiKey, _ = tc.simPrecommitVotePhase(t, proposerIdx, round)
	aggSig, e := multiKey.AggregateSignatures()
	require.NoError(t, e)
	msg := &Message{
		Header: &lib.View{
			Height: 1,
			Round:  round,
			Phase:  Commit,
		},
		Qc: &QC{
			Header: &lib.View{
				Height: 1,
				Round:  round,
				Phase:  PrecommitVote,
			},
			ProposalHash: tc.c.HashProposal(proposal),
			Signature: &lib.AggregateSignature{
				Signature: aggSig,
				Bitmap:    multiKey.Bitmap(),
			},
		},
	}
	require.NoError(t, msg.Sign(proposer))
	require.NoError(t, tc.bft.HandleMessage(msg))
	return
}

func (tc *testConsensus) simPacemakerPhase(t *testing.T) {
	for i := 1; i < len(tc.valKeys); i++ {
		pacemakerMsg := &Message{
			Qc: &lib.QuorumCertificate{
				Header: &lib.View{
					Height: 1,
					Round:  uint64(i + 2),
					Phase:  RoundInterrupt,
				},
			},
		}
		require.NoError(t, pacemakerMsg.Sign(tc.valKeys[i]))
		require.NoError(t, tc.bft.HandleMessage(pacemakerMsg))
	}
}

func (tc *testConsensus) setSortitionData(t *testing.T) {
	selfVal, err := tc.valSet.GetValidator(tc.valKeys[0].PublicKey().Bytes())
	require.NoError(t, err)
	tc.bft.SortitionData = &SortitionData{
		LastProposersPublicKeys: tc.c.proposerKeys.ProposerKeys,
		Height:                  1,
		Round:                   0,
		TotalValidators:         tc.valSet.NumValidators,
		VotingPower:             selfVal.VotingPower,
		TotalPower:              tc.valSet.TotalPower,
	}
}

type testConsensus struct {
	valKeys     []crypto.PrivateKeyI
	valSet      ValSet
	lastValKeys []crypto.PrivateKeyI
	lastValSet  ValSet
	bft         *Consensus
	c           *testController
}

func newTestConsensus(t *testing.T, phase Phase, numValidators int, numLastValidators ...int) (tc *testConsensus) {
	var err error
	if numValidators <= 0 {
		t.Fatalf("invalid number of validators")
	}
	tc, lastValidatorsCount := new(testConsensus), 0
	if numLastValidators != nil {
		lastValidatorsCount = numLastValidators[0]
	} else {
		lastValidatorsCount = numValidators
	}
	tc.lastValSet, tc.lastValKeys = newTestValSet(t, lastValidatorsCount)
	consensusValidators := lib.ConsensusValidators{
		ValidatorSet: make([]*lib.ConsensusValidator, lastValidatorsCount),
	}
	tc.valKeys = make([]crypto.PrivateKeyI, lastValidatorsCount)
	copy(consensusValidators.ValidatorSet, tc.lastValSet.ValidatorSet.ValidatorSet)
	copy(tc.valKeys, tc.lastValKeys)
	diff := numValidators - lastValidatorsCount
	switch {
	case diff < 0:
		tc.valKeys = tc.valKeys[:numValidators]
		consensusValidators.ValidatorSet = consensusValidators.ValidatorSet[:numValidators]
	case diff > 0:
		nvs, nvk := newTestValSet(t, diff)
		tc.valKeys = append(tc.valKeys, nvk...)
		consensusValidators.ValidatorSet = append(consensusValidators.ValidatorSet, nvs.ValidatorSet.ValidatorSet...)
	}
	tc.valSet, err = lib.NewValidatorSet(&consensusValidators)
	require.NoError(t, err)
	var proposerKeys [][]byte
	for _, k := range tc.valKeys {
		proposerKeys = append(proposerKeys, k.PublicKey().Bytes())
	}
	tc.c = &testController{
		Mutex:              sync.Mutex{},
		minEvidenceHeight:  0,
		proposerKeys:       &lib.ProposerKeys{ProposerKeys: proposerKeys},
		lastCandidateTime:  time.Time{},
		certificates:       nil,
		valSet:             map[uint64]ValSet{0: tc.lastValSet, 1: tc.valSet},
		resetBFTChan:       make(chan time.Duration, 1),
		syncingDoneChan:    make(chan struct{}, 1),
		syncing:            &atomic.Bool{},
		gossipCertChan:     make(chan *lib.QuorumCertificate),
		sendToProposerChan: make(chan lib.Signable),
		sendToReplicasChan: make(chan lib.Signable),
	}
	tc.bft, err = New(lib.DefaultConfig(), tc.valKeys[0], 1, tc.valSet, tc.lastValSet, tc.c, tc.c, lib.NewDefaultLogger())
	tc.bft.Phase = phase
	require.NoError(t, err)
	return
}

func newTestValSet(t *testing.T, numValidators int) (valSet ValSet, valKeys []crypto.PrivateKeyI) {
	var err error
	consensusValidators := lib.ConsensusValidators{}
	for i := 0; i < numValidators; i++ {
		votingPower := 1000000
		if i == 0 {
			votingPower = 1000002 // slightly weight the first validator so 2/3 validators can pass the +2/3 maj
		}
		key := newDeterministicConsensusKey(t, i)
		consensusValidators.ValidatorSet = append(consensusValidators.ValidatorSet, &lib.ConsensusValidator{
			PublicKey:   key.PublicKey().Bytes(),
			VotingPower: uint64(votingPower),
			NetAddress:  fmt.Sprintf("http://localhost:8%d", i),
		})
		valKeys = append(valKeys, key)
	}
	valSet, err = lib.NewValidatorSet(&consensusValidators)
	require.NoError(t, err)
	return
}

func newDeterministicConsensusKey(t *testing.T, i int) crypto.PrivateKeyI {
	keys := []string{
		"02853a101301cd7019b78ffa1186842dd93923e563b8ae22e2ab33ae889b23ee",
		"1c6a244fbdf614acb5f0d00a2b56ffcbe2aa23dabd66365dffcd3f06491ae50f",
		"2b38b94c10159d63a12cb26aca4b0e76070a987d49dd10fc5f526031e05801da",
		"31e868f74134032eacba191ca529115c64aa849ac121b75ca79b37420a623036",
		"479839d3edbd0eefa60111db569ded6a1a642cc84781600f0594bd8d4a429319",
		"51eb5eb6eca0b47c8383652a6043aadc66ddbcbe240474d152f4d9a7439eae42",
		"637cb8e916bba4c1773ed34d89ebc4cb86e85c145aea5653a58de930590a2aa4",
		"7235e5757e6f52e6ae4f9e20726d9c514281e58e839e33a7f667167c524ff658"}
	key, err := crypto.NewBLSPrivateKeyFromString(keys[i])
	require.NoError(t, err)
	return key
}
