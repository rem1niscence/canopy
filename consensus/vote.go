package consensus

import (
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"sync"
)

// TRACKING MESSAGES FROM REPLICAS
type (
	VotesForHeight struct {
		sync.Mutex
		votesByHeight
	}
	votesByHeight struct {
		VotesByRound          VotesByRound `json:"votesByRound"`
		PacemakerVotesByRound []*VoteSet   // one set of view votes per height; round -> seenRoundVote
	}
	VotesByRound   map[uint64]VotesByPhase   // round -> VotesByPhase
	VotesByPhase   map[string]VotesByPayload // phase -> VotesByPayload
	VotesByPayload map[string]*VoteSet       // payload -> VoteSet
	VoteSet        struct {
		Vote              *Message                 `json:"vote,omitempty"`
		LastDoubleSigners lib.DoubleSignEvidences  `json:"lastDoubleSigners,omitempty"`
		BadProposers      lib.BadProposerEvidences `json:"badProposers,omitempty"`
		HighQC            *QC                      `json:"highQC,omitempty"`
		TotalVotedPower   uint64                   `json:"totalVotedPower,omitempty"`
		MinPowerFor23Maj  uint64                   `json:"minPowerFor23Maj,omitempty"`
		multiKey          crypto.MultiPublicKeyI
	}
)

func NewVotesForHeight() VotesForHeight {
	const maxRounds = 1000
	return VotesForHeight{
		Mutex: sync.Mutex{},
		votesByHeight: votesByHeight{
			VotesByRound:          make(VotesByRound),
			PacemakerVotesByRound: make([]*VoteSet, maxRounds),
		},
	}
}

func (v *VotesForHeight) NewHeight() {
	v.Lock()
	defer v.Unlock()
	v.VotesByRound = make(VotesByRound)
	v.PacemakerVotesByRound = make([]*VoteSet, 0)
}

func (v *VotesForHeight) NewRound(round uint64) {
	v.Lock()
	defer v.Unlock()
	rvs := make(VotesByPhase)
	rvs[phaseString(ElectionVote)] = make(VotesByPayload)
	rvs[phaseString(ProposeVote)] = make(VotesByPayload)
	rvs[phaseString(PrecommitVote)] = make(VotesByPayload)
	v.VotesByRound[round] = rvs
	v.PacemakerVotesByRound = make([]*VoteSet, 0)
}

func (v *VotesForHeight) AddVote(proposer []byte, height uint64, vote *Message,
	vals ValSet, loadValSet func(height uint64) (lib.ValidatorSet, lib.ErrorI),
	minEvidenceHeight uint64, loadEvidence func(height uint64) (*lib.DoubleSigners, lib.ErrorI)) lib.ErrorI {
	v.Lock()
	defer v.Unlock()
	if vote.Qc.Header.Phase == RoundInterrupt {
		return v.addPacemakerVote(vote, vals)
	}
	vs, err := v.getVoteSet(vote, vals.MinimumMaj23)
	if err != nil {
		return err
	}
	if err = v.handleHighQCAndEvidence(proposer, height, vote, vs, vals, loadValSet, minEvidenceHeight, loadEvidence); err != nil {
		return err
	}
	if err = v.addVote(vote, vs, vals); err != nil {
		return err
	}
	return nil
}

func (v *VotesForHeight) GetMaj23(view *lib.View) (m *Message, sig *lib.AggregateSignature, err lib.ErrorI) {
	v.Lock()
	defer v.Unlock()
	for _, vs := range v.VotesByRound[view.Round][phaseString(view.Phase-1)] {
		if has23maj := vs.TotalVotedPower >= vs.MinPowerFor23Maj; has23maj {
			m = vs.Vote
			m.HighQc, m.LastDoubleSignEvidence, m.BadProposerEvidence = vs.HighQC, vs.LastDoubleSigners.DSE, vs.BadProposers.BPE
			signature, er := vs.multiKey.AggregateSignatures()
			if er != nil {
				return nil, nil, ErrAggregateSignature(er)
			}
			return m, &lib.AggregateSignature{Signature: signature, Bitmap: vs.multiKey.Bitmap()}, nil
		}
	}
	return nil, nil, lib.ErrNoMaj23()
}

