package controller

import (
	"bytes"
	"fmt"
	"github.com/canopy-network/canopy/bft"
	"github.com/canopy-network/canopy/fsm/types"
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
	"github.com/canopy-network/canopy/p2p"
	"math/rand"
	"strings"
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
func (c *Controller) Sync() {
	c.log.Infof("Sync started 🔄 for committee %d", c.Config.ChainId)
	// set isSyncing
	c.isSyncing.Store(true)
	// initialize tracking variables
	reqRecipient, maxHeight, minVDFIterations, syncingPeers := make([]byte, 0), uint64(0), uint64(0), make([]string, 0)
	// initialize a callback for `pollMaxHeight`
	pollMaxHeight := func() { maxHeight, minVDFIterations, syncingPeers = c.pollMaxHeight(1) }
	// check if node is alone in the committee
	if c.singleNodeNetwork() {
		c.finishSyncing()
		return
	}
	// poll max height of all peers
	pollMaxHeight()
	for {
		if c.syncingDone(maxHeight, minVDFIterations) {
			c.log.Info("Synced to top ✅")
			c.finishSyncing()
			return
		}
		// get a random peer to send to
		reqRecipient, _ = lib.StringToBytes(syncingPeers[rand.Intn(len(syncingPeers))])
		c.log.Infof("Syncing height %d 🔄 from %s", c.FSM.Height(), lib.BytesToTruncatedString(reqRecipient))
		// send the request
		c.RequestBlock(false, reqRecipient)
		select {
		case msg := <-c.P2P.Inbox(Block): // got a response
			// if the responder does not equal the requester
			responder := msg.Sender.Address.PublicKey
			c.log.Debugf("Received a block response msg from %s", lib.BytesToTruncatedString(responder))
			if !bytes.Equal(responder, reqRecipient) {
				c.P2P.ChangeReputation(responder, p2p.UnexpectedBlockRep)
				// poll max height of all peers
				pollMaxHeight()
				continue
			}
			blkResponseMsg, ok := msg.Message.(*lib.BlockMessage)
			if !ok {
				c.log.Warn("Not a block response msg")
				c.P2P.ChangeReputation(msg.Sender.Address.PublicKey, p2p.InvalidBlockRep)
				// poll max height of all peers
				pollMaxHeight()
				return
			}
			c.log.Debugf("Handling peer block response msg")
			// process the quorum certificate received from the peer
			if _, _, err := c.handlePeerBlock(msg.Sender.Address.PublicKey, blkResponseMsg); err != nil {
				c.log.Warnf("Peer block invalid:\n%s", err.Error())
				// poll max height of all peers
				pollMaxHeight()
				continue
			}
			// each peer is individually polled with each request as well
			// if that poll max height has grown, we accept that as
			// the new max height
			if blkResponseMsg.MaxHeight > maxHeight && blkResponseMsg.TotalVdfIterations >= minVDFIterations {
				maxHeight, minVDFIterations = blkResponseMsg.MaxHeight, blkResponseMsg.TotalVdfIterations
				c.log.Debugf("Updated chain %d with max height: %d and iterations %d\n%s", c.Config.ChainId, maxHeight, minVDFIterations)
			}
			c.P2P.ChangeReputation(responder, p2p.GoodBlockRep)
		case <-time.After(p2p.SyncTimeoutS * time.Second): // timeout
			c.log.Warnf("Timeout waiting for sync block")
			c.P2P.ChangeReputation(reqRecipient, p2p.TimeoutRep)
			// poll max height of all peers
			pollMaxHeight()
			continue
		}
	}
}

// PUBLISHERS BELOW

// SendTxMsg() gossips a Transaction through the P2P network for a specific committeeID
func (c *Controller) SendTxMsg(tx []byte) lib.ErrorI {
	// create a transaction message object using the tx bytes and the committee id
	msg := &lib.TxMessage{CommitteeId: c.Config.ChainId, Tx: tx}
	// send it to self for de-duplication and awareness of self originated transactions
	if err := c.P2P.SelfSend(c.PublicKey, Tx, msg); err != nil {
		return err
	}
	// gossip to all the peers for the chain
	return c.P2P.SendToPeers(Tx, msg)
}

// SendCertificateResultsTx() originates and auto-sends a CertificateResultsTx after successfully leading a Consensus height
func (c *Controller) SendCertificateResultsTx(qc *lib.QuorumCertificate) {
	c.log.Debugf("Sending certificate results txn for: %s", lib.BytesToString(qc.ResultsHash))
	// save the block to set it back to the object after this function completes
	blk := qc.Block
	defer func() { qc.Block = blk }()
	// it's good practice to omit the block when sending the transaction as it's not relevant to canopy
	qc.Block = nil
	tx, err := types.NewCertificateResultsTx(c.PrivateKey, qc, 0, "")
	if err != nil {
		c.log.Errorf("Creating auto-certificate-results-txn failed with err: %s", err.Error())
		return
	}
	hash, err := c.RemoteCallbacks.Transaction(tx)
	if err != nil {
		c.log.Errorf("Submitting auto-certificate-results-txn failed with err: %s", err.Error())
		return
	}
	c.log.Infof("Successfully submitted the certificate-results-txn with hash %s", *hash)
}

// GossipBlockMsg() gossips a QuorumCertificate (with block) through the P2P network for a specific committeeID
func (c *Controller) GossipBlock(qc *lib.QuorumCertificate) {
	// save the results to add back after this function completes
	results := qc.Results
	defer func() { qc.Results = results }()
	// when sending a certificate message, it's good practice to omit the 'results' field as it is only important for the Canopy Blockchain
	qc.Results = nil
	c.log.Debugf("Gossiping certificate: %s", lib.BytesToString(qc.ResultsHash))
	// create the block message
	blockMessage := &lib.BlockMessage{
		CommitteeId:         c.Config.ChainId,
		MaxHeight:           c.FSM.Height(),
		TotalVdfIterations:  c.FSM.TotalVDFIterations(),
		BlockAndCertificate: qc,
	}
	// gossip the block message to peers
	if err := c.P2P.SendToPeers(Block, blockMessage); err != nil {
		c.log.Errorf("unable to gossip block with err: %s", err.Error())
	}
	// if a single node network - send to self
	if c.singleNodeNetwork() {
		if err := c.P2P.SelfSend(c.PublicKey, Block, blockMessage); err != nil {
			c.log.Errorf("unable to self send block with err: %s", err.Error())
		}
	}
	c.log.Debugf("gossiping done")
}

// RequestBlock() sends a QuorumCertificate (block + certificate) request to peer(s) - `heightOnly` is a request for just the peer's max height
func (c *Controller) RequestBlock(heightOnly bool, recipients ...[]byte) {
	height := c.FSM.Height()
	// if the optional 'recipients' is specified
	if len(recipients) != 0 {
		// for each 'recipient'
		for _, pk := range recipients {
			c.log.Debugf("Requesting block %d for chain %d from %s", height, c.Config.ChainId, lib.BytesToTruncatedString(pk))
			// send it to exactly who was specified in the function call
			if err := c.P2P.SendTo(pk, BlockRequest, &lib.BlockRequestMessage{
				CommitteeId: c.Config.ChainId,
				Height:      height,
				HeightOnly:  heightOnly,
			}); err != nil {
				c.log.Error(err.Error())
			}
		}
	} else {
		c.log.Debugf("Requesting block %d for chain %d from all", height, c.Config.ChainId)
		// send it to the peers
		if err := c.P2P.SendToPeers(BlockRequest, &lib.BlockRequestMessage{
			CommitteeId: c.Config.ChainId,
			Height:      height,
			HeightOnly:  heightOnly,
		}, true); err != nil {
			c.log.Error(err.Error())
		}
	}
}

// SendBlock() responds to a `blockRequest` message to a peer - always sending the self.MaxHeight and sometimes sending the actual block and supporting QC
func (c *Controller) SendBlock(maxHeight, vdfIterations uint64, blockAndCert *lib.QuorumCertificate, recipient []byte) {
	// send the block to the recipient public key specified
	if err := c.P2P.SendTo(recipient, Block, &lib.BlockMessage{
		CommitteeId:         c.Config.ChainId,
		MaxHeight:           maxHeight,
		TotalVdfIterations:  vdfIterations,
		BlockAndCertificate: blockAndCert,
	}); err != nil {
		c.log.Error(err.Error())
	}
}

// SendToReplicas() sends a bft message to a specific ValidatorSet (the Committee)
func (c *Controller) SendToReplicas(replicas lib.ValidatorSet, msg lib.Signable) {
	c.log.Debugf("Sending to %d replicas", replicas.NumValidators)
	// handle the signable message
	message, err := c.signableToConsensusMessage(msg)
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
func (c *Controller) SendToProposer(msg lib.Signable) {
	// handle the signable message
	message, err := c.signableToConsensusMessage(msg)
	if err != nil {
		c.log.Error(err.Error())
		return
	}
	// check if sending to self or peer
	if c.Consensus.SelfIsProposer() {
		// handle self send
		if err = c.P2P.SelfSend(c.PublicKey, Cons, message); err != nil {
			c.log.Error(err.Error())
		}
	} else {
		// handle peer send
		if err = c.P2P.SendTo(c.Consensus.ProposerKey, Cons, message); err != nil {
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
			c.log.Infof("Received new block from %s ✉️", lib.BytesToTruncatedString(msg.Sender.Address.PublicKey))
			// try to cast the message to a block message
			blockMessage, ok := msg.Message.(*lib.BlockMessage)
			// if not a block message, slash the peer
			if !ok {
				c.log.Debug("Invalid Peer Block Message")
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
				go c.Sync()
				// signal exit the out loop
				quit = true
				return
			}
			if err != nil {
				return
			}
			// gossip the block to the node's peers
			c.GossipBlock(qc)
			// reset consensus
			c.Consensus.ResetBFT <- bft.ResetBFT{ProcessTime: time.Since(startTime)}
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
			defer func() { c.Unlock(); c.log.Debug("Done handling inbound consensus message") }()
			c.log.Debug("Handling inbound consensus message")
			// try to cast the p2p message to a 'consensus message'
			consMsg, ok := msg.Message.(*lib.ConsensusMessage)
			// if cast unsuccessful
			if !ok {
				c.P2P.ChangeReputation(msg.Sender.Address.PublicKey, p2p.InvalidMsgRep)
				return
			}
			// if the chain is syncing, return
			if c.isSyncing.Load() {
				return
			}
			// load the committee associated with the committee id at the latest canopy height
			vs, err := c.LoadCommittee(c.FSM.Height())
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
			if err = c.Consensus.HandleMessage(bftMsg); err != nil {
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
				c.log.Warnf("Non-Tx message from %s", lib.BytesToTruncatedString(senderID))
				c.P2P.ChangeReputation(senderID, p2p.InvalidMsgRep)
				return
			}
			// if the message is empty
			if txMsg == nil {
				c.log.Warnf("Empty tx message from %s", lib.BytesToTruncatedString(senderID))
				c.P2P.ChangeReputation(senderID, p2p.InvalidMsgRep)
				return
			}
			// if the chain is syncing, just return without handling
			if c.isSyncing.Load() {
				return
			}
			// handle the transaction under the plugin
			if err := c.HandleTransaction(txMsg.Tx); err != nil {
				if err.Error() == lib.ErrTxFoundInMempool(crypto.HashString(txMsg.Tx)).Error() {
					return
				}
				c.log.Warnf("Handle tx from %s failed with err: %s", lib.BytesToTruncatedString(senderID), err.Error())
				c.P2P.ChangeReputation(senderID, p2p.InvalidTxRep)
				return
			}
			c.log.Infof("Received valid transaction %s from %s for chain %d", crypto.ShortHashString(txMsg.Tx), lib.BytesToString(senderID)[:20], txMsg.CommitteeId)
			// bump peer reputation positively
			c.P2P.ChangeReputation(senderID, p2p.GoodTxRep)
			// gossip the transaction to peers
			if err := c.P2P.SendToPeers(Tx, msg.Message); err != nil {
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
						c.log.Warnf("Rate-limit hit for peer %s", lib.BytesToTruncatedString(senderID))
						c.P2P.ChangeReputation(senderID, p2p.BlockReqExceededRep)
					}
					return
				}
				// try to cast the p2p msg to a block request message
				request, ok := msg.Message.(*lib.BlockRequestMessage)
				if !ok {
					c.log.Warnf("Invalid block-request msg from peer %s", lib.BytesToTruncatedString(senderID))
					c.P2P.ChangeReputation(senderID, p2p.InvalidMsgRep)
					return
				}
				c.log.Debugf("Received a block request from %s", lib.BytesToString(senderID[:20]))
				// create an empty QC that will be populated if the request is more than 'height only'
				var certificate *lib.QuorumCertificate
				var err error
				// if the requesting more than just the height
				if !request.HeightOnly {
					// load the actual certificate and populate the variable
					certificate, err = c.LoadCertificate(request.Height)
					if err != nil {
						c.log.Error(err.Error())
						return
					}
				}
				c.log.Debugf("Responding to a block request from %s, heightOnly=%t", lib.BytesToString(senderID[:20]), request.HeightOnly)
				// send the block back to the requester
				c.SendBlock(c.FSM.Height(), c.FSM.TotalVDFIterations(), certificate, senderID)
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
	port, err := lib.ResolvePort(c.Config.ChainId)
	if err != nil {
		if err != nil {
			c.log.Error(err.Error())
			return
		}
	}
	// ensure self is a validator
	var selfIsValidator bool
	// for each member of the committee
	for _, member := range c.BaseChainInfo.ValidatorSet.ValidatorSet.ValidatorSet {
		if bytes.Equal(member.PublicKey, c.PublicKey) {
			selfIsValidator = true
		}
		// convert the public key to a string
		pkString := lib.BytesToString(member.PublicKey)
		// check the de-duplication map to see if the peer object already exists
		p, found := noDuplicates[pkString]
		// if the peer object doesn't exist on the list
		if !found {
			// create the peer object
			p = &lib.PeerAddress{
				PublicKey:  member.PublicKey,
				NetAddress: strings.ReplaceAll(member.NetAddress, "tcp://", "") + port,
				PeerMeta:   &lib.PeerMeta{ChainId: c.Config.ChainId},
			}
		}
		// add to the de-duplication map to ensure we don't doubly create peer objects
		noDuplicates[pkString] = p
	}
	// if self isn't a validator - don't force P2P to connect to other validators
	if !selfIsValidator {
		c.log.Warnf("Self not a validator so not connecting to %d validators", len(noDuplicates))
		return
	}
	// create a slice to send to the p2p module
	var arr []*lib.PeerAddress
	// iterate through the map and add it to the slice
	for _, peerAddr := range noDuplicates {
		arr = append(arr, peerAddr)
	}
	c.log.Infof("Updating must connects with %d validators", len(arr))
	// send the slice to the p2p module
	c.P2P.MustConnectsReceiver <- arr
}

// handlePeerBlock() validates and handles inbound Quorum Certificates from remote peers
func (c *Controller) handlePeerBlock(senderID []byte, msg *lib.BlockMessage) (qc *lib.QuorumCertificate, stillSyncing bool, err lib.ErrorI) {
	// define a convenience variable for certificate
	qc = msg.BlockAndCertificate
	v := c.Consensus.ValidatorSet
	// validate the quorum certificate
	if err = c.checkPeerQC(c.LoadMaxBlockSize(), &lib.View{
		Height:       c.FSM.Height(),
		CanopyHeight: c.BaseChainInfo.Height,
		NetworkId:    c.Config.NetworkID,
		CommitteeId:  msg.CommitteeId,
	}, v, qc, senderID); err != nil {
		return
	}
	// validates the 'block' and 'block hash' of the proposal
	stillSyncing, err = c.CheckPeerQC(msg.MaxHeight, qc)
	if err != nil {
		c.P2P.ChangeReputation(senderID, p2p.InvalidBlockRep)
		c.log.Warnf("Plugin.CheckPeerQC from %s failed with error: %s", lib.BytesToTruncatedString(senderID), err.Error())
		return
	}
	// attempts to commit the QC to persistence of chain by playing it against the state machine
	if err = c.CommitCertificate(qc); err != nil {
		c.P2P.ChangeReputation(senderID, p2p.InvalidBlockRep)
		c.log.Warnf("CommitCertificate from %s: %s", lib.BytesToTruncatedString(senderID), err.Error())
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
		return lib.ErrWrongHeight()
	}
	// enforce the last saved committee height as valid
	// NOTE: historical committees are accepted up to the last saved height in the state
	// else there's a potential for a long-range attack
	if qc.Header.CanopyHeight < view.CanopyHeight {
		c.P2P.ChangeReputation(senderID, p2p.InvalidJustifyRep)
		return lib.ErrWrongCanopyHeight()
	}
	return nil
}

// pollMaxHeight() polls all peers for their local MaxHeight and totalVDFIterations for a specific committeeID
// NOTE: unlike other P2P transmissions - RequestBlock enforces a minimum reputation on `mustConnects`
// to ensure a byzantine validator cannot cause syncing issues above max_height
func (c *Controller) pollMaxHeight(backoff int) (max, minVDFIterations uint64, syncingPeers []string) {
	maxHeight, minimumVDFIterations := -1, -1
	// empty inbox to start fresh
	c.emptyInbox(Block)
	// ask all peers
	c.log.Infof("Polling chain peers for max height")
	syncingPeers = make([]string, 0)
	// ask only for MaxHeight not the actual QC
	c.RequestBlock(true)
	for {
		c.log.Debug("Waiting for peer max heights")
		select {
		case m := <-c.P2P.Inbox(Block):
			response, ok := m.Message.(*lib.BlockMessage)
			if !ok {
				c.log.Warnf("Invalid block message response from %s", lib.BytesToTruncatedString(m.Sender.Address.PublicKey))
				c.P2P.ChangeReputation(m.Sender.Address.PublicKey, p2p.InvalidMsgRep)
				continue
			}
			c.log.Debugf("Received a block response from peer %s with max height at %d", lib.BytesToTruncatedString(m.Sender.Address.PublicKey), maxHeight)
			// don't listen to any peers below the minimumVDFIterations
			if int(response.TotalVdfIterations) < minimumVDFIterations {
				c.log.Error("Ignoring below minimumVDFIterations")
				continue
			}
			// reset syncing variables if peer exceeds the previous minimumVDFIterations
			maxHeight, minimumVDFIterations = int(response.MaxHeight), int(response.TotalVdfIterations)
			syncingPeers = make([]string, 0)
			// add to syncing peer list
			syncingPeers = append(syncingPeers, lib.BytesToString(m.Sender.Address.PublicKey))
		case <-time.After(p2p.PollMaxHeightTimeoutS * time.Second * time.Duration(backoff)):
			if maxHeight == -1 || minimumVDFIterations == -1 {
				c.log.Warn("no heights received from peers. Trying again")
				return c.pollMaxHeight(backoff + 1)
			}
			c.log.Debugf("Peer max height is %d 🔝", maxHeight)
			return uint64(maxHeight), uint64(minimumVDFIterations), syncingPeers
		}
	}
}

// singleNodeNetwork() returns true if there are no other participants in the committee besides self
func (c *Controller) singleNodeNetwork() bool {
	// if self is the only validator, return true
	return c.BaseChainInfo.ValidatorSet.NumValidators == 1 &&
		bytes.Equal(c.BaseChainInfo.ValidatorSet.ValidatorSet.ValidatorSet[0].PublicKey, c.PublicKey)
}

// emptyInbox() discards all unread messages for a specific topic
func (c *Controller) emptyInbox(topic lib.Topic) {
	// clear the inbox
	for len(c.P2P.Inbox(topic)) > 0 {
		<-c.P2P.Inbox(topic)
	}
}

// syncingDone() checks if the syncing loop may complete for a specific committeeID
func (c *Controller) syncingDone(maxHeight, minVDFIterations uint64) bool {
	// if the plugin height is GTE the max height
	if c.FSM.Height() >= maxHeight {
		// ensure node did not lie about VDF iterations in their chain
		if c.FSM.TotalVDFIterations() < minVDFIterations {
			c.log.Fatalf("VDFIterations error: localVDFIterations: %d, minimumVDFIterations: %d", c.FSM.TotalVDFIterations(), minVDFIterations)
		}
		return true
	}
	return false
}

// finishSyncing() is called when the syncing loop is completed for a specific committeeID
func (c *Controller) finishSyncing() {
	// lock the controller
	c.Lock()
	defer c.Unlock()
	// signal a reset of bft for the chain
	c.Consensus.ResetBFT <- bft.ResetBFT{ProcessTime: time.Since(c.LoadLastCommitTime(c.FSM.Height()))}
	// set syncing to false
	c.isSyncing.Store(false)
	// enable listening for a block
	go c.ListenForBlock()
}

// signableToConsensusMessage() signs, encodes, and wraps a consensus message in preparation for sending
func (c *Controller) signableToConsensusMessage(msg lib.Signable) (*lib.ConsensusMessage, lib.ErrorI) {
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
		CommitteeId: c.Config.ChainId,
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
