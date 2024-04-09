package consensus

import (
	"bytes"
	"github.com/ginchuco/ginchu/crypto"
	lib "github.com/ginchuco/ginchu/types"
	"sync"
)

const (
	maxRounds = 10000
)

type HeightLeaderMessages struct {
	sync.Mutex
	roundLeaderMessages map[uint64]RoundLeaderMessages
}

type RoundLeaderMessages struct {
	Election []*Message         // ELECTION
	Messages PhaseLeaderMessage // PROPOSE, PRECOMMIT, COMMIT
	MultiKey crypto.MultiPublicKeyI
}

type PhaseLeaderMessage map[Phase]*Message

func (hlm *HeightLeaderMessages) GetElectionMessages(round uint64) ElectionMessages {
	hlm.Lock()
	defer hlm.Unlock()
	rlm := hlm.roundLeaderMessages[round]
	cpy := make([]*Message, len(rlm.Election))
	for i, p := range rlm.Election {
		if p == nil {
			continue
		}
		v := *p
		cpy[i] = &v
	}
	return cpy
}

type ElectionMessages []*Message

func (m ElectionMessages) GetCandidates(vs ValidatorSet, data SortitionData) ([]VRFCandidate, lib.ErrorI) {
	var candidates []VRFCandidate
	for _, msg := range m {
		vrf := msg.GetVrf()
		v, err := vs.GetValidator(vrf.PublicKey)
		if err != nil {
			return nil, err
		}
		data.VotingPower = v.VotingPower
		out, isCandidate := VerifyCandidate(&SortitionVerifyParams{
			SortitionData: data,
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

func (hlm *HeightLeaderMessages) AddLeaderMessage(message *Message) lib.ErrorI {
	hlm.Lock()
	defer hlm.Unlock()
	rlm := hlm.roundLeaderMessages[message.Header.Round]
	phase := message.Header.Phase
	if phase == Phase_ELECTION {
		for _, msg := range rlm.Election {
			if bytes.Equal(msg.Signature.PublicKey, message.Signature.PublicKey) {
				return ErrDuplicateLeaderMessage()
			}
		}
		rlm.Election = append(rlm.Election, message)
	} else {
		m := rlm.Messages[phase]
		if m != nil {
			return ErrDuplicateLeaderMessage()
		}
		rlm.Messages[phase] = message
	}
	hlm.roundLeaderMessages[message.Header.Round] = rlm
	return nil
}

func (hlm *HeightLeaderMessages) GetLeaderMessage(view *View) *Message {
	hlm.Lock()
	defer hlm.Unlock()
	view.Phase--
	rlm := hlm.roundLeaderMessages[view.Round]
	return rlm.Messages[view.Phase]
}

type HeightVoteSet struct {
	sync.Mutex
	roundVoteSets  map[uint64]RoundVoteSet
	pacemakerVotes []VoteSet // one set of view votes per height; round -> seenRoundVote
}

type RoundVoteSet map[Phase]PhaseVoteSet // ELECTION_VOTE, PROPOSE_VOTE, PRECOMMIT_VOTE
type PhaseVoteSet map[string]VoteSet     // vote -> VoteSet

type VoteSet struct {
	vote                  *Message
	highQC                *QuorumCertificate
	multiKey              crypto.MultiPublicKeyI
	totalVotedPower       string
	minimumPowerForQuorum string // 2f+1
}

func (hvs *HeightVoteSet) AddVote(view *View, vote *Message, v ValidatorSet) lib.ErrorI {
	hvs.Lock()
	defer hvs.Unlock()
	var vs VoteSet
	var key string
	val, err := v.GetValidator(vote.Signature.PublicKey)
	if err != nil {
		return err
	}
	phase, round := vote.Qc.Header.Phase, vote.Qc.Header.Round
	if phase == Phase_ROUND_INTERRUPT {
		vs = hvs.pacemakerVotes[round]
		vs, err = hvs.addVote(view, vote, vs, val, v)
		if err != nil {
			return err
		}
		hvs.pacemakerVotes[round] = vs
	} else {
		rvs := hvs.roundVoteSets[round]
		bz, _ := vote.SignBytes()
		key = lib.BytesToString(bz)
		pvs := rvs[phase]
		vs, err = hvs.addVote(view, vote, pvs[key], val, v)
		if err != nil {
			return err
		}
		pvs[key] = vs
		rvs[phase] = pvs
		hvs.roundVoteSets[round] = rvs
	}
	return nil
}

func (hvs *HeightVoteSet) addVote(view *View, vote *Message, vs VoteSet, val *Validator, valSet ValidatorSet) (voteSet VoteSet, err lib.ErrorI) {
	enabled, er := vs.multiKey.SignerEnabledAt(val.Index)
	if er != nil {
		return voteSet, ErrInvalidValidatorIndex()
	}
	if enabled {
		return voteSet, ErrDuplicateVote()
	}
	vs.totalVotedPower, err = lib.StringAdd(val.VotingPower, vs.totalVotedPower)
	if err != nil {
		return voteSet, err
	}
	if er = vs.multiKey.AddSigner(vote.Signature.Signature, val.Index); er != nil {
		return voteSet, ErrUnableToAddSigner(er)
	}
	vs.vote = vote
	if vote.HighQc != nil {
		if err = vote.HighQc.CheckHighQC(view, valSet); err != nil {
			return vs, err
		}
		if vs.highQC == nil || vs.highQC.Header.Less(vote.Header) {
			vs.highQC = vote.HighQc
		}
	}
	return vs, nil
}

func (hvs *HeightVoteSet) GetMaj23(view *View) (p *Message, sig, bitmap []byte, err lib.ErrorI) {
	hvs.Lock()
	defer hvs.Unlock()
	view.Phase--
	rvs := hvs.roundVoteSets[view.Round]
	pvs := rvs[view.Phase]
	for _, votes := range pvs {
		has23maj, _ := lib.StringsGTE(votes.totalVotedPower, votes.minimumPowerForQuorum)
		if has23maj {
			votes.vote.HighQc = votes.highQC
			signature, er := votes.multiKey.AggregateSignatures()
			if er != nil {
				return nil, nil, nil, ErrAggregateSignature(er)
			}
			return votes.vote, signature, votes.multiKey.Bitmap(), nil
		}
	}
	return nil, nil, nil, ErrNoMaj23()
}

// Pacemaker returns the highest round where 2/3rds majority have previously signed they were on
func (hvs *HeightVoteSet) Pacemaker() (round uint64) {
	hvs.Lock()
	defer hvs.Unlock()
	for i := len(hvs.pacemakerVotes) - 1; i >= 0; i-- {
		votes := hvs.pacemakerVotes[i]
		has23maj, _ := lib.StringsGTE(votes.totalVotedPower, votes.minimumPowerForQuorum)
		if has23maj {
			return round
		}
	}
	return 0
}
