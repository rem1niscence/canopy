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
	Chains     map[uint64]*Chain  // list of 'chains' this node is running
	P2P        *p2p.P2P           // the P2P module the node uses to connect to the network
	PublicKey  []byte             // self public key
	PrivateKey crypto.PrivateKeyI // self private key
	Config     lib.Config         // node configuration
	log        lib.LoggerI        // object for logging
	sync.Mutex                    // mutex for thread safety
}

// Chain represents a 3rd party L1 'blockchain' that is a client of Canopy
type Chain struct {
	Plugin    plugin.Plugin // the bridge to the 3rd party chain
	Consensus *bft.BFT      // the async consensus process between the committee members for the chain
	isSyncing *atomic.Bool  // is the chain currently being downloaded from peers
}

// New() creates a new instance of a Controller, this is the entry point when initializing an instance of a Canopy application
func New(c lib.Config, valKey crypto.PrivateKeyI, l lib.LoggerI) (*Controller, lib.ErrorI) {
	// Canopy acts as the base blockchain and a plugin
	sm := plugin.RegisteredPlugins[lib.CanopyCommitteeId].(plugin.CanopyPlugin).GetFSM()
	// make a convenience variable for the 'height' of the state machine
	height := sm.Height()
	// load maxMembersPerCommittee param to set limits on P2P
	maxMembersPerCommittee, err := sm.GetMaxValidators()
	if err != nil {
		return nil, err
	}
	// create a controller structure
	controller := &Controller{
		Chains:     make(map[uint64]*Chain),
		P2P:        p2p.New(valKey, maxMembersPerCommittee, c, l),
		PublicKey:  valKey.PublicKey().Bytes(),
		PrivateKey: valKey,
		Config:     c,
		log:        l,
		Mutex:      sync.Mutex{},
	}
	// using the Canopy state machine as the base blockchain, the controller is able to load the public keys for the 'Committees'
	for id, plug := range plugin.RegisteredPlugins {
		// add the callbacks from the controller to the plugin
		// these callbacks allow the plugin to 'trigger' things in the controller
		plug.WithCallbacks(controller.ResetBFTCallback, controller.SendTxMsg)
		// load the committee from the Canopy state
		valSet, e := sm.LoadCommittee(id, height)
		if e != nil {
			return nil, e // TODO is there a chicken and egg problem here with starting a new committee?
		}
		// initialize the chain, setting the plugin
		chain := &Chain{Plugin: plug, isSyncing: &atomic.Bool{}}
		// save this chain in the map under the 'id'
		controller.Chains[id] = chain
		// create a new BFT instance and assign it to the chain
		chain.Consensus, err = bft.New(c, valKey, id, height, plug.Height()-1, valSet, controller, id == lib.CanopyCommitteeId, l)
		if err != nil {
			return nil, err
		}
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
	for id, chain := range c.Chains {
		go c.Sync(id)
		go chain.Consensus.Start()
	}
}

// Stop() terminates the Controller service
func (c *Controller) Stop() {
	// lock the controller
	c.Lock()
	defer c.Unlock()
	// stop the store module
	if err := c.CanopyFSM().Store().(lib.StoreI).Close(); err != nil {
		c.log.Error(err.Error())
	}
	// stop the p2p module
	c.P2P.Stop()
}

// LoadCommittee() gets the ValidatorSet that is authorized to come to Consensus agreement on the Proposal for a specific height/committeeId
func (c *Controller) LoadCommittee(committeeID uint64, height uint64) (lib.ValidatorSet, lib.ErrorI) {
	return c.CanopyFSM().LoadCommittee(committeeID, height)
}

// LoadCertificate() gets the Quorum Block from the committeeID-> plugin at a certain height
func (c *Controller) LoadCertificate(committeeID uint64, height uint64) (*lib.QuorumCertificate, lib.ErrorI) {
	chain, err := c.GetChain(committeeID)
	if err != nil {
		return nil, err
	}
	return chain.Plugin.LoadCertificate(height)
}

// LoadMinimumEvidenceHeight() gets the minimum evidence height from Canopy
func (c *Controller) LoadMinimumEvidenceHeight() (uint64, lib.ErrorI) {
	return c.CanopyFSM().LoadMinimumEvidenceHeight()
}

// IsValidDoubleSigner() Canopy checks if the double signer is valid at a certain height
func (c *Controller) IsValidDoubleSigner(height uint64, address []byte) bool {
	return c.CanopyFSM().IsValidDoubleSigner(height, address) // may only be charged once per height
}

// LoadLastCommitTime() gets a timestamp from the most recent Quorum Block
func (c *Controller) LoadLastCommitTime(committeeID, height uint64) time.Time {
	// get the chain from the controller map
	chain, err := c.GetChain(committeeID)
	if err != nil {
		c.log.Error(err.Error())
		return time.Time{}
	}
	// return the last commit time for the previous height from the plugin
	return chain.Plugin.LoadLastCommitTime(height - 1)
}

// LoadProposerKeys() gets the last Canopy proposer keys
func (c *Controller) LoadLastProposers() *lib.Proposers {
	// load the last proposers from Canopy
	keys, err := c.CanopyFSM().GetLastProposers()
	if err != nil {
		c.log.Error(err.Error())
		return nil
	}
	// return the
	return &lib.Proposers{Addresses: keys.Addresses}
}

// LoadCommitteeHeightInState() returns the last height the committee submitted a proposal for rewards
func (c *Controller) LoadCommitteeHeightInState(committeeID uint64) uint64 {
	// load the committee data from the canopy fsm
	data, err := c.CanopyFSM().GetCommitteeData(committeeID)
	// if the data doesn't exist return 0
	if data == nil || err != nil {
		return 0
	}
	// return the committee height
	return data.LastCanopyHeightUpdated
}

// CanopyFSM() returns the state machine of the Canopy blockchain
func (c *Controller) CanopyFSM() *fsm.StateMachine {
	return c.Chains[lib.CanopyCommitteeId].Plugin.(plugin.CanopyPlugin).GetFSM()
}

// Syncing() returns if any of the supported chains are currently syncing
func (c *Controller) Syncing(committeeID uint64) *atomic.Bool { return c.Chains[committeeID].isSyncing }

// GetCanopyHeight() returns the height of the canopy base-chain
func (c *Controller) GetCanopyHeight() uint64 { return c.CanopyFSM().Height() }

// ConsensusSummary() for the RPC - returns the summary json object of the bft for a specific chainID
func (c *Controller) ConsensusSummary(committeeID uint64) ([]byte, lib.ErrorI) {
	// lock for thread safety
	c.Lock()
	defer c.Unlock()
	// get the chain to summarize
	chain, e := c.GetChain(committeeID)
	if e != nil {
		return nil, e
	}
	// convert self public key from bytes into an object
	selfKey, _ := crypto.NewPublicKeyFromBytes(c.PublicKey)
	// create the consensus summary object
	consensusSummary := &ConsensusSummary{
		Syncing:              chain.isSyncing.Load(),
		View:                 chain.Consensus.View,
		Locked:               chain.Consensus.HighQC != nil,
		Address:              selfKey.Address().Bytes(),
		PublicKey:            c.PublicKey,
		Proposer:             chain.Consensus.ProposerKey,
		Proposals:            chain.Consensus.Proposals,
		Votes:                chain.Consensus.Votes,
		PartialQCs:           chain.Consensus.PartialQCs,
		MinimumPowerFor23Maj: chain.Consensus.ValidatorSet.MinimumMaj23,
		PacemakerVotes:       chain.Consensus.PacemakerMessages,
	}
	// if exists, populate the proposal hash
	if chain.Consensus.Results != nil {
		consensusSummary.ProposalHash = chain.Consensus.Results.Hash()
	}
	// if exists, populate the proposer address
	if chain.Consensus.ProposerKey != nil {
		propKey, _ := crypto.NewPublicKeyFromBytes(chain.Consensus.ProposerKey)
		consensusSummary.ProposerAddress = propKey.Address().Bytes()
	}
	// create a status string
	switch chain.Consensus.View.Phase {
	case bft.Election, bft.Propose, bft.Precommit, bft.Commit:
		proposal := chain.Consensus.GetProposal()
		if proposal == nil {
			consensusSummary.Status = "waiting for proposal"
		} else {
			consensusSummary.Status = "received proposal"
		}
	case bft.ElectionVote, bft.ProposeVote, bft.CommitProcess:
		if bytes.Equal(chain.Consensus.ProposerKey, c.PublicKey) {
			_, _, votedPercentage := chain.Consensus.GetLeadingVote()
			consensusSummary.Status = fmt.Sprintf("received %d%% of votes", votedPercentage)
		} else {
			consensusSummary.Status = "voting on proposal"
		}
	}
	// convert the object into json
	return lib.MarshalJSONIndent(&consensusSummary)
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