func (v *VotesForHeight) getLeadingVote(view *lib.View, totalPower uint64) (m *Message, maxVotePercent uint64, maxVotes uint64) {
	maxVotes, maxVotePercent = 0, 0
	for _, vs := range v.VotesByRound[view.Round][phaseString(view.Phase-1)] {
		if vs.TotalVotedPower >= maxVotes {
			m, maxVotes, maxVotePercent = vs.Vote, vs.TotalVotedPower, lib.Uint64PercentageDiv(vs.TotalVotedPower, totalPower)
		}
	}
	return
}

// Pacemaker returns the highest round where 2/3rds majority have previously signed they were on
func (v *VotesForHeight) Pacemaker() (round uint64) {
	v.Lock()
	defer v.Unlock()
	for i := len(v.PacemakerVotesByRound) - 1; i >= 0; i-- {
		vs := v.PacemakerVotesByRound[i]
		if vs == nil {
			continue
		}
		if has23maj := vs.TotalVotedPower >= vs.MinPowerFor23Maj; has23maj {
			return round
		}
	}
	return 0
}

func (v *VotesForHeight) getVoteSet(vote *Message, min23Maj uint64) (vs *VoteSet, err lib.ErrorI) {
	r, p := vote.Qc.Header.Round, phaseString(vote.Qc.Header.Phase)
	if _, ok := v.VotesByRound[r]; !ok {
		v.VotesByRound[r] = make(VotesByPhase)
	}
	rvs := v.VotesByRound[r]
	bz, err := vote.SignBytes()
	if err != nil {
		return
	}
	key := lib.BytesToString(bz)
	if _, ok := rvs[p]; !ok {
		rvs[p] = make(VotesByPayload)
	}
	pvs := rvs[p]
	if _, ok := pvs[key]; !ok {
		pvs[key] = new(VoteSet)
	}
	vs = pvs[key]
	if vs.MinPowerFor23Maj == 0 {
		vs.MinPowerFor23Maj = min23Maj
	}
	return
}

func (v *VotesForHeight) addVote(vote *Message, vs *VoteSet, vals ValSet) lib.ErrorI {
	if vs.multiKey == nil {
		vs.multiKey = vals.Key.Copy()
	}
	val, err := vals.GetValidator(vote.Signature.PublicKey)
	if err != nil {
		return err
	}
	enabled, er := vs.multiKey.SignerEnabledAt(val.Index)
	if er != nil {
		return lib.ErrInvalidValidatorIndex()
	}
	if enabled {
		return ErrDuplicateVote()
	}
	vs.TotalVotedPower += val.VotingPower
	if er = vs.multiKey.AddSigner(vote.Signature.Signature, val.Index); er != nil {
		return ErrUnableToAddSigner(er)
	}
	vs.Vote = vote
	return nil
}

// LEADER AGGREGATION
func (v *VotesForHeight) handleHighQCAndEvidence(proposer []byte, height uint64, vote *Message,
	vs *VoteSet, vals ValSet, loadValSet func(height uint64) (lib.ValidatorSet, lib.ErrorI),
	minEvidenceHeight uint64, loadEvidence func(height uint64) (*lib.DoubleSigners, lib.ErrorI)) lib.ErrorI {
	// Replicas sending in highQC & evidences to proposer during election vote
	if vote.Qc.Header.Phase == ElectionVote {
		if vote.HighQc != nil {
			if err := vote.HighQc.CheckHighQC(height, vals); err != nil {
				return err
			}
			if vs.HighQC == nil || vs.HighQC.Header.Less(vote.Header) {
				vs.HighQC = vote.HighQc
			}
		}
		for _, evidence := range vote.LastDoubleSignEvidence {
			if err := evidence.CheckBasic(); err != nil {
				continue
			}
			valSet, err := loadValSet(evidence.VoteA.Header.Height)
			if err != nil {
				continue
			}
			vs.LastDoubleSigners.Add(loadValSet, loadEvidence, valSet, evidence, minEvidenceHeight)
		}
		for _, evidence := range vote.BadProposerEvidence {
			vs.BadProposers.Add(proposer, height, vals, evidence)
		}
	}
	return nil
}

func (v *VotesForHeight) addPacemakerVote(vote *Message, vals ValSet) (err lib.ErrorI) {
	vs := v.PacemakerVotesByRound[vote.Qc.Header.Round]
	err = v.addVote(vote, vs, vals)
	if err != nil {
		return err
	}
	v.PacemakerVotesByRound[vote.Qc.Header.Round] = vs
	return
}
