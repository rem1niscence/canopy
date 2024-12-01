package controller

import (
	"bytes"
	"fmt"
	"github.com/canopy-network/canopy/bft"
	"github.com/canopy-network/canopy/fsm/types"
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/p2p"
	"math/rand"
	"time"
)

// Sync() attempts to sync the blockchain for a specific CommitteeID
// 1) Get the height and begin block params from the state_machine
// 2) Get peer max_height from P2P
// 3) Ask peers for a block at a time
// 4) CheckBasic each block by checking if cert was signed by +2/3 maj (COMMIT message)
// 5) Commit block against the state machine, if error fatal
// 5) Do this until reach the max-peer-height
// 6) Stay on top by listening to incoming cert messages
func (c *Controller) Sync(committeeID uint64) {
	c.log.Infof("Sync started 🔄 for committee %d", committeeID)
	// set isSyncing
	c.Chains[committeeID].isSyncing.Store(true)
	// initialize tracking variables
	reqRecipient, maxHeight, minVDFIterations, syncingPeers := make([]byte, 0), uint64(0), uint64(0), make([]string, 0)
	// initialize a callback for `pollMaxHeight`
	pollMaxHeight := func() { maxHeight, minVDFIterations, syncingPeers = c.pollMaxHeight(committeeID, 1) }
	// check if node is alone in the committee
	if c.singleNodeNetwork(committeeID) {
		c.finishSyncing(committeeID)
		return
	}
	// poll max height of all peers
	pollMaxHeight()
	for {
		if c.syncingDone(committeeID, maxHeight, minVDFIterations) {
			c.finishSyncing(committeeID)
			return
		}
		// get a random peer to send to
		reqRecipient, _ = lib.StringToBytes(syncingPeers[rand.Intn(len(syncingPeers))])
		// send the request
		c.RequestBlock(committeeID, false, reqRecipient)
		select {
		case msg := <-c.P2P.Inbox(Block): // got a response
			// if the responder does not equal the requester
			responder := msg.Sender.Address.PublicKey
			if !bytes.Equal(responder, reqRecipient) {
				c.P2P.ChangeReputation(responder, p2p.UnexpectedBlockRep)
				// poll max height of all peers
				pollMaxHeight()
				continue
			}
			blkResponseMsg, ok := msg.Message.(*lib.BlockMessage)
			if !ok {
				c.P2P.ChangeReputation(msg.Sender.Address.PublicKey, p2p.InvalidBlockRep)
				// poll max height of all peers
				pollMaxHeight()
				return
			}
			// process the quorum certificate received from the peer
			if _, _, err := c.handlePeerBlock(msg.Sender.Address.PublicKey, blkResponseMsg); err != nil {
				// poll max height of all peers
				pollMaxHeight()
				continue
			}
			// each peer is individually polled with each request as well
			// if that poll max height has grown, we accept that as
			// the new max height
			if blkResponseMsg.MaxHeight > maxHeight && blkResponseMsg.TotalVdfIterations >= minVDFIterations {
				maxHeight, minVDFIterations = blkResponseMsg.MaxHeight, blkResponseMsg.TotalVdfIterations
			}
			c.P2P.ChangeReputation(responder, p2p.GoodBlockRep)
		case <-time.After(p2p.SyncTimeoutS * time.Second): // timeout
			c.P2P.ChangeReputation(reqRecipient, p2p.TimeoutRep)
			// poll max height of all peers
			pollMaxHeight()
			continue
		}
	}
}

// PUBLISHERS BELOW

// SendTxMsg() gossips a Transaction through the P2P network for a specific committeeID
func (c *Controller) SendTxMsg(committeeID uint64, tx []byte) lib.ErrorI {
	// create a transaction message object using the tx bytes and the committee id
	msg := &lib.TxMessage{CommitteeId: committeeID, Tx: tx}
	// send it to self for de-duplication and awareness of self originated transactions
	if err := c.P2P.SelfSend(c.PublicKey, Tx, msg); err != nil {
		return err
	}
	// gossip to all the peers for the chain
	return c.P2P.SendToChainPeers(committeeID, Tx, msg)
}

