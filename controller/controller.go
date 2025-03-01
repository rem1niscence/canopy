package controller

import (
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

// StartListeners() runs all listeners on separate threads
func (c *Controller) StartListeners() {
	c.log.Debug("Listening for inbound txs, block requests, and consensus messages")
	// listen for syncing peers
	go c.ListenForBlockRequests()
	// listen for inbound consensus messages
	go c.ListenForConsensus()
	// listen for inbound
	go c.ListenForTx()
	// ListenForBlock() is called once syncing finished
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

// IsOwnRoot() returns if this chain is its own root (base)
func (c *Controller) IsOwnRoot() (isOwnRoot bool) {
	// Use the state to check if this chain is the root chain
	isOwnRoot, err := c.FSM.IsOwnRoot()
	if err != nil {
		c.log.Error(err.Error())
	}
	return
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

// emptyInbox() discards all unread messages for a specific topic
func (c *Controller) emptyInbox(topic lib.Topic) {
	// clear the inbox
	for len(c.P2P.Inbox(topic)) > 0 {
		<-c.P2P.Inbox(topic)
	}
}

// convenience aliases that reference the library package
const (
	BlockRequest = lib.Topic_BLOCK_REQUEST
	Block        = lib.Topic_BLOCK
	Tx           = lib.Topic_TX
	Cons         = lib.Topic_CONSENSUS
)
