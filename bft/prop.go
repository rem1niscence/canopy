package bft

import (
	"bytes"
	"github.com/ginchuco/ginchu/lib"
)

// PROPOSALS RECEIVED FROM PROPOSER FOR CURRENT HEIGHT
type ProposalsForHeight map[uint64]map[string][]*Message // [ROUND][PHASE] -> Proposal(s) < ELECTION is multi-proposals >

// AddProposal saves a validated proposal from the Leader in memory
func (c *Consensus) AddProposal(m *Message) lib.ErrorI {
	proposals, found := c.Proposals[m.Header.Round]
	if !found {
		proposals = make(map[string][]*Message)
	}
	phase := m.Header.Phase
	props := proposals[phaseToString(phase)]
	for _, msg := range props {
		if phase != Election || bytes.Equal(msg.Signature.PublicKey, m.Signature.PublicKey) {
			return ErrDuplicateProposerMessage()
		}
	}
	proposals[phaseToString(phase)] = append(props, m)
	c.Proposals[m.Header.Round] = proposals
	return nil
}

// GetProposal retrieves a proposal from the leader at the latest View
func (c *Consensus) GetProposal() *Message { return c.getProposal(c.Round, c.Phase) }
func (c *Consensus) getProposal(round uint64, phase Phase) *Message {
	proposal, found := c.Proposals[round][phaseToString(phase-1)]
	if !found {
		return nil
	}
	return proposal[0]
}

// GetElectionCandidates() retrieves ELECTION messages, verifies, and returns the candidate(s)
func (c *Consensus) GetElectionCandidates() (candidates []VRFCandidate) {
	roundProposal := c.Proposals[c.Round]
	for _, m := range roundProposal[phaseToString(Election)] {
		vrf := m.GetVrf()
		v, err := c.ValidatorSet.GetValidator(vrf.PublicKey)
		if err != nil {
			c.log.Errorf("an error occurred retrieving the Validator from the ValSet: %s", err.Error())
			continue
		}
		c.SortitionData.VotingPower = v.VotingPower
		out, isCandidate := VerifyCandidate(&SortitionVerifyParams{
			SortitionData: c.SortitionData,
			Signature:     vrf.Signature,
			PublicKey:     v.PublicKey,
		})
		if isCandidate {
			candidates = append(candidates, VRFCandidate{
				PublicKey: v.PublicKey,
				Out:       out,
			})
		}
	}
	return candidates
}