// SendCertificateResultsTx() originates and auto-sends a CertificateResultsTx after successfully leading a Consensus height
func (c *Controller) SendCertificateResultsTx(committeeID uint64, qc *lib.QuorumCertificate) {
	c.log.Debugf("Sending certificate results txn for: %s", lib.BytesToString(qc.ResultsHash))
	tx, err := types.NewCertificateResultsTx(c.PrivateKey, qc, 0)
	if err != nil {
		c.log.Errorf("Creating auto-certificate-results-txn failed with err: %s", err.Error())
		return
	}
	// get the pool (of funds for the committee) from the canopy blockchain
	pool, err := c.CanopyFSM().GetPool(qc.Header.CommitteeId)
	if err != nil || pool.Amount == 0 {
		c.log.Errorf("Auto-proposal-txn stopped due to not subsidized")
		return // not subsidized
	}
	// convert the transaction into bytes
	bz, err := lib.Marshal(tx)
	if err != nil {
		c.log.Errorf("Marshalling auto-proposal-txn failed with err: %s", err.Error())
		return
	}
	// send the proposal transaction
	if err = c.SendTxMsg(committeeID, bz); err != nil {
		c.log.Errorf("Gossiping auto-proposal-txn failed with err: %s", err.Error())
		return
	}
}

// GossipBlockMsg() gossips a QuorumCertificate (with block) through the P2P network for a specific committeeID
func (c *Controller) GossipBlock(committeeID uint64, qc *lib.QuorumCertificate) {
	// create a new pointer instance so when the results are omitted it doesn't affect other parts
	cpy := &(*qc)
	// get the chain associated with this quorum certificate
	chain, err := c.GetChain(committeeID)
	if err != nil {
		c.log.Errorf("unable to gossip block with err: %s", err.Error())
		return
	}
	// when sending a certificate message, it's good practice to omit the 'results' field as it is only important for the Canopy Blockchain
	cpy.Results = nil
	c.log.Debugf("Gossiping certificate: %s", lib.BytesToString(cpy.ResultsHash))
	// create the block message
	blockMessage := &lib.BlockMessage{
		CommitteeId:         committeeID,
		MaxHeight:           chain.Plugin.Height(),
		TotalVdfIterations:  chain.Plugin.TotalVDFIterations(),
		BlockAndCertificate: cpy,
	}
	// gossip the block message to peers for a particular committee id
	if err = c.P2P.SendToChainPeers(committeeID, Block, blockMessage); err != nil {
		c.log.Errorf("unable to gossip block with err: %s", err.Error())
	}
	// if a single node network - send to self
	if c.singleNodeNetwork(committeeID) {
		if err = c.P2P.SelfSend(c.PublicKey, Block, blockMessage); err != nil {
			c.log.Errorf("unable to self send block with err: %s", err.Error())
		}
	}
	c.log.Debugf("gossiping done")
}

// RequestBlock() sends a QuorumCertificate (block + certificate) request to peer(s) - `heightOnly` is a request for just the peer's max height
func (c *Controller) RequestBlock(committeeID uint64, heightOnly bool, recipients ...[]byte) {
	// get the chain associated with this quorum certificate
	chain, err := c.GetChain(committeeID)
	if err != nil {
		c.log.Errorf("unable to gossip block with err: %s", err.Error())
		return
	}
	// if the optional 'recipients' is specified
	if len(recipients) != 0 {
		// for each 'recipient'
		for _, pk := range recipients {
			// send it to exactly who was specified in the function call
			if err = c.P2P.SendTo(pk, BlockRequest, &lib.BlockRequestMessage{
				CommitteeId: committeeID,
				Height:      chain.Plugin.Height(),
				HeightOnly:  heightOnly,
			}); err != nil {
				c.log.Error(err.Error())
			}
		}
	} else {
		// send it to the chain peers
		if err = c.P2P.SendToChainPeers(committeeID, BlockRequest, &lib.BlockRequestMessage{
			CommitteeId: committeeID,
			Height:      chain.Plugin.Height(),
			HeightOnly:  heightOnly,
		}, true); err != nil {
			c.log.Error(err.Error())
		}
	}
}

// SendBlock() responds to a `blockRequest` message to a peer - always sending the self.MaxHeight and sometimes sending the actual block and supporting QC
func (c *Controller) SendBlock(committeeID, maxHeight, vdfIterations uint64, blockAndCert *lib.QuorumCertificate, recipient []byte) {
	// send the block to the recipient public key specified
	if err := c.P2P.SendTo(recipient, Block, &lib.BlockMessage{
		CommitteeId:         committeeID,
		MaxHeight:           maxHeight,
		TotalVdfIterations:  vdfIterations,
		BlockAndCertificate: blockAndCert,
	}); err != nil {
		c.log.Error(err.Error())
	}
}

