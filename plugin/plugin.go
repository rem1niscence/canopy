package plugin

import (
	"github.com/ginchuco/ginchu/fsm"
	"github.com/ginchuco/ginchu/lib"
	"time"
)

// TODO

// - [ ] Update 'sequence numbers' to use prune safe entropy 'height+entropy'?

// - [ ] See if we want to split the proposal structure into the main 3 pieces
// - [ ] Confirm we want to keep QC and proposal structure separate (I think we may - because the modularity separation between the bft and the proposal interpreter)
// - [ ] Support all types of crypto keys (secp25k1, ed25519, eth)

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
		// ResetAndStartBFTTimer() resets and starts the bft trigger timer
		ResetAndStartBFTTimer()
		// ProduceBlockCandidate() produce a proposal block candidate; ex. reap the mempool and create a block
		ProduceProposal(committeeHeight uint64) (proposal *lib.Proposal, err lib.ErrorI)
		// ValidateBlockCandidate() is this proposal block candidate valid according to the 3rd party chain?
		ValidateProposal(committeeHeight uint64, proposal *lib.Proposal) lib.ErrorI
		// IntegratedChain() returns if this chain is using external consensus or integrated consensus
		IntegratedChain() bool
		// HandleTx() integrated only, handle a transaction received from the p2p network; ex. save to mempool of the 3rd party chain
		HandleTx(tx []byte) lib.ErrorI
		// CheckPeerProposal() integrated only, basic check on the peer proposal and determine if the controller is out of sync
		CheckPeerProposal(maxHeight uint64, prop *lib.Proposal) (outOfSync bool, err lib.ErrorI)
		// CommitCertificate() integrated only, commit the quorum certificate to 3rd party chain's block store
		CommitCertificate(qc *lib.QuorumCertificate) lib.ErrorI
		// LoadCertificate() integrated only, get the quorum certificate from the 3rd party chain's block store to support peer syncing
		LoadCertificate(height uint64) (*lib.QuorumCertificate, lib.ErrorI)
		// LoadLastCommitTime() integrated only, get the last certificate timestamp
		LoadLastCommitTime(height uint64) time.Time
	}

	BftTimerFinishedCallback func(committeeID uint64)
	GossipTxCallback         func(committeeID uint64, tx []byte) lib.ErrorI

	CanopyPlugin interface {
		Plugin
		GetFSM() *fsm.StateMachine
		PendingPageForRPC(p lib.PageParams) (page *lib.Page, err lib.ErrorI)
	}
)
