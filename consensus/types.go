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

type QuorumCertificate = lib.QuorumCertificate
type Phase = lib.Phase

type HeightLeaderMessages struct {
	sync.Mutex
	roundLeaderMessages map[uint64]RoundLeaderMessages
	partialQCs          map[string]*QuorumCertificate // track for double sign evidence
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
		cpy[i] = &Message{
			Header:                 p.Header,
			Vrf:                    p.Vrf,
			Qc:                     p.Qc,
			HighQc:                 p.HighQc,
			LastDoubleSignEvidence: p.LastDoubleSignEvidence,
			BadProposerEvidence:    p.BadProposerEvidence,
			Signature:              p.Signature,
		}
	}
	return cpy
}

func (hlm *HeightLeaderMessages) NewHeight() {
	hlm.Lock()
	defer hlm.Unlock()
	hlm.roundLeaderMessages = make(map[uint64]RoundLeaderMessages)
	hlm.partialQCs = make(map[string]*QuorumCertificate)
}

func (hlm *HeightLeaderMessages) NewRound(round uint64, multiKey crypto.MultiPublicKeyI) {
	hlm.Lock()
	defer hlm.Unlock()
	rlm := hlm.roundLeaderMessages[round]
	rlm.Election = make([]*Message, 0)
	rlm.Messages = make(PhaseLeaderMessage)
	rlm.MultiKey = multiKey.Copy()
	hlm.roundLeaderMessages[round] = rlm
}

type ElectionMessages []*Message

