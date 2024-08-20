package controller

import (
	"bytes"
	"fmt"
	"github.com/ginchuco/ginchu/bft"
	"github.com/ginchuco/ginchu/fsm"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"github.com/ginchuco/ginchu/p2p"
	"github.com/ginchuco/ginchu/plugin"
	"sync"
	"sync/atomic"
	"time"
)

var _ bft.Controller = new(Controller)

type Controller struct {
	Plugins    map[uint64]plugin.Plugin
	Consensus  map[uint64]*bft.Consensus
	P2P        *p2p.P2P
	FSM        *fsm.StateMachine
	PublicKey  []byte
	PrivateKey crypto.PrivateKeyI
	syncing    *atomic.Bool
	Config     lib.Config
	log        lib.LoggerI
	sync.Mutex
}

func (c *Controller) LoadCommittee(committeeID uint64, height uint64) (lib.ValidatorSet, lib.ErrorI) {
	return c.FSM.LoadCommittee(committeeID, height)
}

func (c *Controller) LoadCertificate(committeeID uint64, height uint64) (*lib.QuorumCertificate, lib.ErrorI) {
	plug, _, err := c.GetPluginAndConsensus(committeeID)
	if err != nil {
		return nil, err
	}
	return plug.LoadCertificate(height)
}

func (c *Controller) LoadMinimumEvidenceHeight() (uint64, lib.ErrorI) {
	return c.FSM.LoadMinimumEvidenceHeight()
}

func (c *Controller) IsValidDoubleSigner(height uint64, address []byte) bool {
	return c.FSM.IsValidDoubleSigner(height, lib.CANOPY_COMMITTEE_ID, address) // may only be charged once per height per chain
}

func (c *Controller) LoadLastCommitTime(committeeID, height uint64) time.Time {
	plug, _, err := c.GetPluginAndConsensus(committeeID)
	if err != nil {
		c.log.Error(err.Error())
		return time.Time{}
	}
	return plug.LoadLastCommitTime(height - 1)
}

func (c *Controller) LoadProposerKeys() *lib.ProposerKeys {
	keys, err := c.FSM.GetProducerKeys()
	if err != nil {
		c.log.Error(err.Error())
		return nil
	}
	return &lib.ProposerKeys{ProposerKeys: keys.ProposerKeys}
}

func (c *Controller) Syncing() *atomic.Bool { return c.syncing }

func New(c lib.Config, valKey crypto.PrivateKeyI, l lib.LoggerI) (*Controller, lib.ErrorI) {
	sm := plugin.RegisteredPlugins[lib.CANOPY_COMMITTEE_ID].(plugin.CanopyPlugin).GetFSM()
	height := sm.Height()
	maxMembersPerCommittee, err := sm.GetMaxValidators()
	if err != nil {
		return nil, err
	}
	controller := &Controller{
		Plugins:    plugin.RegisteredPlugins,
		Consensus:  make(map[uint64]*bft.Consensus),
		P2P:        p2p.New(valKey, maxMembersPerCommittee, c, l),
		FSM:        sm,
		PublicKey:  valKey.PublicKey().Bytes(),
		PrivateKey: valKey,
		syncing:    &atomic.Bool{},
		Config:     c,
		log:        l,
		Mutex:      sync.Mutex{},
	}
	for id := range controller.Plugins {
		valSet, e := sm.LoadCommittee(id, height)
		if e != nil {
			return nil, e
		}
		lastVS, e := sm.LoadCommittee(id, height-1)
		if e != nil {
			return nil, e
		}
		controller.Consensus[id], err = bft.New(c, valKey, id, height, valSet, lastVS, controller, l)
	}
	return controller, err
}

func (c *Controller) Start() {
	c.P2P.Start()
	c.StartListeners()
	for id, cons := range c.Consensus {
		go c.Sync(id)
		go cons.Start()
	}
}

func (c *Controller) Stop() {
	c.Lock()
	defer c.Unlock()
	if err := c.FSM.Store().(lib.StoreI).Close(); err != nil {
		c.log.Error(err.Error())
	}
	c.P2P.Stop()
}

func (c *Controller) GetHeight(committeeID uint64) (uint64, lib.ErrorI) {
	c.Lock()
	defer c.Unlock()
	_, cons, err := c.GetPluginAndConsensus(committeeID)
	if err != nil {
		return 0, err
	}
	return cons.Height, nil
}

func (c *Controller) ConsensusSummary(committeeID uint64) ([]byte, lib.ErrorI) {
	c.Lock()
	defer c.Unlock()
	hash := lib.HexBytes{}
	_, cons, e := c.GetPluginAndConsensus(committeeID)
	if e != nil {
		return nil, e
	}
	if cons.Proposal != nil {
		hash = cons.Proposal.Hash()
	}
	selfKey, err := crypto.NewPublicKeyFromBytes(c.PublicKey)
	if err != nil {
		return nil, bft.ErrInvalidPublicKey()
	}
	var propAddress lib.HexBytes
	if cons.ProposerKey != nil {
		propKey, _ := crypto.NewPublicKeyFromBytes(cons.ProposerKey)
		propAddress = propKey.Address().Bytes()
	}
	status := ""
	switch cons.View.Phase {
	case bft.Election, bft.Propose, bft.Precommit, bft.Commit:
		proposal := cons.GetProposal()
		if proposal == nil {
			status = "waiting for proposal"
		} else {
			status = "received proposal"
		}
	default:
		if bytes.Equal(cons.ProposerKey, c.PublicKey) {
			_, _, votedPercentage := cons.GetLeadingVote()
			status = fmt.Sprintf("received %d%% of votes", votedPercentage)
		} else {
			status = "voting on proposal"
		}
	}
	return lib.MarshalJSONIndent(ConsensusSummary{
		Syncing:              c.syncing.Load(),
		View:                 cons.View,
		ProposalHash:         hash,
		Locked:               cons.Locked,
		Address:              selfKey.Address().Bytes(),
		PublicKey:            c.PublicKey,
		ProposerAddress:      propAddress,
		Proposer:             cons.ProposerKey,
		Proposals:            cons.Proposals,
		Votes:                cons.Votes,
		PartialQCs:           cons.PartialQCs,
		MinimumPowerFor23Maj: cons.ValidatorSet.MinimumMaj23,
		PacemakerVotes:       cons.PacemakerMessages,
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
