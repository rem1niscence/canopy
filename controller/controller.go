package controller

import (
	"bytes"
	"fmt"
	"github.com/canopy-network/canopy/bft"
	"github.com/canopy-network/canopy/fsm"
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
	"github.com/canopy-network/canopy/p2p"
	"github.com/canopy-network/canopy/plugin"
	"sync"
	"sync/atomic"
	"time"
)

var _ bft.Controller = new(Controller)

// Controller acts as the 'manager' of the modules of the application
type Controller struct {
	Plugins    map[uint64]plugin.Plugin
	Consensus  map[uint64]*bft.BFT
	P2P        *p2p.P2P
	FSM        *fsm.StateMachine
	PublicKey  []byte
	PrivateKey crypto.PrivateKeyI
	isSyncing  map[uint64]*atomic.Bool
	Config     lib.Config
	log        lib.LoggerI
	sync.Mutex
}

// New() creates a new instance of a Controller, this is the entry point when initializing an instance of a Canopy application
func New(c lib.Config, valKey crypto.PrivateKeyI, l lib.LoggerI) (*Controller, lib.ErrorI) {
	// Canopy acts as the base blockchain and a plugin
	sm := plugin.RegisteredPlugins[lib.CanopyCommitteeId].(plugin.CanopyPlugin).GetFSM()
	height := sm.Height()
	// load maxMembersPerCommittee param to set limits on P2P
	maxMembersPerCommittee, err := sm.GetMaxValidators()
	if err != nil {
		return nil, err
	}
	controller := &Controller{
		Plugins:    plugin.RegisteredPlugins,
		Consensus:  make(map[uint64]*bft.BFT),
		P2P:        p2p.New(valKey, maxMembersPerCommittee, c, l),
		FSM:        sm,
		PublicKey:  valKey.PublicKey().Bytes(),
		PrivateKey: valKey,
		isSyncing:  make(map[uint64]*atomic.Bool),
		Config:     c,
		log:        l,
		Mutex:      sync.Mutex{},
	}
	// using the Canopy state machine as the base blockchain, the controller is able to load the public keys for the 'Committees'
	for id, plug := range controller.Plugins {
		plug.WithCallbacks(controller.ResetBFTCallback, controller.SendTxMsg)
		valSet, e := sm.LoadCommittee(id, height)
		if e != nil {
			return nil, e
		}
		// initialize BFT instances for each plugin
		controller.Consensus[id], err = bft.New(c, valKey, id, height, plug.Height(), valSet, controller, id == lib.CanopyCommitteeId, l)
		controller.isSyncing[id] = &atomic.Bool{}
		controller.Plugins[id] = plug
	}
	return controller, err
}

// Start() begins the Controller service
func (c *Controller) Start() {
	// start the P2P module
	c.P2P.Start()
	// start internal Controller listeners for P2P
	c.StartListeners()
	// sync and start each bft module
	for id, cons := range c.Consensus {
		go c.Sync(id)
		go cons.Start()
	}
}

// Stop() terminates the Controller service
func (c *Controller) Stop() {
	c.Lock()
	defer c.Unlock()
	if err := c.FSM.Store().(lib.StoreI).Close(); err != nil {
		c.log.Error(err.Error())
	}
	c.P2P.Stop()
}

// GetHeight() returns the latest held version of the state for a specific committeeID
func (c *Controller) GetHeight(committeeID uint64) (uint64, lib.ErrorI) {
	c.Lock()
	defer c.Unlock()
	plug, _, err := c.GetPluginAndConsensus(committeeID)
	if err != nil {
		return 0, err
	}
	return plug.Height(), nil
}

// LoadCommittee() gets the ValidatorSet that is authorized to come to Consensus agreement on the Proposal for a specific height/committeeId
func (c *Controller) LoadCommittee(committeeID uint64, height uint64) (lib.ValidatorSet, lib.ErrorI) {
	return c.FSM.LoadCommittee(committeeID, height)
}

// LoadCertificate() gets the Quorum Certificate from the committeeID-> plugin at a certain height
func (c *Controller) LoadCertificate(committeeID uint64, height uint64) (*lib.QuorumCertificate, lib.ErrorI) {
	plug, _, err := c.GetPluginAndConsensus(committeeID)
	if err != nil {
		return nil, err
	}
	return plug.LoadCertificate(height)
}

// LoadMinimumEvidenceHeight() gets the minimum evidence height from Canopy
func (c *Controller) LoadMinimumEvidenceHeight() (uint64, lib.ErrorI) {
	return c.FSM.LoadMinimumEvidenceHeight()
}

// IsValidDoubleSigner() Canopy checks if the double signer is valid at a certain height
func (c *Controller) IsValidDoubleSigner(height uint64, address []byte) bool {
	return c.FSM.IsValidDoubleSigner(height, address) // may only be charged once per height
}

// LoadLastCommitTime() gets a timestamp from the most recent Quorum Certificate
func (c *Controller) LoadLastCommitTime(committeeID, height uint64) time.Time {
	plug, _, err := c.GetPluginAndConsensus(committeeID)
	if err != nil {
		c.log.Error(err.Error())
		return time.Time{}
	}
	return plug.LoadLastCommitTime(height - 1)
}

// LoadProposerKeys() gets the last Canopy proposer keys
func (c *Controller) LoadLastProposers() *lib.Proposers {
	keys, err := c.FSM.GetLastProposers()
	if err != nil {
		c.log.Error(err.Error())
		return nil
	}
	return &lib.Proposers{Addresses: keys.Addresses}
}

// LoadCanopyHeight() loads the latest version of the Canopy blockchain
func (c *Controller) LoadCanopyHeight() uint64 { return c.FSM.Height() }

// LoadCommitteeHeightInState() returns the last height the committee submitted a proposal for rewards
func (c *Controller) LoadCommitteeHeightInState(committeeID uint64) uint64 {
	fund, err := c.FSM.GetCommitteeData(committeeID)
	if err != nil {
		return 0
	}
	if fund == nil {
		return 0
	}
	return fund.CommitteeHeight
}

// Syncing() returns if any of the supported chains are currently syncing
func (c *Controller) Syncing(committeeID uint64) *atomic.Bool { return c.isSyncing[committeeID] }

// ConsensusSummary() returns the summary json object of the bft for a specific chainID
func (c *Controller) ConsensusSummary(committeeID uint64) ([]byte, lib.ErrorI) {
	c.Lock()
	defer c.Unlock()
	hash := lib.HexBytes{}
	_, cons, e := c.GetPluginAndConsensus(committeeID)
	if e != nil {
		return nil, e
	}
	if cons.Results != nil {
		hash = cons.Results.Hash()
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
		Syncing:              c.isSyncing[committeeID].Load(),
		View:                 cons.View,
		ProposalHash:         hash,
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

// ConsensusSummary is simply a json informational structure about the local status of the BFT
type ConsensusSummary struct {
	Syncing              bool                   `json:"isSyncing"`
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