// SendToReplicas() sends a bft message to a specific ValidatorSet (the Committee)
func (c *Controller) SendToReplicas(committeeID uint64, replicas lib.ValidatorSet, msg lib.Signable) {
	// handle the signable message
	message, err := c.signableToConsensusMessage(committeeID, msg)
	if err != nil {
		c.log.Error(err.Error())
		return
	}
	// for each replica
	for _, replica := range replicas.ValidatorSet.ValidatorSet {
		// check if replica is self
		if bytes.Equal(replica.PublicKey, c.PublicKey) {
			// send to self
			if err = c.P2P.SelfSend(c.PublicKey, Cons, message); err != nil {
				c.log.Error(err.Error())
			}
		} else {
			// send to peer
			if err = c.P2P.SendTo(replica.PublicKey, Cons, message); err != nil {
				c.log.Warn(err.Error())
			}
		}
	}
}

// SendToProposer() sends a bft message to the leader of the Consensus round
func (c *Controller) SendToProposer(committeeID uint64, msg lib.Signable) {
	// get the chain associated with the message
	chain, err := c.GetChain(committeeID)
	if err != nil {
		c.log.Error(err.Error())
		return
	}
	// handle the signable message
	message, err := c.signableToConsensusMessage(committeeID, msg)
	if err != nil {
		c.log.Error(err.Error())
		return
	}
	// check if sending to self or peer
	if chain.Consensus.SelfIsProposer() {
		// handle self send
		if err = c.P2P.SelfSend(c.PublicKey, Cons, message); err != nil {
			c.log.Error(err.Error())
		}
	} else {
		// handle peer send
		if err = c.P2P.SendTo(chain.Consensus.ProposerKey, Cons, message); err != nil {
			c.log.Error(err.Error())
			return
		}
	}
}

// LISTENERS BELOW

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

// ListenForBlock() listens for inbound block messages, internally routes them, and gossips them to peers
func (c *Controller) ListenForBlock() {
	c.log.Debug("Listening for inbound blocks")
	// initialize a cache that prevents duplicate messages
	cache := lib.NewMessageCache()
	// wait and execute for each inbound message received
	for msg := range c.P2P.Inbox(Block) {
		// create a variable to signal a 'stop loop'
		var quit bool
		// wrap in a function call to use 'defer' functionality
		func() {
			// lock the controller to prevent multi-thread conflicts
			c.Lock()
			defer c.Unlock()
			// check and add the message to the cache to prevent duplicates
			if ok := cache.Add(msg); !ok {
				return
			}
			c.log.Infof("Received new block from %s 📥", lib.BytesToTruncatedString(msg.Sender.Address.PublicKey))
			// try to cast the message to a block message
			blockMessage, ok := msg.Message.(*lib.BlockMessage)
			// if not a block message, slash the peer
			if !ok {
				c.P2P.ChangeReputation(msg.Sender.Address.PublicKey, p2p.InvalidBlockRep)
				return
			}
			// track processing time for consensus module
			startTime := time.Now()
			// handle the peer block
			qc, outOfSync, err := c.handlePeerBlock(msg.Sender.Address.PublicKey, blockMessage)
			// if the node has fallen 'out of sync' with the chain
			if outOfSync {
				c.log.Warnf("Node fell out of sync for committeeID: %d", blockMessage.CommitteeId)
				// revert to syncing mode
				go c.Sync(blockMessage.CommitteeId)
				// signal exit the out loop
				quit = true
				return
			}
			if err != nil {
				c.log.Debugf("peer block was invalid: %s", err.Error())
				c.P2P.ChangeReputation(msg.Sender.Address.PublicKey, p2p.InvalidBlockRep)
				return
			}
			// gossip the block to the node's peers
			c.GossipBlock(blockMessage.CommitteeId, qc)
			// check if the block is for the Canopy (special case)
			if blockMessage.CommitteeId == lib.CanopyCommitteeId {
				// a new canopy block means all BFTs are reset with updated committees which prevents conflicting validator sets among peers
				newCanopyHeight := c.CanopyFSM().Height()
				// for each chain (that isn't canopy)
				for _, chain := range c.Chains {
					if chain.Consensus.CommitteeId == lib.CanopyCommitteeId {
						// reset the canopy BFT as if it was a target chain
						c.Chains[blockMessage.CommitteeId].Consensus.ResetBFTChan() <- bft.ResetBFT{ProcessTime: time.Since(startTime)}
						continue
					}
					// load the new committee
					newCommittee, e := c.LoadCommittee(chain.Consensus.CommitteeId, newCanopyHeight)
					if e != nil {
						c.log.Error(e.Error())
						continue
					}
					// reset & update the consensus module
					chain.Consensus.ResetBFTChan() <- bft.ResetBFT{UpdatedCommitteeSet: newCommittee, UpdatedCanopyHeight: newCanopyHeight}
				}
				// update the peer 'must connect'
				c.UpdateP2PMustConnect()
			} else {
				// block for a different committee than Canopy (standard case)
				c.Chains[blockMessage.CommitteeId].Consensus.ResetBFTChan() <- bft.ResetBFT{ProcessTime: time.Since(startTime)}
			}
		}()
		// if quit signaled, exit the loop
		if quit {
			return
		}
	}
}

