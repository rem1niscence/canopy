package plugin

import (
	"github.com/ginchuco/canopy/fsm"
	"github.com/ginchuco/canopy/lib"
	"time"
)

var RegisteredPlugins = make(map[uint64]Plugin)

type (
	Plugin interface {
		WithCallbacks(
			// bftTimerFinishedCallback() tells the controller that the bft timer has completed
			bftTimerFinishedCallback BftTimerFinishedCallback,
			// gossipTxCallback() tells the controller to gossip a new transaction to the p2p network
			gossipTxCallback GossipTxCallback,
		)
		// Start() the plugin service
		Start()
		// Height() get the height of the 3rd party chain
		Height() uint64
		// TotalVDFIterations() optional; loads the total number of unbroken sequential VDF iterations in the chain
		TotalVDFIterations() uint64
		// ResetAndStartBFTTimer() resets and starts the bft trigger timer
		ResetAndStartBFTTimer()
		// ProduceProposal() produce a proposal <block candidate and reward recipients >; ex. reap the mempool and create a block and determine who is paid for the block reward
		ProduceProposal(vdf *lib.VDF) (block []byte, recipients *lib.RewardRecipients, err lib.ErrorI)
		// ValidateCertificate() is this certificate valid according to the 3rd party chain
		ValidateCertificate(committeeHeight uint64, qc *lib.QuorumCertificate) lib.ErrorI
		// IntegratedChain() returns if this chain is using external consensus or integrated consensus
		IntegratedChain() bool
		// HandleTx() integrated only, handle a transaction received from the p2p network; ex. save to mempool of the 3rd party chain
		HandleTx(tx []byte) lib.ErrorI
		// CheckPeerQC() integrated only, basic check on the peer block and determine if the controller is out of sync
		CheckPeerQC(maxHeight uint64, qc *lib.QuorumCertificate) (stillCatchingUp bool, err lib.ErrorI)
		// CommitCertificate() integrated only, attempt to commit the quorum certificate to 3rd party chain's block store by having the chain apply the block
		CommitCertificate(qc *lib.QuorumCertificate) lib.ErrorI
		// LoadCertificate() integrated only, get the quorum certificate from the 3rd party chain's block store to support peer syncing
		LoadCertificate(height uint64) (*lib.QuorumCertificate, lib.ErrorI)
		// LoadLastCommitTime() integrated only, get the last certificate timestamp
		LoadLastCommitTime(height uint64) time.Time
		// LoadMaxBlockSize() integrated only, gets the max block size in bytes (must be below 32MB)
		LoadMaxBlockSize() int
	}

	BftTimerFinishedCallback func(committeeID uint64)
	GossipTxCallback         func(committeeID uint64, tx []byte) lib.ErrorI

	// CanopyPlugin is a unique extension of the Plugin interface that contains Canopy specific functionality
	CanopyPlugin interface {
		Plugin
		// GetFSM() returns the finite state machine implementation of Canopy
		// this is needed by all other plugins to access committee information
		GetFSM() *fsm.StateMachine
		// PendingPageForRPC() returns a page of mempool transactions
		PendingPageForRPC(p lib.PageParams) (page *lib.Page, err lib.ErrorI)
	}
)
