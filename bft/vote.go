package bft

import (
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
)

// LEADER TRACKING AND AGGREGATING MESSAGES FROM REPLICAS
type (
	VotesForHeight map[uint64]map[string]map[string]*VoteSet // [Round] -> [Phase] -> [Payload-Hash] -> VoteSet
	VoteSet        struct {
		Vote            *Message               `json:"vote,omitempty"`
		TotalVotedPower uint64                 `json:"totalVotedPower,omitempty"`
		multiKey        crypto.MultiPublicKeyI // tracks and aggregates bls signatures from replicas
	}
)

func (v *VotesForHeight) NewRound(round uint64) {
	votesByPhase := make(map[string]map[string]*VoteSet)
	votesByPhase[phaseToString(ElectionVote)] = make(map[string]*VoteSet)
	votesByPhase[phaseToString(ProposeVote)] = make(map[string]*VoteSet)
	votesByPhase[phaseToString(PrecommitVote)] = make(map[string]*VoteSet)
	(*v)[round] = votesByPhase
}

func (c *Consensus) GetMajorityVote() (m *Message, sig *lib.AggregateSignature, err lib.ErrorI) {
	for _, voteSet := range c.Votes[c.View.Round][phaseToString(c.View.Phase-1)] {
		if has23maj := voteSet.TotalVotedPower >= c.ValidatorSet.MinimumMaj23; has23maj {
			signature, e := voteSet.multiKey.AggregateSignatures()
			if e != nil {
				return nil, nil, ErrAggregateSignature(e)
			}
			return voteSet.Vote, &lib.AggregateSignature{Signature: signature, Bitmap: voteSet.multiKey.Bitmap()}, nil
		}
	}
	return nil, nil, lib.ErrNoMaj23()
}

func (c *Consensus) GetLeadingVote() (m *Message, maxVotePercent uint64, maxVotes uint64) {
	for _, voteSet := range c.Votes[c.View.Round][phaseToString(c.View.Phase-1)] {
		if voteSet.TotalVotedPower >= maxVotes {
			m, maxVotes, maxVotePercent = voteSet.Vote, voteSet.TotalVotedPower, lib.Uint64PercentageDiv(voteSet.TotalVotedPower, c.ValidatorSet.TotalPower)
		}
	}
	return
}

func (c *Consensus) AddVote(vote *Message) lib.ErrorI {
	voteSet := c.getVoteSet(vote)
	if err := c.handleHighQCAndEvidence(vote); err != nil {
		return err
	}
	if err := c.addSigToVoteSet(vote, voteSet); err != nil {
		return err
	}
	return nil
}

func (c *Consensus) getVoteSet(vote *Message) (voteSet *VoteSet) {
	round, phase, ok := vote.Qc.Header.Round, phaseToString(vote.Qc.Header.Phase), false
	if _, ok = c.Votes[round]; !ok {
		c.Votes[round] = make(map[string]map[string]*VoteSet)
	}
	if _, ok = c.Votes[round][phase]; !ok {
		c.Votes[round][phase] = make(map[string]*VoteSet)
	}
	payload := crypto.HashString(vote.SignBytes())
	if voteSet, ok = c.Votes[round][phase][payload]; !ok {
		voteSet = &VoteSet{
			Vote:     vote,
			multiKey: c.ValidatorSet.Key.Copy(),
		}
		c.Votes[round][phase][payload] = voteSet
	}
	return
}

func (c *Consensus) addSigToVoteSet(vote *Message, voteSet *VoteSet) lib.ErrorI {
	val, err := c.ValidatorSet.GetValidator(vote.Signature.PublicKey)
	if err != nil {
		return err
	}
	enabled, er := voteSet.multiKey.SignerEnabledAt(val.Index)
	if er != nil {
		return lib.ErrInvalidValidatorIndex()
	}
	if enabled {
		return ErrDuplicateVote()
	}
	voteSet.TotalVotedPower += val.VotingPower
	if er = voteSet.multiKey.AddSigner(vote.Signature.Signature, val.Index); er != nil {
		return ErrUnableToAddSigner(er)
	}
	return nil
}

// LEADER AGGREGATION
func (c *Consensus) handleHighQCAndEvidence(vote *Message) lib.ErrorI {
	// Replicas sending in highQC & evidences to proposer during election vote
	if vote.Qc.Header.Phase == ElectionVote {
		if vote.HighQc != nil {
			if err := vote.HighQc.CheckHighQC(c.Height, c.ValidatorSet); err != nil {
				return err
			}
			if c.HighQC == nil || c.HighQC.Header.Less(vote.HighQc.Header) {
				c.HighQC = vote.HighQc
				c.Proposal = vote.HighQc.Proposal
			}
		}
		for _, evidence := range vote.LastDoubleSignEvidence {
			if err := c.AddDSE(&c.ByzantineEvidence.DSE, evidence); err != nil {
				return err
			}
		}
		for _, evidence := range vote.BadProposerEvidence {
			if err := c.AddBPE(&c.ByzantineEvidence.BPE, evidence); err != nil {
				return err
			}
		}
	}
	return nil
}
