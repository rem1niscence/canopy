package controller

import (
	"bytes"
	"fmt"
	"github.com/canopy-network/canopy/bft"
	"github.com/canopy-network/canopy/fsm/types"
	"github.com/canopy-network/canopy/lib"
	"math/rand"
	"time"
)

const (
	PollMaxHeightTimeoutS = 5  // wait time for polling the maximum height of the peers
	SyncTimeoutS          = 5  // wait time to receive an individual block (certificate) from a peer during syncing
	MaxBlockReqPerWindow  = 5  // maximum block (certificate) requests per window per requester
	BlockReqWindowS       = 10 // the 'window of time' before resetting limits for block (certificate) requests

	GoodBlockRep        = 3  // rep boost for sending us a valid block (certificate)
	GoodTxRep           = 3  // rep boost for sending us a valid transaction (certificate)
	TimeoutRep          = -1 // rep slash for not responding in time
	UnexpectedBlockRep  = -1 // rep slash for sending us a block we weren't expecting
	InvalidTxRep        = -3 // rep slash for sending us an invalid transaction
	NotValRep           = -3 // rep slash for sending us a validator only message but not being a validator
	InvalidMsgRep       = -3 // rep slash for sending an invalid message
	InvalidBlockRep     = -3 // rep slash for sending an invalid block (certificate) message
	InvalidJustifyRep   = -3 // rep slash for sending an invalid certificate justification
	BlockReqExceededRep = -3 // rep slash for over-requesting blocks (certificates)
)

// Sync() attempts to sync the blockchain for a specific CommitteeID
// 1) Get the height and begin block params from the state_machine
// 2) Get peer max_height from P2P
// 3) Ask peers for a block at a time
// 4) Check each block by checking if cert was signed by +2/3 maj (COMMIT message)
// 5) Commit block against the state machine, if error fatal
// 5) Do this until reach the max-peer-height
// 6) Stay on top by listening to incoming cert messages
func (c *Controller) Sync(committeeID uint64) {
	c.log.Infof("Sync started 🔄 for committee %d", committeeID)
	// set isSyncing
	c.isSyncing[committeeID].Store(true)
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
		c.SendCertReqMsg(committeeID, false, reqRecipient)
		select {
		case msg := <-c.P2P.Inbox(Certificate): // got a response
			// if the responder does not equal the requester
			responder := msg.Sender.Address.PublicKey
			if !bytes.Equal(responder, reqRecipient) {
				c.P2P.ChangeReputation(responder, UnexpectedBlockRep)
				// poll max height of all peers
				pollMaxHeight()
				continue
			}
			blkResponseMsg, ok := msg.Message.(*lib.BlockResponseMessage)
			if !ok {
				c.P2P.ChangeReputation(msg.Sender.Address.PublicKey, InvalidBlockRep)
				// poll max height of all peers
				pollMaxHeight()
				return
			}
			// process the quorum certificate received by the peer
			if _, _, err := c.handlePeerCert(msg.Sender.Address.PublicKey, blkResponseMsg); err != nil {
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
			c.P2P.ChangeReputation(responder, GoodBlockRep)
		case <-time.After(SyncTimeoutS * time.Second): // timeout
			c.P2P.ChangeReputation(reqRecipient, TimeoutRep)
			// poll max height of all peers
			pollMaxHeight()
			continue
		}
	}
}

// PUBLISHERS BELOW

// SendTxMsg() gossips a Transaction through the P2P network for a specific committeeID
func (c *Controller) SendTxMsg(committeeID uint64, tx []byte) lib.ErrorI {
	msg := &lib.TxMessage{Tx: tx, CommitteeId: committeeID}
	if err := c.P2P.SelfSend(c.PublicKey, Tx, msg); err != nil {
		return err
	}
	if err := c.P2P.SendToChainPeers(committeeID, Tx, msg); err != nil {
		c.log.Error(fmt.Sprintf("unable to gossip tx with err: %s", err.Error()))
	}
	return nil
}

