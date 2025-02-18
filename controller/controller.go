package controller

import (
	"bytes"
	"fmt"
	"github.com/canopy-network/canopy/bft"
	"github.com/canopy-network/canopy/fsm"
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
	"github.com/canopy-network/canopy/p2p"
	"sync"
	"sync/atomic"
	"time"
)

var _ bft.Controller = new(Controller)

// Controller acts as the 'manager' of the modules of the application
type Controller struct {
	FSM           *fsm.StateMachine
	RootChainInfo lib.RootChainInfo
	Mempool       *Mempool
	Consensus     *bft.BFT           // the async consensus process between the committee members for the chain
	isSyncing     *atomic.Bool       // is the chain currently being downloaded from peers
	P2P           *p2p.P2P           // the P2P module the node uses to connect to the network
	Address       []byte             // self address
	PublicKey     []byte             // self public key
	PrivateKey    crypto.PrivateKeyI // self private key
	Config        lib.Config         // node configuration
	log           lib.LoggerI        // object for logging
	sync.Mutex                       // mutex for thread safety
}

// New() creates a new instance of a Controller, this is the entry point when initializing an instance of a Canopy application
func New(fsm *fsm.StateMachine, c lib.Config, valKey crypto.PrivateKeyI, l lib.LoggerI) (*Controller, lib.ErrorI) {
	// make a convenience variable for the 'height' of the state machine
	height := fsm.Height()
	// load maxMembersPerCommittee param to set limits on P2P
	maxMembersPerCommittee, err := fsm.GetMaxValidators()
	if err != nil {
		return nil, err
	}
	mempoolFSM, err := fsm.Copy()
	if err != nil {
		return nil, err
	}
	mempool, err := NewMempool(mempoolFSM, c.MempoolConfig, l)
	if err != nil {
		return nil, err
	}
	// create a controller structure
	controller := &Controller{
		FSM:        fsm,
		Mempool:    mempool,
		Consensus:  nil,
		isSyncing:  &atomic.Bool{},
		P2P:        p2p.New(valKey, maxMembersPerCommittee, c, l),
		Address:    valKey.PublicKey().Address().Bytes(),
		PublicKey:  valKey.PublicKey().Bytes(),
		PrivateKey: valKey,
		Config:     c,
		log:        l,
		Mutex:      sync.Mutex{},
	}
	controller.Consensus, err = bft.New(c, valKey, height, height-1, controller, true, l)
	if err != nil {
		return nil, err
	}
	return controller, err
}

// Start() begins the Controller service
func (c *Controller) Start() {
	// start the P2P module
	c.P2P.Start()
	// start internal Controller listeners for P2P
	c.StartListeners()
	go func() {
		c.log.Warnf("Attempting to connect to the root-Chain")
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for range t.C {
			if c.RootChainInfo.GetHeight() != 0 {
				break
			}
		}
		// start internal Controller listeners for P2P
		c.StartListeners()
		// sync and start each bft module
		go c.Sync()
		go c.Consensus.Start()
	}()
}

// Stop() terminates the Controller service
func (c *Controller) Stop() {
	// lock the controller
	c.Lock()
	defer c.Unlock()
	// stop the store module
	if err := c.FSM.Store().(lib.StoreI).Close(); err != nil {
		c.log.Error(err.Error())
	}
	// stop the p2p module
	c.P2P.Stop()
}

// UpdateRootChainInfo() receives updates from the root-Chain thread
func (c *Controller) UpdateRootChainInfo(info *lib.RootChainInfo) {
	c.Lock()
	defer c.Unlock()
	info.RemoteCallbacks = c.RootChainInfo.RemoteCallbacks
	info.Log = c.log
	// update the root-Chain info
	c.RootChainInfo = *info
	if info.LastValidatorSet.NumValidators == 0 {
		// signal to reset consensus and start a new height
		c.Consensus.ResetBFT <- bft.ResetBFT{IsRootChainUpdate: false}
	} else {
		// signal to reset consensus
		c.Consensus.ResetBFT <- bft.ResetBFT{IsRootChainUpdate: true}
	}
	// update the peer 'must connect'
	c.UpdateP2PMustConnect()
}

// LoadCommittee() gets the ValidatorSet that is authorized to come to Consensus agreement on the Proposal for a specific height/chainId
func (c *Controller) LoadCommittee(height uint64) (lib.ValidatorSet, lib.ErrorI) {
	return c.RootChainInfo.GetValidatorSet(c.Config.ChainId, height)
}

// LoadRootChainOrderBook() gets the order book from the root-Chain
func (c *Controller) LoadRootChainOrderBook(height uint64) (*lib.OrderBook, lib.ErrorI) {
	return c.RootChainInfo.GetOrders(height, c.Config.ChainId)
}

// LoadCertificate() gets the Quorum Block from the chainId-> plugin at a certain height
func (c *Controller) LoadCertificate(height uint64) (*lib.QuorumCertificate, lib.ErrorI) {
	return c.FSM.LoadCertificate(height)
}

// LoadMinimumEvidenceHeight() gets the minimum evidence height from Canopy
func (c *Controller) LoadMinimumEvidenceHeight(height uint64) (uint64, lib.ErrorI) {
	return c.RootChainInfo.GetMinimumEvidenceHeight(height)
}