// ListenForConsensus() listens and internally routes inbound consensus messages
func (c *Controller) ListenForConsensus() {
	// wait and execute for each consensus message received
	for msg := range c.P2P.Inbox(Cons) {
		// define an error handling function for convenience
		handleErr := func(e error, delta int32) {
			c.log.Error(e.Error())
			c.P2P.ChangeReputation(msg.Sender.Address.PublicKey, delta)
		}
		func() {
			// lock the controller for thread safety
			c.Lock()
			defer c.Unlock()
			// try to cast the p2p message to a 'consensus message'
			consMsg, ok := msg.Message.(*lib.ConsensusMessage)
			// if cast unsuccessful
			if !ok {
				c.P2P.ChangeReputation(msg.Sender.Address.PublicKey, p2p.InvalidMsgRep)
				return
			}
			// get the chain in question
			chain, ok := c.Chains[consMsg.CommitteeId]
			if !ok {
				return
			}
			// if the chain is syncing, return
			if chain.isSyncing.Load() {
				return
			}
			// load the committee associated with the committee id at the latest canopy height
			vs, err := c.LoadCommittee(consMsg.CommitteeId, c.CanopyFSM().Height())
			if err != nil {
				handleErr(err, 0)
				return
			}
			// ensure the sender is a validator
			if _, err = vs.GetValidator(msg.Sender.Address.PublicKey); err != nil {
				handleErr(err, p2p.NotValRep)
				return
			}
			// convert the bytes to a bft.Message object
			bftMsg := new(bft.Message)
			if err = lib.Unmarshal(consMsg.Message, bftMsg); err != nil {
				handleErr(err, p2p.InvalidMsgRep)
				return
			}
			// route the message to the consensus module
			if err = chain.Consensus.HandleMessage(bftMsg); err != nil {
				handleErr(err, p2p.InvalidMsgRep)
				return
			}
		}()
	}
}

// ListenForTx() listen for inbound tx messages, internally route them, and gossip them to peers
func (c *Controller) ListenForTx() {
	// create a new message cache to filter out duplicate transaction messages
	cache := lib.NewMessageCache()
	// wait and execute for each inbound transaction message
	for msg := range c.P2P.Inbox(Tx) {
		func() {
			// lock the controller for thread safety
			c.Lock()
			defer c.Unlock()
			// check and add the message to the cache to prevent duplicates
			if ok := cache.Add(msg); !ok {
				return
			}
			// create a convenience variable for the identity of the sender
			senderID := msg.Sender.Address.PublicKey
			// try to cast the p2p message as a tx message
			txMsg, ok := msg.Message.(*lib.TxMessage)
			// if the cast failed
			if !ok {
				c.P2P.ChangeReputation(senderID, p2p.InvalidMsgRep)
				return
			}
			// if the message is empty
			if txMsg == nil {
				c.P2P.ChangeReputation(senderID, p2p.InvalidMsgRep)
				return
			}
			// get the chain from the
			chain, err := c.GetChain(txMsg.CommitteeId)
			if err != nil {
				c.P2P.ChangeReputation(senderID, p2p.InvalidMsgRep)
				return
			}
			// if the chain is syncing, just return without handling
			if chain.isSyncing.Load() {
				return
			}
			// handle the transaction under the plugin
			if err = chain.Plugin.HandleTx(txMsg.Tx); err != nil {
				c.P2P.ChangeReputation(senderID, p2p.InvalidTxRep)
				return
			}
			// bump peer reputation positively
			c.P2P.ChangeReputation(senderID, p2p.GoodTxRep)
			// gossip the transaction to peers
			if err = c.P2P.SendToChainPeers(txMsg.CommitteeId, Tx, msg.Message); err != nil {
				c.log.Error(fmt.Sprintf("unable to gossip tx with err: %s", err.Error()))
			}
		}()

	}
}

