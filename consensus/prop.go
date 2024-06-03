package consensus

import (
	"bytes"
	"github.com/ginchuco/ginchu/lib"
	"sync"
)

// TRACKING MESSAGES FROM PROPOSER
type (
	ProposalsForHeight struct {
		sync.Mutex
		proposalsForHeight
	}
	proposalsForHeight struct {
		ProposalsByRound ProposalsByRound
		PartialQCs       PartialQCsByPayload
	}
	ProposalsByRound map[uint64]RoundProposals
	RoundProposals   struct {
		Election []*Message       `json:"1_ELECTION,omitempty"`
		Messages ProposalsByPhase `json:"messages,omitempty"`
	}
	ProposalsByPhase map[string]*Proposal // PROPOSE, PRECOMMIT, COMMIT
	Proposal         struct {
		Message          *Message `json:"message"`
		TotalVotedPower  uint64   `json:"totalVotedPower"`
		MinPowerFor23Maj uint64   `json:"minPowerFor23Maj"`
	}
	PartialQCsByPayload map[string]*QC
)

func NewProposalsForHeight() ProposalsForHeight {
	return ProposalsForHeight{sync.Mutex{}, proposalsForHeight{make(ProposalsByRound), make(PartialQCsByPayload)}}
}

func (p *ProposalsForHeight) NewHeight() {
	p.Lock()
	defer p.Unlock()
	p.PartialQCs = make(PartialQCsByPayload)
	p.ProposalsByRound = make(ProposalsByRound)
}

func (p *ProposalsForHeight) NewRound(round uint64) {
	p.Lock()
	defer p.Unlock()
	rlm := p.ProposalsByRound[round]
	rlm.Election, rlm.Messages = make([]*Message, 0), make(ProposalsByPhase)
	p.ProposalsByRound[round] = rlm
}

