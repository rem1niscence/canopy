package consensus

import (
	"bytes"
	"github.com/drand/kyber"
	"github.com/ginchuco/ginchu/crypto"
	lib "github.com/ginchuco/ginchu/types"
	"sync"
)

type HeightLeaderMessages struct {
	sync.Mutex
	roundLeaderMessages map[uint64]RoundLeaderMessages
}

func (hlm *HeightLeaderMessages) GetRound(round uint64) RoundLeaderMessages {
	return hlm.roundLeaderMessages[round]
}

func (hlm *HeightLeaderMessages) AddElectionMessage(message *ElectionMessage) lib.ErrorI {
	hlm.Lock()
	defer hlm.Unlock()
	rlm := hlm.roundLeaderMessages[message.Header.Round]
	for _, msg := range rlm.Election {
		if bytes.Equal(msg.CandidateSignature.PublicKey, message.CandidateSignature.PublicKey) {
			return ErrDuplicateLeaderMessage()
		}
	}
	rlm.Election = append(rlm.Election, message)
	hlm.roundLeaderMessages[message.Header.Round] = rlm
	return nil
}

func (hlm *HeightLeaderMessages) AddLeaderMessage(message *LeaderMessage) lib.ErrorI {
	hlm.Lock()
	defer hlm.Unlock()
	rlm := hlm.roundLeaderMessages[message.Header.Round]
	switch message.Header.Phase {
	case Phase_PROPOSE:
		if rlm.ProposeMessage != nil {
			return ErrDuplicateLeaderMessage()
		}
		rlm.ProposeMessage = message
	case Phase_PRECOMMIT:
		if rlm.PrecommitMessage != nil {
			return ErrDuplicateLeaderMessage()
		}
		rlm.PrecommitMessage = message
	case Phase_COMMIT:
		if rlm.CommitMessage != nil {
			return ErrDuplicateLeaderMessage()
		}
		rlm.CommitMessage = message
	}
	hlm.roundLeaderMessages[message.Header.Round] = rlm
	return nil
}

type RoundLeaderMessages struct {
	Election         []*ElectionMessage
	ProposeMessage   *LeaderMessage
	PrecommitMessage *LeaderMessage
	CommitMessage    *LeaderMessage
	MultiKey         crypto.MultiPublicKeyI
}

func (rlm *RoundLeaderMessages) GetElectionMessages() []*ElectionMessage {
	cpy := make([]*ElectionMessage, len(rlm.Election))
	for i, msg := range rlm.Election {
		cpy[i] = msg.Copy()
	}
	return cpy
}

func (rlm *RoundLeaderMessages) GetProposeMessage() *LeaderMessage {
	return rlm.ProposeMessage
}

func (rlm *RoundLeaderMessages) GetPrecommitMessage() *LeaderMessage {
	return rlm.PrecommitMessage
}

func (rlm *RoundLeaderMessages) GetCommitMessage() *LeaderMessage {
	return rlm.CommitMessage
}

type HeightVoteSet struct {
	sync.Mutex
	roundVoteSets  map[uint64]RoundVoteSet
	validatorViews []VoteSet // height -> used for pacemaker
}

func (hvs *HeightVoteSet) Add(vote *ReplicaMessage, v ValidatorSet) lib.ErrorI {
	hvs.Lock()
	defer hvs.Unlock()
	val, err := v.GetValidator(vote.PartialSignature.PublicKey)
	if err != nil {
		return err
	}
	rvs := hvs.roundVoteSets[vote.Header.Round]
	switch vote.Header.Phase {
	case Phase_ELECTION_VOTE:
		key := lib.BytesToString(vote.GetLeaderPublicKey())
		voteSet := rvs.Election[key]
		if err = voteSet.AddVote(vote.PartialSignature.Signature, val); err != nil {
			return err
		}
		rvs.Election[key] = voteSet
		return nil
	case Phase_PROPOSE_VOTE:
		key := lib.BytesToString(vote.GetBlock().BlockHeader.Hash)
		voteSet := rvs.Propose[key]
		if err = voteSet.AddVote(vote.PartialSignature.Signature, val); err != nil {
			return err
		}
		rvs.Propose[key] = voteSet
		return nil
	case Phase_PRECOMMIT_VOTE:
		key := lib.BytesToString(vote.GetBlock().BlockHeader.Hash)
		voteSet := rvs.Precommit[key]
		if err = voteSet.AddVote(vote.PartialSignature.Signature, val); err != nil {
			return err
		}
		rvs.Precommit[key] = voteSet
		return nil
	default:
		return ErrWrongPhase()
	}
}

func (hvs *HeightVoteSet) AddPacemakerMessage(p *PacemakerMessage, v ValidatorSet) lib.ErrorI {
	val, err := v.GetValidator(p.PartialSignature.PublicKey)
	if err != nil {
		return err
	}
	voteSet := hvs.validatorViews[p.Header.Round]
	if err = voteSet.AddVote(p.PartialSignature.Signature, val); err != nil {
		return err
	}
	hvs.validatorViews[p.Header.Round] = voteSet
	return nil
}

func (hvs *HeightVoteSet) GetRound(r uint64) RoundVoteSet {
	return hvs.roundVoteSets[r]
}

// Pacemaker returns the highest round where 2/3rds majority have previously signed they were on
func (hvs *HeightVoteSet) Pacemaker() (round uint64) {
	for i := len(hvs.validatorViews) - 1; i >= 0; i-- {
		if hvs.validatorViews[i].HasQuorum() {
			return round
		}
	}
	return 0
}

type RoundVoteSet struct {
	Election  map[string]VoteSet
	Propose   map[string]VoteSet
	Precommit map[string]VoteSet
}

type ValidatorView struct {
	Validator Validator
	View      *View
}

