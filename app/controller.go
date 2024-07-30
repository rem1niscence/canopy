package app

import (
	"bytes"
	"fmt"
	"github.com/ginchuco/ginchu/bft"
	"github.com/ginchuco/ginchu/fsm"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"github.com/ginchuco/ginchu/p2p"
	"sync"
	"sync/atomic"
	"time"
)

var _ bft.Controller = new(Controller)
var _ bft.App = new(Controller)

type Controller struct {
	Consensus  *bft.Consensus
	P2P        *p2p.P2P
	FSM        *fsm.StateMachine
	Mempool    *Mempool
	PublicKey  []byte
	PrivateKey crypto.PrivateKeyI
	resetBFT   chan time.Duration
	syncing    *atomic.Bool
	Config     lib.Config
	log        lib.LoggerI
	sync.Mutex
}

func (c *Controller) LoadValSet(height uint64) (lib.ValidatorSet, lib.ErrorI) {
	return c.FSM.LoadValSet(height)
}

func (c *Controller) LoadCertificate(height uint64) (*lib.QuorumCertificate, lib.ErrorI) {
	return c.FSM.LoadCertificate(height)
}

func (c *Controller) LoadMinimumEvidenceHeight() (uint64, lib.ErrorI) {
	return c.FSM.LoadMinimumEvidenceHeight()
}

func (c *Controller) IsValidDoubleSigner(height uint64, address []byte) bool {
	return c.FSM.IsValidDoubleSigner(height, address)
}

func (c *Controller) LoadLastCommitTime(height uint64) time.Time {
	cert, err := c.FSM.LoadCertificate(height - 1)
	if err != nil {
		c.log.Error(err.Error())
		return time.Time{}
	}
	block, err := c.proposalToBlock(cert.Proposal)
	if err != nil {
		c.log.Error(err.Error())
		return time.Time{}
	}
	if block.BlockHeader == nil {
		return time.Time{}
	}
	return block.BlockHeader.Time.AsTime()
}

func (c *Controller) LoadProposerKeys() *lib.ProposerKeys {
	keys, err := c.FSM.GetProducerKeys()
	if err != nil {
		c.log.Error(err.Error())
		return nil
	}
	return &lib.ProposerKeys{ProposerKeys: keys.ProposerKeys}
}

func (c *Controller) proposalToBlock(proposal []byte) (block *lib.Block, err lib.ErrorI) {
	block = new(lib.Block)
	err = lib.Unmarshal(proposal, block)
	return
}

func (c *Controller) ResetBFTChan() chan time.Duration { return c.resetBFT }
func (c *Controller) Syncing() *atomic.Bool            { return c.syncing }

func New(c lib.Config, valKey crypto.PrivateKeyI, db lib.StoreI, l lib.LoggerI) (*Controller, lib.ErrorI) {
	sm, err := fsm.New(c, db, l)
	if err != nil {
		return nil, err
	}
	height := sm.Height()
	valSet, err := sm.LoadValSet(height)
	if err != nil {
		return nil, err
	}
	lastVS, err := sm.LoadValSet(height - 1)
	if err != nil {
		return nil, err
	}
	maxVals, err := sm.GetMaxValidators()
	if err != nil {
		return nil, err
	}
	mempool, err := NewMempool(sm, c.MempoolConfig, l)
	if err != nil {
		return nil, err
	}
	controller := &Controller{
		P2P:        p2p.New(valKey, maxVals, c, l),
		FSM:        sm,
		Mempool:    mempool,
		PublicKey:  valKey.PublicKey().Bytes(),
		PrivateKey: valKey,
		Mutex:      sync.Mutex{},
		resetBFT:   make(chan time.Duration, 1),
		syncing:    &atomic.Bool{},
		Config:     c,
		log:        l,
	}
	controller.Consensus, err = bft.New(c, valKey, height, valSet, lastVS, controller, controller, l)
	return controller, err
}

func (c *Controller) Start() {
	c.P2P.Start()
	c.StartListeners()
	go c.Sync()
	go c.Consensus.Start()
}

func (c *Controller) Stop() {
	c.Lock()
	defer c.Unlock()
	if err := c.FSM.Store().(lib.StoreI).Close(); err != nil {
		c.log.Error(err.Error())
	}
	c.P2P.Stop()
}

func (c *Controller) GetHeight() uint64 {
	c.Lock()
	defer c.Unlock()
	return c.Consensus.Height
}

func (c *Controller) GetValSet() lib.ValidatorSet {
	c.Lock()
	defer c.Unlock()
	return c.Consensus.ValidatorSet
}

func (c *Controller) ConsensusSummary() ([]byte, lib.ErrorI) {
	c.Lock()
	defer c.Unlock()
	hash := lib.HexBytes{}
	if c.Consensus.Proposal != nil {
		hash = c.HashProposal(c.Consensus.Proposal)
	}
	selfKey, err := crypto.NewPublicKeyFromBytes(c.PublicKey)
	if err != nil {
		return nil, bft.ErrInvalidPublicKey()
	}
	var propAddress lib.HexBytes
	if c.Consensus.ProposerKey != nil {
		propKey, _ := crypto.NewPublicKeyFromBytes(c.Consensus.ProposerKey)
		propAddress = propKey.Address().Bytes()
	}
	status := ""
	switch c.Consensus.View.Phase {
	case bft.Election, bft.Propose, bft.Precommit, bft.Commit:
		proposal := c.Consensus.GetProposal()
		if proposal == nil {
			status = "waiting for proposal"
		} else {
			status = "received proposal"
		}
	default:
		if bytes.Equal(c.Consensus.ProposerKey, c.PublicKey) {
			_, _, votedPercentage := c.Consensus.GetLeadingVote()
			status = fmt.Sprintf("received %d%% of votes", votedPercentage)
		} else {
			status = "voting on proposal"
		}
	}
	return lib.MarshalJSONIndent(ConsensusSummary{
		Syncing:              c.syncing.Load(),
		View:                 c.Consensus.View,
		ProposalHash:         hash,
		Locked:               c.Consensus.Locked,
		Address:              selfKey.Address().Bytes(),
		PublicKey:            c.PublicKey,
		ProposerAddress:      propAddress,
		Proposer:             c.Consensus.ProposerKey,
		Proposals:            c.Consensus.Proposals,
		Votes:                c.Consensus.Votes,
		PartialQCs:           c.Consensus.PartialQCs,
		MinimumPowerFor23Maj: c.Consensus.ValidatorSet.MinimumMaj23,
		PacemakerVotes:       c.Consensus.PacemakerMessages,
		Status:               status,
	})
}

type ConsensusSummary struct {
	Syncing              bool                   `json:"syncing"`
	View                 *lib.View              `json:"view"`
	ProposalHash         lib.HexBytes           `json:"blockHash"`
	Locked               bool                   `json:"locked"`
	Address              lib.HexBytes           `json:"address"`
	PublicKey            lib.HexBytes           `json:"publicKey"`
	ProposerAddress      lib.HexBytes           `json:"proposerAddress"`
	Proposer             lib.HexBytes           `json:"proposer"`
	Proposals            bft.ProposalsForHeight `json:"proposals"`
	PartialQCs           bft.PartialQCs         `json:"partialQCs"`
	PacemakerVotes       bft.PacemakerMessages  `json:"pacemakerVotes"`
	MinimumPowerFor23Maj uint64                 `json:"minimumPowerFor23Maj"`
	Votes                bft.VotesForHeight     `json:"votes"`
	Status               string                 `json:"status"`
}