// SendCertificateResultsTx() originates and auto-sends a ProposalTx after successfully leading a Consensus height
func (c *Controller) SendCertificateResultsTx(committeeID uint64, qc *lib.QuorumCertificate) {
	c.log.Debugf("Sending certificate results txn for: %s", lib.BytesToString(qc.ResultsHash))
	tx, err := types.NewCertificateResultsTx(c.PrivateKey, qc, 0)
	if err != nil {
		c.log.Errorf("Creating auto-certificate-results-txn failed with err: %s", err.Error())
		return
	}
	pool, err := c.FSM.GetPool(qc.Header.CommitteeId)
	if err != nil || pool.Amount == 0 {
		c.log.Errorf("Auto-proposal-txn stopped due to not subsidized")
		return // not subsidized
	}
	bz, err := lib.Marshal(tx)
	if err != nil {
		c.log.Errorf("Marshalling auto-proposal-txn failed with err: %s", err.Error())
		return
	}
	if err = c.SendTxMsg(committeeID, bz); err != nil {
		c.log.Errorf("Gossiping auto-proposal-txn failed with err: %s", err.Error())
		return
	}
}

// SendCertMsg() gossips a QuorumCertificate (with block) through the P2P network for a specific committeeID
func (c *Controller) SendCertMsg(committeeID uint64, qc *lib.QuorumCertificate) {
	qc.Results = nil // omit the results
	plug, _, err := c.GetPluginAndConsensus(committeeID)
	if err != nil {
		c.log.Errorf("unable to gossip block with err: %s", err.Error())
		return
	}
	c.log.Debugf("Gossiping certificate: %s", lib.BytesToString(qc.ResultsHash))
	blockMessage := &lib.BlockResponseMessage{
		CommitteeId:         committeeID,
		MaxHeight:           plug.Height(),
		TotalVdfIterations:  plug.TotalVDFIterations(),
		BlockAndCertificate: qc,
	}
	if err = c.P2P.SendToChainPeers(committeeID, Certificate, blockMessage); err != nil {
		c.log.Errorf("unable to gossip block with err: %s", err.Error())
	}
	c.log.Debugf("gossiping done")
}

// SendCertReqMsg() sends a QuorumCertificate request to peer(s) - `heightOnly` is a request for just the peer's max height
func (c *Controller) SendCertReqMsg(committeeID uint64, heightOnly bool, sendTo ...[]byte) {
	plug, _, err := c.GetPluginAndConsensus(committeeID)
	if err != nil {
		c.log.Errorf("unable to gossip block with err: %s", err.Error())
		return
	}
	if len(sendTo) == 0 {
		if err = c.P2P.SendToChainPeers(committeeID, CertificateRequest, &lib.BlockRequestMessage{
			CommitteeId: committeeID,
			Height:      plug.Height(),
			HeightOnly:  heightOnly,
		}, true); err != nil {
			c.log.Error(err.Error())
		}
	} else {
		for _, pk := range sendTo {
			if err = c.P2P.SendTo(pk, CertificateRequest, &lib.BlockRequestMessage{
				CommitteeId: committeeID,
				Height:      plug.Height(),
				HeightOnly:  heightOnly,
			}); err != nil {
				c.log.Error(err.Error())
			}
		}
	}
}

// SendBlockRespMsg() responds to a `blockRequest` message to peer(s) - always sending the self.MaxHeight and sometimes sending the actual block and supporting QC
func (c *Controller) SendBlockRespMsg(committeeID, maxHeight, vdfIterations uint64, blockAndCert *lib.QuorumCertificate, sendTo ...[]byte) {
	var err error
	if len(sendTo) == 0 {
		if err = c.P2P.SendToChainPeers(committeeID, Certificate, &lib.BlockResponseMessage{
			CommitteeId:         committeeID,
			MaxHeight:           maxHeight,
			TotalVdfIterations:  vdfIterations,
			BlockAndCertificate: blockAndCert,
		}); err != nil {
			c.log.Error(err.Error())
		}
	} else {
		for _, pk := range sendTo {
			if err = c.P2P.SendTo(pk, Certificate, &lib.BlockResponseMessage{
				CommitteeId:         committeeID,
				MaxHeight:           maxHeight,
				TotalVdfIterations:  vdfIterations,
				BlockAndCertificate: blockAndCert,
			}); err != nil {
				c.log.Error(err.Error())
			}
		}
	}
}