func (p *ProposalsForHeight) GetElectionCandidates(round uint64, vs ValSet, d *SortitionData) (candidates []VRFCandidate, e lib.ErrorI) {
	p.Lock()
	defer p.Unlock()
	rlm := p.ProposalsByRound[round]
	for _, m := range rlm.Election {
		vrf := m.GetVrf()
		v, err := vs.GetValidator(vrf.PublicKey)
		if err != nil {
			return nil, err
		}
		d.VotingPower = v.VotingPower
		out, isCandidate := VerifyCandidate(&SortitionVerifyParams{
			SortitionData: d,
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
	return candidates, nil
}

func (p *ProposalsForHeight) AddProposal(message *Message, vs ValSet) lib.ErrorI {
	p.Lock()
	defer p.Unlock()
	proposals, phase := p.ProposalsByRound[message.Header.Round], message.Header.Phase
	if phase == Election {
		for _, msg := range proposals.Election {
			if bytes.Equal(msg.Signature.PublicKey, message.Signature.PublicKey) {
				return ErrDuplicateProposerMessage()
			}
		}
		proposals.Election = append(proposals.Election, message)
	} else {
		key := phaseString(phase)
		if _, found := proposals.Messages[key]; found {
			return ErrDuplicateProposerMessage()
		}
		_, totalSigned, min23Maj, err := message.Qc.Signature.GetSignerInfo(vs)
		if err != nil {
			return err
		}
		proposals.Messages[key] = &Proposal{
			Message:          message,
			TotalVotedPower:  totalSigned,
			MinPowerFor23Maj: min23Maj,
		}
	}
	p.ProposalsByRound[message.Header.Round] = proposals
	return nil
}

func (p *ProposalsForHeight) GetProposal(view *lib.View) *Message {
	p.Lock()
	defer p.Unlock()
	return p.getProposal(view)
}

func (p *ProposalsForHeight) getProposal(view *lib.View) *Message {
	proposal, ok := p.ProposalsByRound[view.Round].Messages[phaseString(view.Phase-1)]
	if !ok {
		return nil
	}
	return proposal.Message
}

func (p *ProposalsForHeight) AddPartialQC(message *Message) (err lib.ErrorI) {
	p.Lock()
	defer p.Unlock()
	bz, err := lib.Marshal(message.Qc)
	if err != nil {
		return
	}
	p.PartialQCs[lib.BytesToString(bz)] = message.Qc
	return
}

func (p *ProposalsForHeight) GetByzantineEvidence(
	v *lib.View, vs ValSet, loadValSet func(height uint64) (lib.ValidatorSet, lib.ErrorI),
	getMinEvidenceHeight func() (uint64, lib.ErrorI), loadEvidence func(height uint64) (*lib.DoubleSigners, lib.ErrorI),
	loadCertificate func(height uint64) (*lib.QuorumCertificate, lib.ErrorI),
	proposerKey []byte, hvs *VotesForHeight, selfIsProposer bool) *BE {
	bpe := p.GetBPE(v, vs, proposerKey)
	minEvidenceHeight, err := getMinEvidenceHeight()
	if err != nil {
		return nil
	}
	dse := p.GetDSE(v, hvs, vs, minEvidenceHeight, loadEvidence, loadCertificate, loadValSet, selfIsProposer)
	if bpe == nil && dse == nil {
		return nil
	}
	return &lib.ByzantineEvidence{
		DSE: dse,
		BPE: bpe,
	}
}

func (p *ProposalsForHeight) GetBPE(view *lib.View, vs ValSet, proposerKey []byte) BPE {
	p.Lock()
	defer p.Unlock()
	e := lib.NewBPE(nil)
	for r := uint64(0); r < view.Round; r++ {
		v := view.Copy()
		v.Phase, v.Round = ProposeVote, r
		if msg := p.GetProposal(v); msg != nil && msg.Qc != nil {
			e.Add(proposerKey, view.Height, vs, &lib.BadProposerEvidence{ElectionVoteQc: msg.Qc})
		}
	}
	return e.BPE
}

func (p *ProposalsForHeight) GetDSE(view *lib.View, hvs *VotesForHeight, vs ValSet,
	minEvidenceHeight uint64, loadEvidence func(height uint64) (*lib.DoubleSigners, lib.ErrorI),
	loadCertificate func(height uint64) (*lib.QuorumCertificate, lib.ErrorI),
	loadValSet func(height uint64) (lib.ValidatorSet, lib.ErrorI), selfIsProposer bool) DSE {
	p.Lock()
	hvs.Lock()
	defer func() { p.Unlock(); hvs.Unlock() }()
	dse := lib.NewDSE(nil)
	p.addDSEByPartialQC(view.Height, vs, minEvidenceHeight, loadEvidence, loadCertificate, dse, loadValSet)
	p.addDSEByCandidate(view, hvs, dse, minEvidenceHeight, loadEvidence, loadValSet, selfIsProposer)
	if !dse.HasDoubleSigners() {
		return nil
	}
	return dse.DSE
}

func (p *ProposalsForHeight) addDSEByPartialQC(height uint64, vs ValSet,
	minEvidenceHeight uint64, loadEvidence func(height uint64) (*lib.DoubleSigners, lib.ErrorI),
	loadCertificate func(height uint64) (*lib.QuorumCertificate, lib.ErrorI),
	dse lib.DoubleSignEvidences, loadValSet func(height uint64) (lib.ValidatorSet, lib.ErrorI)) {
	for _, pQC := range p.PartialQCs { // REPLICA with two proposer messages for same (H,R,P) - the partial is the malicious one
		evidenceHeight := pQC.Header.Height
		if evidenceHeight == height {
			rlm := p.ProposalsByRound[pQC.Header.Round]
			if rlm.Messages == nil {
				continue
			}
			proposal := rlm.Messages[phaseString(pQC.Header.Phase)]
			if proposal == nil || proposal.Message == nil {
				continue
			}
			dse.Add(loadValSet, loadEvidence, vs, &lib.DoubleSignEvidence{
				VoteA: proposal.Message.Qc, // if both a partial and full exists
				VoteB: pQC,
			}, minEvidenceHeight)
		} else {
			if pQC.Header.Phase != PrecommitVote {
				continue // historically can only process precommit vote
			}
			certificate, err := loadCertificate(evidenceHeight)
			if err != nil {
				continue
			}
			valSet, err := loadValSet(evidenceHeight)
			if err != nil {
				continue
			}
			dse.Add(loadValSet, loadEvidence, valSet, &lib.DoubleSignEvidence{
				VoteA: certificate, // if both a partial and full exists
				VoteB: pQC,
			}, minEvidenceHeight)
		}
	}
}

func (p *ProposalsForHeight) addDSEByCandidate(
	view *lib.View, hvs *VotesForHeight, dse lib.DoubleSignEvidences,
	minEvidenceHeight uint64, loadEvidence func(height uint64) (*lib.DoubleSigners, lib.ErrorI),
	loadValSet func(height uint64) (lib.ValidatorSet, lib.ErrorI), selfIsProposer bool) {
	if !selfIsProposer { // CANDIDATE exposing double sign election
		for r := uint64(0); r < view.Round-1; r++ {
			rvs := hvs.VotesByRound[r]
			if rvs == nil {
				continue
			}
			ps := phaseString(ElectionVote)
			ev := rvs[ps]
			if ev == nil {
				continue
			}
			rlm := p.ProposalsByRound[r]
			if rlm.Messages == nil {
				continue
			}
			proposal := rlm.Messages[ps]
			if proposal == nil || proposal.Message == nil {
				continue
			}
			for _, voteSet := range ev {
				if voteSet.Vote != nil {
					valSet, err := loadValSet(voteSet.Vote.Header.Height)
					if err != nil {
						continue
					}
					dse.Add(loadValSet, loadEvidence, valSet, &lib.DoubleSignEvidence{
						VoteA: proposal.Message.Qc,
						VoteB: voteSet.Vote.Qc,
					}, minEvidenceHeight)
				}
			}
		}
	}
}