// ListenForBlockRequests() listen for inbound block request messages from syncing peers, handles and answer them
func (c *Controller) ListenForBlockRequests() {
	// initialize a rate limiter for the inbound syncing messages
	l := lib.NewLimiter(p2p.MaxBlockReqPerWindow, c.P2P.MaxPossiblePeers()*p2p.MaxBlockReqPerWindow, p2p.BlockReqWindowS)
	for {
		select {
		// wait and execute for each inbound block request
		case msg := <-c.P2P.Inbox(BlockRequest):
			func() {
				// lock the controller for thread safety
				c.Lock()
				defer c.Unlock()
				// create a convenience variable for the sender of the block request
				senderID := msg.Sender.Address.PublicKey
				// check with the rate limiter to see if *this peer* or *all peers* are blocked
				blocked, allBlocked := l.NewRequest(lib.BytesToString(senderID))
				// if *this peer* or *all peers* are blocked
				if blocked || allBlocked {
					// if only this specific peer is blocked, slash the reputation
					if blocked {
						c.P2P.ChangeReputation(senderID, p2p.BlockReqExceededRep)
					}
					return
				}
				// try to cast the p2p msg to a block request message
				request, ok := msg.Message.(*lib.BlockRequestMessage)
				if !ok {
					c.P2P.ChangeReputation(senderID, p2p.InvalidMsgRep)
					return
				}
				// get the chain from the controller map
				chain, err := c.GetChain(request.CommitteeId)
				if err != nil {
					c.log.Error(err.Error())
					return
				}
				// if the chain is syncing, return without handling
				if chain.isSyncing.Load() {
					return
				}
				// create an empty QC that will be populated if the request is more than 'height only'
				var certificate *lib.QuorumCertificate
				// if the requesting more than just the height
				if !request.HeightOnly {
					// load the actual certificate and populate the variable
					certificate, err = chain.Plugin.LoadCertificate(request.Height)
					if err != nil {
						c.log.Error(err.Error())
						return
					}
				}
				// send the block back to the requester
				c.SendBlock(request.CommitteeId, chain.Plugin.Height(), chain.Plugin.TotalVDFIterations(), certificate, senderID)
			}()
		case <-l.TimeToReset():
			l.Reset()
		}
	}
}

// INTERNAL HELPERS BELOW

// UpdateP2PMustConnect() tells the P2P module which nodes are *required* to be connected to (usually fellow committee members or none if not in committee)
func (c *Controller) UpdateP2PMustConnect() {
	// define a list
	noDuplicates := make(map[string]*lib.PeerAddress)
	// for each chain
	for committeeID := range c.Chains {
		// get the list of committee members
		committee, err := c.CanopyFSM().GetCommitteeMembers(committeeID)
		if err != nil {
			c.log.Errorf("unable to get must connect peers for committee %d with error %s", committeeID, err.Error())
			continue
		}
		// for each member of the committee
		for _, member := range committee.ValidatorSet.ValidatorSet {
			// convert the public key to a string
			pkString := lib.BytesToString(member.PublicKey)
			// check the de-duplication map to see if the peer object already exists
			p, found := noDuplicates[pkString]
			// if the peer object doesn't exist on the list
			if !found {
				// create the peer object
				p = &lib.PeerAddress{
					PublicKey:  member.PublicKey,
					NetAddress: member.NetAddress,
					PeerMeta:   &lib.PeerMeta{Chains: []uint64{committeeID}},
				}
			} else {
				// if the peer object already exists in the list, simply add this id to its list of chains
				p.PeerMeta.Chains = append(p.PeerMeta.Chains, committeeID)
			}
			// add to the de-duplication map to ensure we don't doubly create peer objects
			noDuplicates[pkString] = p
		}
	}
	// create a slice to send to the p2p module
	var arr []*lib.PeerAddress
	// iterate through the map and add it to the slice
	for _, peerAddr := range noDuplicates {
		arr = append(arr, peerAddr)
	}
	// send the slice to the p2p module
	c.P2P.MustConnectReceiver() <- arr
}