// SendConsMsgToReplicas() sends a bft message to a specific ValidatorSet (typically the Committee)
func (c *Controller) SendConsMsgToReplicas(committeeID uint64, replicas lib.ValidatorSet, msg lib.Signable) {
	if err := msg.Sign(c.PrivateKey); err != nil {
		c.log.Error(err.Error())
		return
	}
	consMsgBz, err := lib.Marshal(msg)
	if err != nil {
		c.log.Error(err.Error())
		return
	}
	wrapper := &lib.ConsensusMessage{
		CommitteeId: committeeID,
		Message:     consMsgBz,
	}
	for _, replica := range replicas.ValidatorSet.ValidatorSet {
		if bytes.Equal(replica.PublicKey, c.PublicKey) {
			if err = c.P2P.SelfSend(c.PublicKey, Cons, wrapper); err != nil {
				c.log.Error(err.Error())
			}
		}
		if err = c.P2P.SendTo(replica.PublicKey, Cons, wrapper); err != nil {
			c.log.Warn(err.Error())
		}
	}
}

// SendConsMsgToProposer() sends a bft message to the leader of the Consensus round
func (c *Controller) SendConsMsgToProposer(committeeID uint64, msg lib.Signable) {
	if err := msg.Sign(c.PrivateKey); err != nil {
		c.log.Error(err.Error())
		return
	}
	consMsgBz, err := lib.Marshal(msg)
	if err != nil {
		c.log.Error(err.Error())
		return
	}
	wrapper := &lib.ConsensusMessage{
		CommitteeId: committeeID,
		Message:     consMsgBz,
	}
	_, cons, err := c.GetPluginAndConsensus(committeeID)
	if err != nil {
		c.log.Error(err.Error())
		return
	}
	if cons.SelfIsProposer() {
		if err = c.P2P.SelfSend(c.PublicKey, Cons, wrapper); err != nil {
			c.log.Error(err.Error())
		}
	} else {
		if err = c.P2P.SendTo(cons.ProposerKey, Cons, wrapper); err != nil {
			c.log.Error(err.Error())
			return
		}
	}
}

// LISTENERS BELOW

// StartListeners() runs all listeners on separate threads
func (c *Controller) StartListeners() {
	c.log.Debug("Listening for inbound txs, block requests, and consensus messages")
	go c.ListenForCertificateRequests()
	go c.ListenForConsensus()
	go c.ListenForTx()
}

// ListenForCertificate() subscribes to -> validates -> internally routes -> and gossips - inbound Quorum Certificate messages
// - all nodes receive new blocks authorized by the quorum via this listener
func (c *Controller) ListenForCertificate() {
	c.log.Debug("Listening for inbound blocks")
	cache := lib.NewMessageCache()
	for msg := range c.P2P.Inbox(Certificate) {
		quit := false
		func() {
			c.Lock()
			defer c.Unlock()
			startTime := time.Now()
			if ok := cache.Add(msg); !ok {
				return
			}
			c.log.Infof("Received new block from %s 📥", lib.BzToTruncStr(msg.Sender.Address.PublicKey))
			blkResponseMsg, ok := msg.Message.(*lib.BlockResponseMessage)
			if !ok {
				c.P2P.ChangeReputation(msg.Sender.Address.PublicKey, InvalidBlockRep)
				return
			}
			qc, outOfSync, err := c.handlePeerCert(msg.Sender.Address.PublicKey, blkResponseMsg)
			if outOfSync {
				c.log.Warnf("Node fell out of sync for committeeID: %d", blkResponseMsg.CommitteeId)
				go c.Sync(blkResponseMsg.CommitteeId)
				quit = true
				return
			}
			if err != nil {
				c.log.Error(err.Error())
				c.P2P.ChangeReputation(msg.Sender.Address.PublicKey, InvalidBlockRep)
				return
			}
			c.SendCertMsg(blkResponseMsg.CommitteeId, qc)
			// NEW TARGET BLOCK MEANS NEW_HEIGHT FOR TARGET BFT
			if blkResponseMsg.CommitteeId != lib.CanopyCommitteeId {
				c.Consensus[blkResponseMsg.CommitteeId].ResetBFTChan() <- bft.ResetBFT{ProcessTime: time.Since(startTime)}
				return
			}
			// NEW CANOPY BLOCK = RESET ALL BFTs WITH UPDATED COMMITTEES TO PREVENT CONFLICTING VALIDATOR SETS AMONG PEERS
			newCanopyHeight := c.FSM.Height()
			for _, cons := range c.Consensus {
				if cons.CommitteeId == lib.CanopyCommitteeId {
					continue
				}
				valSet, e := c.LoadCommittee(cons.CommitteeId, newCanopyHeight)
				if e != nil {
					c.log.Error(e.Error())
					continue
				}
				cons.ResetBFTChan() <- bft.ResetBFT{UpdatedCommitteeSet: valSet, UpdatedCommitteeHeight: newCanopyHeight}
			}
			c.UpdateP2PMustConnect()
		}()
		if quit {
			return
		}
	}
}