func (m ElectionMessages) GetCandidates(vs lib.ValidatorSetWrapper, data SortitionData) ([]VRFCandidate, lib.ErrorI) {
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
	if phase == lib.Phase_ELECTION {
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

func (hlm *HeightLeaderMessages) GetLeaderMessage(view *lib.View) *Message {
	hlm.Lock()
	defer hlm.Unlock()
	view.Phase--
	rlm := hlm.roundLeaderMessages[view.Round]
	return rlm.Messages[view.Phase]
}

func (hlm *HeightLeaderMessages) AddIncompleteQC(message *Message) lib.ErrorI {
	hlm.Lock()
	defer hlm.Unlock()
	bz, err := lib.Marshal(message.Qc)
	if err != nil {
		return err
	}
	hlm.partialQCs[lib.BytesToString(bz)] = message.Qc
	return nil
}
func (hlm *HeightLeaderMessages) GetByzantineEvidence(
	app lib.App, v *lib.View, vs lib.ValidatorSetWrapper,
	leaderKey []byte, hvs *HeightVoteSet, selfIsLeader bool) *BE {
	bpe := hlm.GetBadProposerEvidence(v, vs, leaderKey)
	dse := hlm.GetDoubleSignEvidence(app, v, hvs, selfIsLeader)
	if bpe == nil && dse == nil {
		return nil
	}
	return &lib.ByzantineEvidence{
		DSE: dse,
		BPE: bpe,
	}
}

func (hlm *HeightLeaderMessages) GetDoubleSignEvidence(app lib.App, view *lib.View, hvs *HeightVoteSet, selfIsLeader bool) DSE {
	hlm.Lock()
	defer hlm.Unlock()
	hvs.Lock()
	defer hvs.Unlock()
	dse := make(lib.DoubleSignEvidences, 0)
	for _, p := range hlm.partialQCs {
		rlm := hlm.roundLeaderMessages[p.Header.Round]
		if rlm.Messages == nil {
			continue
		}
		message := rlm.Messages[p.Header.Phase]
		if message == nil {
			continue
		}
		dse.Add(app, &lib.DoubleSignEvidence{
			VoteA: message.Qc,
			VoteB: p,
		})
	}
	if !selfIsLeader {
		v := view.Copy()
		for round := v.Round - 1; ; round-- {
			rvs := hvs.roundVoteSets[round]
			if rvs == nil {
				continue
			}
			ev := rvs[lib.Phase_ELECTION_VOTE]
			if ev == nil {
				continue
			}
			rlm := hlm.roundLeaderMessages[round]
			if rlm.Messages == nil {
				continue
			}
			message := rlm.Messages[lib.Phase_ELECTION_VOTE]
			if message == nil {
				continue
			}
			for _, voteSet := range ev {
				if voteSet.vote != nil {
					dse.Add(app, &lib.DoubleSignEvidence{
						VoteA: message.Qc,
						VoteB: voteSet.vote.Qc,
					})
				}
			}
			if round == 0 {
				break
			}
		}
	}
	doubleSigners, err := dse.GetDoubleSigners(app)
	if err != nil {
		return nil
	}
	if len(doubleSigners) == 0 {
		return nil
	}
	return dse
}

func (hlm *HeightLeaderMessages) GetBadProposerEvidence(view *lib.View, vs lib.ValidatorSetWrapper, leaderKey []byte) BPE {
	hlm.Lock()
	defer hlm.Unlock()
	bpe := make(lib.BadProposerEvidences, 0)
	for r := view.Round - 1; ; r-- {
		v := view.Copy()
		v.Round = r
		v.Phase = lib.Phase_PROPOSE_VOTE
		msg := hlm.GetLeaderMessage(v)
		if msg != nil && msg.Qc != nil {
			bpe.Add(leaderKey, view.Height, vs, &lib.BadProposerEvidence{ElectionVoteQc: msg.Qc})
		}
		if r == 0 {
			break
		}
	}
	if len(bpe) == 0 {
		return nil
	}
	return bpe
}

type HeightVoteSet struct {
	sync.Mutex
	roundVoteSets  map[uint64]RoundVoteSet
	pacemakerVotes []VoteSet // one set of view votes per height; round -> seenRoundVote
}

type RoundVoteSet map[Phase]PhaseVoteSet // ELECTION_VOTE, PROPOSE_VOTE, PRECOMMIT_VOTE
type PhaseVoteSet map[string]VoteSet     // vote -> VoteSet TODO why would there be different votes?

type VoteSet struct {
	vote                  *Message
	lastDoubleSigners     lib.DoubleSignEvidences
	badProposers          lib.BadProposerEvidences
	highQC                *QuorumCertificate
	multiKey              crypto.MultiPublicKeyI
	totalVotedPower       string
	minimumPowerForQuorum string // 2f+1
}

func (hvs *HeightVoteSet) NewHeight() {
	hvs.Lock()
	defer hvs.Unlock()
	hvs.roundVoteSets = make(map[uint64]RoundVoteSet)
	hvs.pacemakerVotes = make([]VoteSet, 0)
}

func (hvs *HeightVoteSet) NewRound(round uint64) {
	hvs.Lock()
	defer hvs.Unlock()
	rvs := make(RoundVoteSet)
	rvs[lib.Phase_ELECTION_VOTE] = make(PhaseVoteSet)
	rvs[lib.Phase_PROPOSE_VOTE] = make(PhaseVoteSet)
	rvs[lib.Phase_PRECOMMIT_VOTE] = make(PhaseVoteSet)
	hvs.roundVoteSets[round] = rvs
	hvs.pacemakerVotes = make([]VoteSet, 0)
}

func (hvs *HeightVoteSet) AddVote(app lib.App, leaderKey []byte, view *lib.View, vote *Message, v lib.ValidatorSetWrapper) lib.ErrorI {
	hvs.Lock()
	defer hvs.Unlock()
	var vs VoteSet
	var key string
	val, err := v.GetValidator(vote.Signature.PublicKey)
	if err != nil {
		return err
	}
	phase, round := vote.Qc.Header.Phase, vote.Qc.Header.Round
	if phase == lib.Phase_ROUND_INTERRUPT {
		vs = hvs.pacemakerVotes[round]
		vs, err = hvs.addVote(app, leaderKey, view, vote, vs, val, v)
		if err != nil {
			return err
		}
		hvs.pacemakerVotes[round] = vs
	} else {
		rvs := hvs.roundVoteSets[round]
		bz, _ := vote.SignBytes()
		key = lib.BytesToString(bz)
		pvs := rvs[phase]
		vs, err = hvs.addVote(app, leaderKey, view, vote, pvs[key], val, v)
		if err != nil {
			return err
		}
		pvs[key] = vs
		rvs[phase] = pvs
		hvs.roundVoteSets[round] = rvs
	}
	return nil
}

func (hvs *HeightVoteSet) addVote(app lib.App, leaderKey []byte, view *lib.View, vote *Message, vs VoteSet, val *lib.ValidatorWrapper, v lib.ValidatorSetWrapper) (voteSet VoteSet, err lib.ErrorI) {
	if vs.multiKey == nil {
		vs.multiKey = v.Key.Copy()
	}
	enabled, er := vs.multiKey.SignerEnabledAt(val.Index)
	if er != nil {
		return voteSet, lib.ErrInvalidValidatorIndex()
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
	if view.Phase == lib.Phase_ELECTION_VOTE {
		// Replicas sending in highQC & evidences to leader during election vote
		if vote.HighQc != nil {
			if err = vote.HighQc.CheckHighQC(view, v); err != nil {
				return vs, err
			}
			if vs.highQC == nil || vs.highQC.Header.Less(vote.Header) {
				vs.highQC = vote.HighQc
			}
		}
		for _, evidence := range vote.LastDoubleSignEvidence {
			if vs.lastDoubleSigners == nil {
				vs.lastDoubleSigners = make([]*lib.DoubleSignEvidence, 0)
			}
			vs.lastDoubleSigners.Add(app, evidence)
		}
		for _, evidence := range vote.BadProposerEvidence {
			if vs.badProposers == nil {
				vs.badProposers = make([]*lib.BadProposerEvidence, 0)
			}
			vs.badProposers.Add(leaderKey, view.Height, v, evidence)
		}
	}
	return vs, nil
}

func (hvs *HeightVoteSet) GetMaj23(view *lib.View) (p *Message, sig, bitmap []byte, err lib.ErrorI) {
	hvs.Lock()
	defer hvs.Unlock()
	view.Phase--
	rvs := hvs.roundVoteSets[view.Round]
	pvs := rvs[view.Phase]
	for _, votes := range pvs {
		has23maj, _ := lib.StringsGTE(votes.totalVotedPower, votes.minimumPowerForQuorum)
		if has23maj {
			votes.vote.HighQc = votes.highQC
			votes.vote.LastDoubleSignEvidence = votes.lastDoubleSigners
			votes.vote.BadProposerEvidence = votes.badProposers
			signature, er := votes.multiKey.AggregateSignatures()
			if er != nil {
				return nil, nil, nil, ErrAggregateSignature(er)
			}
			return votes.vote, signature, votes.multiKey.Bitmap(), nil
		}
	}
	return nil, nil, nil, lib.ErrNoMaj23()
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

func (x *Message) SignBytes() (signBytes []byte, err lib.ErrorI) {
	switch {
	case x.IsLeaderMessage():
		return lib.Marshal(&Message{
			Header: x.Header,
			Vrf:    x.Vrf,
			Qc: &QuorumCertificate{
				Header:          x.Qc.Header,
				Block:           x.Qc.Block,
				LeaderPublicKey: x.Qc.LeaderPublicKey,
				Signature:       x.Qc.Signature,
			},
			HighQc:                 x.HighQc,
			LastDoubleSignEvidence: x.LastDoubleSignEvidence,
			BadProposerEvidence:    x.BadProposerEvidence,
		})
	case x.IsReplicaMessage():
		return lib.Marshal(&QuorumCertificate{
			Header:          x.Qc.Header,
			Block:           x.Qc.Block,
			LeaderPublicKey: x.Qc.LeaderPublicKey,
		})
	case x.IsPacemakerMessage():
		return lib.Marshal(&Message{Header: x.Header})
	default:
		return nil, ErrUnknownConsensusMsg(x)
	}
}

func (x *Message) Check(view *lib.View, vs lib.ValidatorSetWrapper) (isPartialQC bool, err lib.ErrorI) {
	if x == nil {
		return false, ErrEmptyLeaderMessage()
	}
	if err = checkSignature(x.Signature, x); err != nil {
		return false, err
	}
	if err = x.Header.Check(view); err != nil {
		return false, err
	}
	switch {
	case x.IsLeaderMessage():
		if x.Header.Phase != lib.Phase_ELECTION {
			isPartialQC, err = x.Qc.Check(view, vs)
			if err != nil {
				return false, err
			}
			if err = x.Qc.Block.Check(); err != nil {
				return false, err
			}
		}
		switch x.Header.Phase {
		case lib.Phase_ELECTION:
			if err = checkSignatureBasic(x.Vrf); err != nil {
				return false, err
			}
			if !bytes.Equal(x.Signature.PublicKey, x.Vrf.PublicKey) {
				return false, ErrMismatchPublicKeys()
			}
		case lib.Phase_PROPOSE:
			if len(x.Qc.LeaderPublicKey) != crypto.BLS12381PubKeySize {
				return false, lib.ErrInvalidLeaderPublicKey()
			}
		case lib.Phase_PRECOMMIT, lib.Phase_COMMIT: // nothing
		}
	case x.IsReplicaMessage():
		isPartialQC, err = x.Qc.Check(view, vs)
		if err != nil {
			return false, err
		}
		if isPartialQC {
			return false, lib.ErrNoMaj23()
		}
		if x.Qc.Header.Phase == lib.Phase_PROPOSE_VOTE {
			if !bytes.Equal(x.Signature.PublicKey, x.Qc.LeaderPublicKey) {
				return false, ErrMismatchPublicKeys()
			}
		}
		switch x.Qc.Header.Phase {
		case lib.Phase_ELECTION_VOTE:
			if len(x.Qc.LeaderPublicKey) != crypto.BLS12381PubKeySize {
				return false, lib.ErrInvalidLeaderPublicKey()
			}
		case lib.Phase_PROPOSE_VOTE, lib.Phase_PRECOMMIT_VOTE:
			if err := x.Qc.Block.Check(); err != nil {
				return false, err
			}
		}
	default:
		if !x.IsPacemakerMessage() {
			return false, ErrUnknownConsensusMsg(x)
		}
	}
	return
}

func (x *Message) Sign(privateKey crypto.PrivateKeyI) lib.ErrorI {
	bz, err := x.SignBytes()
	if err != nil {
		return err
	}
	x.Signature = new(lib.Signature)
	x.Signature.PublicKey = privateKey.PublicKey().Bytes()
	x.Signature.Signature = privateKey.Sign(bz)
	return nil
}

func (x *Message) IsReplicaMessage() bool {
	if x.Header != nil {
		return false
	}
	h := x.Qc.Header
	return h.Phase == lib.Phase_ELECTION_VOTE || h.Phase == lib.Phase_PROPOSE_VOTE || h.Phase == lib.Phase_PRECOMMIT_VOTE
}

func (x *Message) IsLeaderMessage() bool {
	h := x.Header
	if h == nil {
		return false
	}
	return h.Phase == lib.Phase_ELECTION || h.Phase == lib.Phase_PROPOSE || h.Phase == lib.Phase_PRECOMMIT || h.Phase == lib.Phase_COMMIT
}

func (x *Message) IsPacemakerMessage() bool {
	h := x.Header
	if h == nil {
		return false
	}
	return h.Phase == lib.Phase_ROUND_INTERRUPT
}

func checkSignature(signature *lib.Signature, sb lib.SignByte) lib.ErrorI {
	if err := checkSignatureBasic(signature); err != nil {
		return err
	}
	publicKey, err := lib.PublicKeyFromBytes(signature.PublicKey)
	if err != nil {
		return err
	}
	msg, err := sb.SignBytes()
	if err != nil {
		return err
	}
	if !publicKey.VerifyBytes(msg, signature.Signature) {
		return ErrInvalidPartialSignature()
	}
	return nil
}

func checkSignatureBasic(signature *lib.Signature) lib.ErrorI {
	if signature == nil || len(signature.PublicKey) == 0 || len(signature.Signature) == 0 {
		return ErrPartialSignatureEmpty()
	}
	if len(signature.PublicKey) != crypto.BLS12381PubKeySize {
		return ErrInvalidPublicKey()
	}
	if len(signature.Signature) != crypto.BLS12381SignatureSize {
		return ErrInvalidSignatureLength()
	}
	return nil
}