// handlePeerBlock() validates and handles inbound Quorum Certificates from remote peers
func (c *Controller) handlePeerBlock(senderID []byte, msg *lib.BlockMessage) (qc *lib.QuorumCertificate, stillCatchingUp bool, err lib.ErrorI) {
	// define a convenience variable for certificate
	qc = msg.BlockAndCertificate
	// get the plugin and consensus module for the specific committee
	chain, e := c.GetChain(msg.CommitteeId)
	if e != nil {
		c.P2P.ChangeReputation(senderID, p2p.InvalidBlockRep)
		return
	}
	v := chain.Consensus.ValidatorSet
	// validate the quorum certificate
	if err = c.checkPeerQC(chain.Plugin.LoadMaxBlockSize(), &lib.View{
		Height:       chain.Plugin.Height(),
		CanopyHeight: c.LoadCommitteeHeightInState(msg.CommitteeId),
		NetworkId:    c.Config.NetworkID,
		CommitteeId:  msg.CommitteeId,
	}, v, qc, senderID); err != nil {
		c.P2P.ChangeReputation(senderID, p2p.InvalidBlockRep)
		return
	}
	// validates the 'block' and 'block hash' of the proposal
	stillCatchingUp, err = chain.Plugin.CheckPeerQC(msg.MaxHeight, qc)
	if err != nil {
		c.P2P.ChangeReputation(senderID, p2p.InvalidBlockRep)
		return
	}
	// attempts to commit the QC to persistence of chain by playing it against the state machine
	if err = chain.Plugin.CommitCertificate(qc); err != nil {
		c.P2P.ChangeReputation(senderID, p2p.InvalidBlockRep)
		return
	}
	return
}

// checkPeerQC() performs validity checks on a QuorumCertificate received from a peer
func (c *Controller) checkPeerQC(maxBlockSize int, view *lib.View, v lib.ValidatorSet, qc *lib.QuorumCertificate, senderID []byte) lib.ErrorI {
	// validate the quorum certificate
	isPartialQC, err := qc.Check(v, maxBlockSize, view, false)
	if err != nil {
		c.P2P.ChangeReputation(senderID, p2p.InvalidJustifyRep)
		return err
	}
	// if the quorum certificate doesn't have a +2/3rds majority
	if isPartialQC {
		c.P2P.ChangeReputation(senderID, p2p.InvalidJustifyRep)
		return lib.ErrNoMaj23()
	}
	// if the results structure was not pruned from the certificate
	if qc.Results != nil {
		c.P2P.ChangeReputation(senderID, p2p.InvalidMsgRep)
		return lib.ErrNonNilCertResults()
	}
	// if this certificate isn't finalized
	if qc.Header.Phase != lib.Phase_PRECOMMIT_VOTE {
		c.P2P.ChangeReputation(senderID, p2p.InvalidMsgRep)
		return lib.ErrWrongPhase()
	}
	// enforce the target height
	if qc.Header.Height != view.Height {
		c.P2P.ChangeReputation(senderID, p2p.InvalidMsgRep)
		return lib.ErrWrongHeight()
	}
	// enforce the last saved committee height as valid
	// NOTE: historical committees are accepted up to the last saved height in the state
	// else there's a potential for a long-range attack
	if qc.Header.CanopyHeight < view.CanopyHeight {
		c.P2P.ChangeReputation(senderID, p2p.InvalidJustifyRep)
		return lib.ErrInvalidQCCommitteeHeight()
	}
	return nil
}