// ListenForConsensus() subscribes to -> validates -> and internally routes - inbound Consensus messages
func (c *Controller) ListenForConsensus() {
	for msg := range c.P2P.Inbox(Cons) {
		if c.isAnySyncing() {
			continue
		}
		func() {
			c.Lock()
			defer c.Unlock()
			consMsg, ok := msg.Message.(*lib.ConsensusMessage)
			if !ok {
				c.P2P.ChangeReputation(msg.Sender.Address.PublicKey, InvalidMsgRep)
				return
			}
			handleErr := func(e error, delta int32) {
				c.log.Error(e.Error())
				c.P2P.ChangeReputation(msg.Sender.Address.PublicKey, delta)
			}
			consensus, ok := c.Consensus[consMsg.CommitteeId]
			if !ok {
				return
			}
			vs, e := c.LoadCommittee(consMsg.CommitteeId, c.FSM.Height())
			if e != nil {
				handleErr(e, 0)
				return
			}
			if _, err := vs.GetValidator(msg.Sender.Address.PublicKey); err != nil {
				handleErr(e, NotValRep)
				return
			}
			if err := consensus.HandleMessage(msg.Message); err != nil {
				handleErr(e, InvalidMsgRep)
				return
			}
		}()
	}
}

// ListenForTx() subscribes to -> validates -> internally routes -> and gossips - inbound Transaction messages
func (c *Controller) ListenForTx() {
	cache := lib.NewMessageCache()
	for msg := range c.P2P.Inbox(Tx) {
		func() {
			c.Lock()
			defer c.Unlock()
			if c.isAnySyncing() {
				return
			}
			if ok := cache.Add(msg); !ok {
				return
			}
			senderID := msg.Sender.Address.PublicKey
			txMsg, ok := msg.Message.(*lib.TxMessage)
			if !ok {
				c.P2P.ChangeReputation(senderID, InvalidMsgRep)
				return
			}
			if txMsg == nil {
				c.P2P.ChangeReputation(senderID, InvalidMsgRep)
				return
			}
			plug, _, err := c.GetPluginAndConsensus(txMsg.CommitteeId)
			if err != nil {
				c.P2P.ChangeReputation(senderID, InvalidMsgRep)
				return
			}
			if err = plug.HandleTx(txMsg.Tx); err != nil {
				c.P2P.ChangeReputation(senderID, InvalidTxRep)
				return
			}
			c.P2P.ChangeReputation(senderID, GoodTxRep)
			if err = c.P2P.SendToChainPeers(txMsg.CommitteeId, Tx, msg.Message); err != nil {
				c.log.Error(fmt.Sprintf("unable to gossip tx with err: %s", err.Error()))
			}
		}()

	}
}

// ListenForCertificateRequests() subscribes to -> validates -> internally routes -> and responds to - inbound CertificateRequest messages
func (c *Controller) ListenForCertificateRequests() {
	l := lib.NewLimiter(MaxBlockReqPerWindow, c.P2P.MaxPossiblePeers()*MaxBlockReqPerWindow, BlockReqWindowS)
	for {
		select {
		case msg := <-c.P2P.Inbox(CertificateRequest):
			func() {
				c.Lock()
				defer c.Unlock()
				if c.isAnySyncing() {
					return
				}
				senderID := msg.Sender.Address.PublicKey
				blocked, allBlocked := l.NewRequest(lib.BytesToString(senderID))
				if blocked || allBlocked {
					if blocked {
						c.P2P.ChangeReputation(senderID, BlockReqExceededRep)
					}
					return
				}
				request, ok := msg.Message.(*lib.BlockRequestMessage)
				if !ok {
					c.P2P.ChangeReputation(senderID, InvalidMsgRep)
					return
				}
				plug, _, err := c.GetPluginAndConsensus(request.CommitteeId)
				if err != nil {
					c.log.Error(err.Error())
					return
				}
				var cert *lib.QuorumCertificate
				if !request.HeightOnly {
					cert, err = plug.LoadCertificate(request.Height)
					if err != nil {
						c.log.Error(err.Error())
						return
					}
				}
				c.SendBlockRespMsg(request.CommitteeId, plug.Height(), plug.TotalVDFIterations(), cert, senderID)
			}()
		case <-l.C():
			l.Reset()
		}
	}
}