// GetRootChainLotteryWinner() gets the pseudorandomly selected delegate to reward and their cut
func (c *Controller) GetRootChainLotteryWinner(rootHeight uint64) (winner *lib.LotteryWinner, err lib.ErrorI) {
	return c.RootChainInfo.GetLotteryWinner(rootHeight, c.Config.ChainId)
}

// IsValidDoubleSigner() Canopy checks if the double signer is valid at a certain height
func (c *Controller) IsValidDoubleSigner(height uint64, address []byte) bool {
	isValidDoubleSigner, err := c.RootChainInfo.IsValidDoubleSigner(height, lib.BytesToString(address))
	if err != nil {
		c.log.Errorf("IsValidDoubleSigner failed with error: %s", err.Error())
		return false
	}
	return *isValidDoubleSigner
}

// LoadMaxBlockSize() gets the max block size from the state
func (c *Controller) LoadMaxBlockSize() int {
	params, _ := c.FSM.GetParamsCons()
	if params == nil {
		return 0
	}
	return int(params.BlockSize)
}

// LoadLastCommitTime() gets a timestamp from the most recent Quorum Block
func (c *Controller) LoadLastCommitTime(height uint64) time.Time {
	cert, err := c.FSM.LoadCertificate(height)
	if err != nil {
		c.log.Error(err.Error())
		return time.Time{}
	}
	block := new(lib.Block)
	if err = lib.Unmarshal(cert.Block, block); err != nil {
		c.log.Error(err.Error())
		return time.Time{}
	}
	if block.BlockHeader == nil {
		return time.Time{}
	}
	return time.UnixMicro(int64(block.BlockHeader.Time))
}

// LoadProposerKeys() gets the last Root-ChainId proposer keys
func (c *Controller) LoadLastProposers(height uint64) (*lib.Proposers, lib.ErrorI) {
	// return the last proposers from the Root-ChainId
	return c.RootChainInfo.GetLastProposers(height)
}

// LoadCommitteeData() returns the state metadata for the 'self chain'
func (c *Controller) LoadCommitteeData() (*lib.CommitteeData, lib.ErrorI) {
	data, err := c.FSM.GetCommitteeData(c.Config.ChainId)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Syncing() returns if any of the supported chains are currently syncing
func (c *Controller) Syncing() *atomic.Bool { return c.isSyncing }

// RootChainHeight() returns the height of the canopy root-Chain
func (c *Controller) RootChainHeight() uint64 { return c.RootChainInfo.GetHeight() }

// ChainHeight() returns the height of this target chain
func (c *Controller) ChainHeight() uint64 { return c.FSM.Height() }

// ConsensusSummary() for the RPC - returns the summary json object of the bft for a specific chainID
func (c *Controller) ConsensusSummary() ([]byte, lib.ErrorI) {
	// lock for thread safety
	c.Lock()
	defer c.Unlock()
	// convert self public key from bytes into an object
	selfKey, _ := crypto.NewPublicKeyFromBytes(c.PublicKey)
	// create the consensus summary object
	consensusSummary := &ConsensusSummary{
		Syncing:              c.isSyncing.Load(),
		View:                 c.Consensus.View,
		Locked:               c.Consensus.HighQC != nil,
		Address:              selfKey.Address().Bytes(),
		PublicKey:            c.PublicKey,
		Proposer:             c.Consensus.ProposerKey,
		Proposals:            c.Consensus.Proposals,
		PartialQCs:           c.Consensus.PartialQCs,
		PacemakerVotes:       c.Consensus.PacemakerMessages,
		MinimumPowerFor23Maj: c.Consensus.ValidatorSet.MinimumMaj23,
		Votes:                c.Consensus.Votes,
		Status:               "",
	}
	consensusSummary.BlockHash = c.Consensus.BlockHash
	// if exists, populate the proposal hash
	if c.Consensus.Results != nil {
		consensusSummary.ResultsHash = c.Consensus.Results.Hash()
	}
	if c.Consensus.HighQC != nil {
		consensusSummary.BlockHash = c.Consensus.HighQC.BlockHash
		consensusSummary.ResultsHash = c.Consensus.HighQC.ResultsHash
	}
	// if exists, populate the proposer address
	if c.Consensus.ProposerKey != nil {
		propKey, _ := crypto.NewPublicKeyFromBytes(c.Consensus.ProposerKey)
		consensusSummary.ProposerAddress = propKey.Address().Bytes()
	}
	// create a status string
	switch c.Consensus.View.Phase {
	case bft.Election, bft.Propose, bft.Precommit, bft.Commit:
		proposal := c.Consensus.GetProposal()
		if proposal == nil {
			consensusSummary.Status = "waiting for proposal"
		} else {
			consensusSummary.Status = "received proposal"
		}
	case bft.ElectionVote, bft.ProposeVote, bft.CommitProcess:
		if bytes.Equal(c.Consensus.ProposerKey, c.PublicKey) {
			_, _, votedPercentage := c.Consensus.GetLeadingVote()
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
	BlockHash            lib.HexBytes           `json:"blockHash"`
	ResultsHash          lib.HexBytes           `json:"resultsHash"`
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