// pollMaxHeight() polls all peers for their local MaxHeight and totalVDFIterations for a specific committeeID
// NOTE: unlike other P2P transmissions - RequestBlock enforces a minimum reputation on `mustConnects`
// to ensure a byzantine validator cannot cause syncing issues above max_height
func (c *Controller) pollMaxHeight(committeeID uint64, backoff int) (maxHeight, minimumVDFIterations uint64, syncingPeers []string) {
	// empty inbox to start fresh
	c.emptyInbox(Block)
	// ask all peers
	c.log.Info("Polling all peers for max height")
	syncingPeers = make([]string, 0)
	// ask only for MaxHeight not the actual QC
	c.RequestBlock(committeeID, true)
	for {
		c.log.Debug("Waiting for peer max heights")
		select {
		case m := <-c.P2P.Inbox(Block):
			response, ok := m.Message.(*lib.BlockMessage)
			if !ok {
				c.P2P.ChangeReputation(m.Sender.Address.PublicKey, p2p.InvalidMsgRep)
				continue
			}
			// don't listen to any peers below the minimumVDFIterations
			if response.TotalVdfIterations < minimumVDFIterations {
				continue
			}
			// reset syncing variables if peer exceeds the previous minimumVDFIterations
			if response.TotalVdfIterations > minimumVDFIterations {
				maxHeight, minimumVDFIterations = response.MaxHeight, response.TotalVdfIterations
				syncingPeers = make([]string, 0)
			}
			// add to syncing peer list
			syncingPeers = append(syncingPeers, lib.BytesToString(c.PublicKey))
		case <-time.After(p2p.PollMaxHeightTimeoutS * time.Second * time.Duration(backoff)):
			if maxHeight == 0 { // genesis file is 0 and first height is 1
				c.log.Warn("no heights received from peers. Trying again")
				return c.pollMaxHeight(committeeID, backoff+1)
			}
			c.log.Debugf("Waiting peer max height is %d", maxHeight)
			return
		}
	}
}

// singleNodeNetwork() returns true if there are no other participants in the committee besides self
func (c *Controller) singleNodeNetwork(committeeID uint64) bool {
	// load the committee for Canopy
	valSet, err := c.LoadCommittee(committeeID, c.CanopyFSM().Height())
	if err != nil {
		c.log.Error(err.Error())
		return false
	}
	// if self is the only validator, return true
	return len(valSet.ValidatorSet.ValidatorSet) == 1 &&
		bytes.Equal(valSet.ValidatorSet.ValidatorSet[0].PublicKey, c.PublicKey)
}

// emptyInbox() discards all unread messages for a specific topic
func (c *Controller) emptyInbox(topic lib.Topic) {
	// clear the inbox
	for len(c.P2P.Inbox(topic)) > 0 {
		<-c.P2P.Inbox(topic)
	}
}

// syncingDone() checks if the syncing loop may complete for a specific committeeID
func (c *Controller) syncingDone(committeeID, maxHeight, minVDFIterations uint64) bool {
	// get the chain from the controller map
	chain, err := c.GetChain(committeeID)
	if err != nil {
		c.log.Error(err.Error())
	}
	// if the plugin height is GTE the max height
	if chain.Plugin.Height() >= maxHeight {
		// ensure node did not lie about VDF iterations in their chain
		if chain.Plugin.TotalVDFIterations() < minVDFIterations {
			c.log.Fatalf("VDFIterations error: localVDFIterations: %d, minimumVDFIterations: %d", chain.Plugin.TotalVDFIterations(), minVDFIterations)
		}
		return true
	}
	return false
}

// finishSyncing() is called when the syncing loop is completed for a specific committeeID
func (c *Controller) finishSyncing(committeeID uint64) {
	// lock the controller
	c.Lock()
	defer c.Unlock()
	// get the chain from the controller map
	chain := c.Chains[committeeID]
	// signal a reset of bft for the chain
	chain.Consensus.ResetBFTChan() <- bft.ResetBFT{
		ProcessTime: time.Since(c.LoadLastCommitTime(committeeID, chain.Plugin.Height())),
	}
	// set syncing to false
	chain.isSyncing.Store(false)
	// set the chain in the list
	c.Chains[committeeID] = chain
	// enable listening for a block
	go c.ListenForBlock()
}

// signableToConsensusMessage() signs, encodes, and wraps a consensus message in preparation for sending
func (c *Controller) signableToConsensusMessage(committeeId uint64, msg lib.Signable) (*lib.ConsensusMessage, lib.ErrorI) {
	// sign the message
	if err := msg.Sign(c.PrivateKey); err != nil {
		return nil, err
	}
	// convert the message to bytes
	messageBytes, err := lib.Marshal(msg)
	if err != nil {
		return nil, err
	}
	// wrap the message in consensus
	return &lib.ConsensusMessage{
		CommitteeId: committeeId,
		Message:     messageBytes,
	}, nil
}

// convenience aliases that reference the library package
const (
	BlockRequest = lib.Topic_BLOCK_REQUEST
	Block        = lib.Topic_BLOCK
	Tx           = lib.Topic_TX
	Cons         = lib.Topic_CONSENSUS
)