// INTERNAL HELPERS BELOW

// UpdateP2PMustConnect() tells the P2P module which nodes must be connected to (usually fellow committee members or none if not in committee)
func (c *Controller) UpdateP2PMustConnect() {
	m, s := make(map[string]*lib.PeerAddress), make([]*lib.PeerAddress, 0)
	for committeeID := range c.Consensus {
		committee, err := c.FSM.GetCommitteeMembers(committeeID)
		if err != nil {
			c.log.Errorf("unable to get must connect peers for committee %d with error %s", committeeID, err.Error())
			continue
		}
		for _, member := range committee.ValidatorSet.ValidatorSet {
			pkString := lib.BytesToString(member.PublicKey)
			p, found := m[pkString]
			if !found {
				p = &lib.PeerAddress{
					PublicKey:  member.PublicKey,
					NetAddress: member.NetAddress,
					PeerMeta: &lib.PeerMeta{
						Chains: []uint64{committeeID},
					},
				}
			} else {
				p.PeerMeta.Chains = append(p.PeerMeta.Chains, committeeID)
			}
			m[pkString] = p
		}
	}
	for _, peerAddr := range m {
		s = append(s, peerAddr)
	}
	c.P2P.MustConnectReceiver() <- s
}

// handlePeerCert() validates and handles inbound Quorum Certificates from remote peers
func (c *Controller) handlePeerCert(senderID []byte, msg *lib.BlockResponseMessage) (qc *lib.QuorumCertificate, stillCatchingUp bool, err lib.ErrorI) {
	// define an error handling function
	handleErr, qc := func(err error) {
		c.P2P.ChangeReputation(senderID, InvalidBlockRep)
		c.log.Error(err.Error())
	}, msg.BlockAndCertificate
	// get the plugin and consensus module for the specific committee
	plug, cons, e := c.GetPluginAndConsensus(msg.CommitteeId)
	if e != nil {
		handleErr(e)
		return
	}
	v := cons.ValidatorSet
	// validate the quorum certificate
	if err = c.checkPeerQC(plug.LoadMaxBlockSize(), &lib.View{
		Height:          plug.Height() + 1,
		CommitteeHeight: c.LoadCommitteeHeightInState(msg.CommitteeId),
		NetworkId:       c.Config.NetworkID,
		CommitteeId:     msg.CommitteeId,
	}, v, qc, senderID); err != nil {
		handleErr(err)
		return
	}
	// validates the 'block' and 'block hash' of the proposal
	stillCatchingUp, err = plug.CheckPeerQC(msg.MaxHeight, qc)
	if err != nil {
		handleErr(err)
		return
	}
	// attempts to commit the QC to persistence of chain by playing it against the state machine
	if err = plug.CommitCertificate(qc); err != nil {
		handleErr(err)
		return
	}
	// increment the height of the consensus
	cons.Height = plug.Height() + 1
	return
}

// checkPeerQC() performs validity checks on a QuorumCertificate received from a peer
func (c *Controller) checkPeerQC(maxBlockSize int, view *lib.View, v lib.ValidatorSet, qc *lib.QuorumCertificate, senderID []byte) lib.ErrorI {
	isPartialQC, err := qc.Check(v, maxBlockSize, view, false)
	if err != nil {
		c.P2P.ChangeReputation(senderID, InvalidJustifyRep)
		return err
	}
	if isPartialQC {
		c.P2P.ChangeReputation(senderID, InvalidJustifyRep)
		return lib.ErrNoMaj23()
	}
	if qc.Results != nil {
		c.P2P.ChangeReputation(senderID, InvalidMsgRep)
		return lib.ErrNonNilCertResults()
	}
	if qc.Header.Phase != lib.Phase_PRECOMMIT_VOTE {
		c.P2P.ChangeReputation(senderID, InvalidMsgRep)
		return lib.ErrWrongPhase()
	}
	// enforce the target height
	if qc.Header.Height != view.Height {
		c.P2P.ChangeReputation(senderID, InvalidMsgRep)
		return lib.ErrWrongHeight()
	}
	// enforce the last saved committee height as valid
	// NOTE: historical committees are accepted up to the last saved height in the state
	// else there's a potential for a long-range attack
	if qc.Header.CommitteeHeight < view.CommitteeHeight {
		c.P2P.ChangeReputation(senderID, InvalidJustifyRep)
		return lib.ErrInvalidQCCommitteeHeight()
	}
	return nil
}

