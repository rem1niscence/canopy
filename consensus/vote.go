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
		votesByRound          VotesByRound
		pacemakerVotesByRound []*VoteSet // one set of view votes per height; round -> seenRoundVote
	}

	VotesByRound   map[uint64]VotesByPhase  // round -> VotesByPhase
	VotesByPhase   map[Phase]VotesByPayload // phase -> VotesByPayload
	VotesByPayload map[string]*VoteSet      // payload -> VoteSet
	VoteSet        struct {
		vote              *Message
		lastDoubleSigners lib.DoubleSignEvidences
		badProposers      lib.BadProposerEvidences
		highQC            *QC
		multiKey          crypto.MultiPublicKeyI
		totalVotedPower   uint64
		minPowerFor23Maj  uint64
	}
)

func NewVotesForHeight() VotesForHeight {
	const maxRounds = 1000
	return VotesForHeight{
		Mutex:                 sync.Mutex{},
		votesByRound:          make(VotesByRound),
		pacemakerVotesByRound: make([]*VoteSet, maxRounds),
	}
}

func (v *VotesForHeight) NewHeight() {
	v.Lock()
	defer v.Unlock()
	v.votesByRound = make(VotesByRound)
	v.pacemakerVotesByRound = make([]*VoteSet, 0)
}

func (v *VotesForHeight) NewRound(round uint64) {
	v.Lock()
	defer v.Unlock()
	rvs := make(VotesByPhase)
	rvs[ElectionVote] = make(VotesByPayload)
	rvs[ProposeVote] = make(VotesByPayload)
	rvs[PrecommitVote] = make(VotesByPayload)
	v.votesByRound[round] = rvs
	v.pacemakerVotesByRound = make([]*VoteSet, 0)
}

func (v *VotesForHeight) AddVote(proposer []byte, height uint64, vote *Message, vals, lastVals ValSet) lib.ErrorI {
	v.Lock()
	defer v.Unlock()
	if vote.Qc.Header.Phase == RoundInterrupt {
		return v.addPacemakerVote(vote, vals)
	}
	vs, err := v.getVoteSet(vote)
	if err != nil {
		return err
	}
	if err = v.handleHighQCAndEvidence(proposer, height, vote, vs, vals, lastVals); err != nil {
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
	for _, vs := range v.votesByRound[view.Round][view.Phase-1] {
		if has23maj := vs.totalVotedPower >= vs.minPowerFor23Maj; has23maj {
			m = vs.vote
			m.HighQc, m.LastDoubleSignEvidence, m.BadProposerEvidence = vs.highQC, vs.lastDoubleSigners.DSE, vs.badProposers.BPE
			signature, er := vs.multiKey.AggregateSignatures()
			if er != nil {
				return nil, nil, ErrAggregateSignature(er)
			}
			return m, &lib.AggregateSignature{Signature: signature, Bitmap: vs.multiKey.Bitmap()}, nil
		}
	}
	return nil, nil, lib.ErrNoMaj23()
}

// Pacemaker returns the highest round where 2/3rds majority have previously signed they were on
func (v *VotesForHeight) Pacemaker() (round uint64) {
	v.Lock()
	defer v.Unlock()
	for i := len(v.pacemakerVotesByRound) - 1; i >= 0; i-- {
		vs := v.pacemakerVotesByRound[i]
		if vs == nil {
			continue
		}
		if has23maj := vs.totalVotedPower >= vs.minPowerFor23Maj; has23maj {
			return round
		}
	}
	return 0
}

func (v *VotesForHeight) getVoteSet(vote *Message) (*VoteSet, lib.ErrorI) {
	r, p := vote.Qc.Header.Round, vote.Qc.Header.Phase
	if _, ok := v.votesByRound[r]; !ok {
		v.votesByRound[r] = make(VotesByPhase)
	}
	rvs := v.votesByRound[r]
	bz, err := vote.SignBytes()
	if err != nil {
		return nil, err
	}
	key := lib.BytesToString(bz)
	if _, ok := rvs[p]; !ok {
		rvs[p] = make(VotesByPayload)
	}
	pvs := rvs[p]
	if _, ok := pvs[key]; !ok {
		pvs[key] = new(VoteSet)
	}
	return pvs[key], nil
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
	vs.totalVotedPower += val.VotingPower
	if er = vs.multiKey.AddSigner(vote.Signature.Signature, val.Index); er != nil {
		return ErrUnableToAddSigner(er)
	}
	vs.vote = vote
	return nil
}

func (v *VotesForHeight) handleHighQCAndEvidence(proposer []byte, height uint64, vote *Message, vs *VoteSet, vals, lastVals ValSet) lib.ErrorI {
	// Replicas sending in highQC & evidences to proposer during election vote
	if vote.Qc.Header.Phase == ElectionVote {
		if vote.HighQc != nil {
			if err := vote.HighQc.CheckHighQC(height, vals); err != nil {
				return err
			}
			if vs.highQC == nil || vs.highQC.Header.Less(vote.Header) {
				vs.highQC = vote.HighQc
			}
		}
		for _, evidence := range vote.LastDoubleSignEvidence {
			vs.lastDoubleSigners.Add(height, vals, lastVals, evidence)
		}
		for _, evidence := range vote.BadProposerEvidence {
			vs.badProposers.Add(proposer, height, vals, evidence)
		}
	}
	return nil
}

func (v *VotesForHeight) addPacemakerVote(vote *Message, vals ValSet) (err lib.ErrorI) {
	vs := v.pacemakerVotesByRound[vote.Qc.Header.Round]
	err = v.addVote(vote, vs, vals)
	if err != nil {
		return err
	}
	v.pacemakerVotesByRound[vote.Qc.Header.Round] = vs
	return
}
