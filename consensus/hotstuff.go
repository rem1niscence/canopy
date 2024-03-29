package consensus

import (
	"github.com/drand/kyber"
	"github.com/ginchuco/ginchu/crypto"
	lib "github.com/ginchuco/ginchu/types"
	"github.com/gogo/protobuf/proto"
)

/*
	NEWROUND
		- Leader receives +2/3 signatures from replicas confirming they're the leader
		- If a valid 'HighQC' is included in the NEW_ROUND then the Leader uses
			`HighQC.Block` as the proposal block, else they create a new block

	PREPARE (Block Proposal and Voting)
		- Leader sends QC containing the NEW_ROUND votes and the new proposal block to the replicas
		- Replicas check the validity of QC.Votes (NEWROUND votes) and QC.Block and send a signed vote back to the proposer

	PRECOMMIT (Voting on the Votes)
		- Leader receives +2/3 signatures from replicas confirming the validity of PREPARE-MSG and sends the votes to the replicas
		- Replicas check the validity of QC.Votes (PREPARE votes), set `HighQC`
		- Replicas run the VRF and (if a candidate) attach their VRF output to their vote and send a signed vote back to the proposer

	COMMIT (Locking)
		- Leader receives +2/3 signatures from replicas confirming the validity of PRECOMMIT-MSG and sends the votes to the replicas
		- Replicas check the validity of QC.Votes (PRECOMMIT votes), set `isLock=true`, see the lowest VRF Output (next leader),
			and send a signed vote back to the proposer

	DECIDE (Committing)
		- Leader receives +2/3 signatures from replicas confirming the validity of the COMMIT-MSG and sends the votes to the replicas
		- Replicas check the validity of QC.Votes and commit the block to storage

	ATTACKS:
		- Leader omits votes to attack replicas?

	NOTES:
		- HighQC is just used to synchronize everyone on Prepare

*/

type Hotstuff struct {
	// self
	PrivateKey crypto.PrivateKeyI
	PublicKey  crypto.PublicKeyI

	// hotstuff
	ValidatorSet    ValidatorSet
	LeaderPublicKey crypto.PublicKeyI
	View            View
	State           State
	PrepareQC       *QuorumCert // highQC
	PreCommitQC     *QuorumCert // lockQC

	// util
	log lib.LoggerI
}

type State struct {
	Height     uint64
	ViewNumber uint64
}

type View struct {
	Block          *lib.Block
	NewViewVote    *Tally
	PrepareVote    *Tally
	PreCommitVote  *Tally
	CommitVote     *Tally
	ReplicasHighQC []QuorumCert
}

type Tally struct {
	multiKey        crypto.MultiPublicKey
	totalVotedPower string
}

func NewTally(multiKey crypto.MultiPublicKey) *Tally {
	return &Tally{multiKey: multiKey}
}

func (h *Hotstuff) HandleMessage(message proto.Message) lib.ErrorI {
	if h.SelfIsLeader() {
		switch msg := message.(type) {
		case *NewView:
			return h.HandleNewViewMessage(msg)
		case *PrepareVote:
			return h.HandlePrepareVoteMessage(msg)
		case *PreCommitVote:
			return h.HandlePrecommitVoteMessage(msg)
		case *CommitVote:
			return h.HandleCommitVoteMessage(msg)
		default:
			return ErrUnknownConsensusMsg(msg)
		}
	} else { // replica
		switch msg := message.(type) {
		case *Prepare:
			return h.HandlePrepareMessage(msg)
		case *PreCommit:
			return h.HandlePrecommitMessage(msg)
		case *Commit:
			return h.HandleCommitMessage(msg)
		case *Decide:
			return h.HandleDecideMessage(msg)
		default:
			return ErrUnknownConsensusMsg(msg)
		}
	}
}

func (h *Hotstuff) InitializeNewView() {
	h.State.ViewNumber++
	h.View.NewViewVote = NewTally(h.ValidatorSet.key.Copy())
	h.View.PrepareVote = NewTally(h.ValidatorSet.key.Copy())
	h.View.PreCommitVote = NewTally(h.ValidatorSet.key.Copy())
	h.View.CommitVote = NewTally(h.ValidatorSet.key.Copy())
}

// Leader Message Handling

func (h *Hotstuff) HandleNewViewMessage(message *NewView) lib.ErrorI {

}

func (h *Hotstuff) HandlePrepareVoteMessage(message *PrepareVote) lib.ErrorI {

}

func (h *Hotstuff) HandlePrecommitVoteMessage(message *PreCommitVote) lib.ErrorI {

}

func (h *Hotstuff) HandleCommitVoteMessage(message *CommitVote) lib.ErrorI {

}

// Replica Message Handling

func (h *Hotstuff) HandlePrepareMessage(message *Prepare) lib.ErrorI {

}

func (h *Hotstuff) HandlePrecommitMessage(message *PreCommit) lib.ErrorI {

}

func (h *Hotstuff) HandleCommitMessage(message *Commit) lib.ErrorI {

}

func (h *Hotstuff) HandleDecideMessage(message *Decide) lib.ErrorI {

}

func (h *Hotstuff) SelfIsLeader() bool { return h.IsLeader(h.PublicKey) }
func (h *Hotstuff) IsLeader(publicKey crypto.PublicKeyI) bool {
	return publicKey.Equals(h.LeaderPublicKey)
}

type ValidatorSet struct {
	key                   crypto.MultiPublicKey
	powerMap              map[string]Validator // public_key -> Validator
	totalPower            string
	minimumPowerForQuorum string // 2f+1 aka 66%+
	numValidators         int
}

type Validator struct {
	PublicKey   crypto.PublicKeyI
	VotingPower string
}

func NewValidatorSet(validators *lib.ValidatorSet) (vs ValidatorSet, err lib.ErrorI) {
	totalPower, count := "", 0
	points, powerMap := make([]kyber.Point, 0), make(map[string]Validator)
	for _, v := range validators.ValidatorSet {
		point, er := crypto.NewBLSPointFromBytes(v.PublicKey)
		if err != nil {
			return ValidatorSet{}, lib.ErrPubKeyFromBytes(er)
		}
		points = append(points, point)
		powerMap[lib.BytesToString(v.PublicKey)] = Validator{
			PublicKey:   crypto.NewBLS12381PublicKey(point),
			VotingPower: v.VotingPower,
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