// pollMaxHeight() polls all peers for their local MaxHeight and totalVDFIterations for a specific committeeID
// NOTE: unlike other P2P transmissions - SendCertReqMsg enforces a minimum reputation on `mustConnects`
// to ensure a byzantine validator cannot cause syncing issues above max_height
func (c *Controller) pollMaxHeight(committeeID uint64, backoff int) (maxHeight, minimumVDFIterations uint64, syncingPeers []string) {
	// empty inbox to start fresh
	c.emptyInbox(Certificate)
	// ask all peers
	c.log.Info("Polling all peers for max height")
	syncingPeers = make([]string, 0)
	// ask only for MaxHeight not the actual QC
	c.SendCertReqMsg(committeeID, true)
	for {
		c.log.Debug("Waiting for peer max heights")
		select {
		case m := <-c.P2P.Inbox(Certificate):
			response, ok := m.Message.(*lib.BlockResponseMessage)
			if !ok {
				c.P2P.ChangeReputation(m.Sender.Address.PublicKey, InvalidMsgRep)
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
		case <-time.After(PollMaxHeightTimeoutS * time.Second * time.Duration(backoff)):
			if maxHeight == 0 { // genesis file is 0 and first height is 1
				c.log.Warn("No heights received from peers. Trying again")
				return c.pollMaxHeight(committeeID, backoff+1)
			}
			c.log.Debugf("Waiting peer max height is %d", maxHeight)
			return
		}
	}
}

// singleNodeNetwork() returns true if there are no other participants in the committee besides self
func (c *Controller) singleNodeNetwork(committeeID uint64) bool {
	valSet, err := c.LoadCommittee(committeeID, c.FSM.Height())
	if err != nil {
		c.log.Error(err.Error())
		return false
	}
	return len(valSet.ValidatorSet.ValidatorSet) == 1 &&
		bytes.Equal(valSet.ValidatorSet.ValidatorSet[0].PublicKey, c.PublicKey)
}

// isAnySyncing() returns if any committee is currently syncing
func (c *Controller) isAnySyncing() bool {
	for _, b := range c.isSyncing {
		if b.Load() == true {
			return true
		}
	}
	return false
}

// emptyInbox() discards all unread messages for a specific topic
func (c *Controller) emptyInbox(topic lib.Topic) {
	for len(c.P2P.Inbox(topic)) > 0 {
		<-c.P2P.Inbox(topic)
	}
}

// syncingDone() checks if the syncing loop may complete for a specific committeeID
func (c *Controller) syncingDone(committeeID, maxHeight, minVDFIterations uint64) bool {
	plug, _, err := c.GetPluginAndConsensus(committeeID)
	if err != nil {
		c.log.Error(err.Error())
	}
	if plug.Height() >= maxHeight {
		// ensure node did not lie about VDF iterations in their chain
		if plug.TotalVDFIterations() < minVDFIterations {
			c.log.Fatalf("VDFIterations error: localVDFIterations: %d, minimumVDFIterations: %d", plug.TotalVDFIterations(), minVDFIterations)
		}
		return true
	}
	return false
}

// finishSyncing() is called when the syncing loop is completed for a specific committeeID
func (c *Controller) finishSyncing(committeeID uint64) {
	pluginHeight, err := c.GetHeight(committeeID)
	if err != nil {
		c.log.Error(err.Error())
	}
	c.Consensus[committeeID].ResetBFTChan() <- bft.ResetBFT{
		ProcessTime: time.Since(c.LoadLastCommitTime(committeeID, pluginHeight)),
	}
	c.isSyncing[committeeID].Store(false)
	go c.ListenForCertificate()
}

const (
	CertificateRequest = lib.Topic_CERTIFICATE_REQUEST
	Certificate        = lib.Topic_CERTIFICATE
	Tx                 = lib.Topic_TX
	Cons               = lib.Topic_CONSENSUS
)