func (rvs *RoundVoteSet) GetElectionVotesFor(payload []byte) VoteSet {
	return rvs.Election[lib.BytesToString(payload)]
}

func (rvs *RoundVoteSet) GetQuorumForElection() (payload *ReplicaMessage, multiKey crypto.MultiPublicKeyI) {
	for _, votes := range rvs.Election {
		if votes.HasQuorum() {
			votes.payload.HighQC = nil
			votes.payload.PartialSignature = nil
			return votes.payload, votes.multiKey
		}
	}
	return
}

func (rvs *RoundVoteSet) GetProposeVotesFor(payload []byte) VoteSet {
	return rvs.Propose[lib.BytesToString(payload)]
}

func (rvs *RoundVoteSet) GetQuorumForPropose() (payload *ReplicaMessage, multiKey crypto.MultiPublicKeyI) {
	for _, votes := range rvs.Precommit {
		if votes.HasQuorum() {
			votes.payload.PartialSignature = nil
			return votes.payload, votes.multiKey
		}
	}
	return
}

func (rvs *RoundVoteSet) GetPrecommitVotesFor(payload []byte) VoteSet {
	return rvs.Precommit[lib.BytesToString(payload)]
}

func (rvs *RoundVoteSet) GetQuorumForPrecommit() (payload *ReplicaMessage, multiKey crypto.MultiPublicKeyI) {
	for _, votes := range rvs.Precommit {
		if votes.HasQuorum() {
			votes.payload.PartialSignature = nil
			return votes.payload, votes.multiKey
		}
	}
	return
}

type VoteSet struct {
	payload               *ReplicaMessage
	pacemakerPayload      *PacemakerMessage
	multiKey              crypto.MultiPublicKeyI
	totalVotedPower       string
	minimumPowerForQuorum string // 2f+1
}

func (vs *VoteSet) AddVote(signature []byte, val *Validator) (err lib.ErrorI) {
	enabled, er := vs.multiKey.SignerEnabledAt(val.Index)
	if er != nil {
		return ErrInvalidValidatorIndex()
	}
	if enabled {
		return ErrDuplicateVote()
	}
	vs.totalVotedPower, err = lib.StringAdd(val.VotingPower, vs.totalVotedPower)
	if err != nil {
		return err
	}
	if er = vs.multiKey.AddSigner(signature, val.Index); er != nil {
		return ErrUnableToAddSigner(er)
	}
	return nil
}

func (vs *VoteSet) Copy() *VoteSet {
	return &VoteSet{
		multiKey:        vs.multiKey.Copy(),
		totalVotedPower: vs.totalVotedPower,
	}
}

func (vs *VoteSet) HasQuorum() bool {
	hasQuorum, _ := lib.StringsGTE(vs.totalVotedPower, vs.minimumPowerForQuorum)
	return hasQuorum
}

type ValidatorSet struct {
	validatorSet          *lib.ValidatorSet
	key                   crypto.MultiPublicKeyI
	powerMap              map[string]Validator // public_key -> Validator
	totalPower            string
	minimumPowerForQuorum string // 2f+1
	numValidators         uint64
}

type Validator struct {
	PublicKey   crypto.PublicKeyI
	VotingPower string
	Index       int
}

func NewValidatorSet(validators *lib.ValidatorSet) (vs ValidatorSet, err lib.ErrorI) {
	totalPower, count := "", uint64(0)
	points, powerMap := make([]kyber.Point, 0), make(map[string]Validator)
	for i, v := range validators.ValidatorSet {
		point, er := crypto.NewBLSPointFromBytes(v.PublicKey)
		if err != nil {
			return ValidatorSet{}, lib.ErrPubKeyFromBytes(er)
		}
		points = append(points, point)
		powerMap[lib.BytesToString(v.PublicKey)] = Validator{
			PublicKey:   crypto.NewBLS12381PublicKey(point),
			VotingPower: v.VotingPower,
			Index:       i,
		}
		totalPower, err = lib.StringAdd(totalPower, v.VotingPower)
		if err != nil {
			return
		}
		count++
	}
	minimumPowerForQuorum, err := lib.StringReducePercentage(totalPower, 33)
	if err != nil {
		return
	}
	mpk, er := crypto.NewMultiBLSFromPoints(points, nil)
	if er != nil {
		return ValidatorSet{}, lib.ErrNewMultiPubKey(er)
	}
	return ValidatorSet{
		validatorSet:          validators,
		key:                   mpk,
		powerMap:              powerMap,
		totalPower:            totalPower,
		minimumPowerForQuorum: minimumPowerForQuorum,
		numValidators:         count,
	}, nil
}

func (vs *ValidatorSet) GetValidator(publicKey []byte) (*Validator, lib.ErrorI) {
	val, found := vs.powerMap[lib.BytesToString(publicKey)]
	if !found {
		return nil, ErrValidatorNotInSet(publicKey)
	}
	return &val, nil
}

func (vs *ValidatorSet) GetValidatorAtIndex(i int) (*Validator, lib.ErrorI) {
	if uint64(i) >= vs.numValidators {
		return nil, ErrInvalidValidatorIndex()
	}
	val := vs.validatorSet.ValidatorSet[i]
	publicKey, err := publicKeyFromBytes(val.PublicKey)
	if err != nil {
		return nil, err
	}
	return &Validator{
		PublicKey:   publicKey,
		VotingPower: val.VotingPower,
		Index:       i,
	}, nil
}

func publicKeyFromBytes(pubKey []byte) (crypto.PublicKeyI, lib.ErrorI) {
	publicKey, err := crypto.NewBLSPublicKeyFromBytes(pubKey)
	if err != nil {
		return nil, ErrPubKeyFromBytes(err)
	}
	return publicKey, nil
}
