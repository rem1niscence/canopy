package bft

import (
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"sync"
	"sync/atomic"
	"time"
)

var _ Controller = &testController{}
var _ App = &testController{}

type testController struct {
	sync.Mutex
	minEvidenceHeight uint64
	proposerKeys      *lib.ProposerKeys
	lastCandidateTime time.Time
	certificates      map[uint64]*lib.QuorumCertificate
	valSet            map[uint64]ValSet
	resetBFTChan      chan time.Duration
	syncingDoneChan   chan struct{}
	syncing           *atomic.Bool

	gossipCertChan     chan *lib.QuorumCertificate
	sendToProposerChan chan lib.Signable
	sendToReplicasChan chan lib.Signable
}

const expectedCandidateLen = crypto.HashSize

func (t *testController) ProduceProposal(_ *ByzantineEvidence) (candidate []byte, err lib.ErrorI) {
	candidate = crypto.Hash([]byte("mock"))
	return
}

func (t *testController) CheckProposal(candidate []byte) lib.ErrorI {
	if len(candidate) != 0 {
		return nil
	}
	return ErrEmptyMessage()
}

func (t *testController) ValidateProposal(candidate []byte, _ *ByzantineEvidence) lib.ErrorI {
	if len(candidate) == expectedCandidateLen {
		return nil
	}
	return ErrEmptyMessage()
}

func (t *testController) LoadMinimumEvidenceHeight() (uint64, lib.ErrorI) {
	return t.minEvidenceHeight, nil
}

func (t *testController) LoadCertificate(h uint64) (*lib.QuorumCertificate, lib.ErrorI) {
	return t.certificates[h], nil
}
func (t *testController) LoadProposerKeys() *lib.ProposerKeys {
	return t.proposerKeys
}
func (t *testController) IsValidDoubleSigner(_ uint64, _ []byte) bool        { return true }
func (t *testController) HashProposal(bytes []byte) []byte                   { return crypto.Hash(bytes) }
func (t *testController) LoadLastCommitTime(_ uint64) time.Time              { return t.lastCandidateTime }
func (t *testController) LoadValSet(h uint64) (lib.ValidatorSet, lib.ErrorI) { return t.valSet[h], nil }
func (t *testController) ResetBFTChan() chan time.Duration                   { return t.resetBFTChan }
func (t *testController) SyncingDoneChan() chan struct{}                     { return t.syncingDoneChan }
func (t *testController) Syncing() *atomic.Bool                              { return t.syncing }
func (t *testController) GossipCertificate(cert *lib.QuorumCertificate)      { t.gossipCertChan <- cert }
func (t *testController) SendToReplicas(msg lib.Signable)                    { t.sendToReplicasChan <- msg }
func (t *testController) SendToProposer(msg lib.Signable)                    { t.sendToProposerChan <- msg }
