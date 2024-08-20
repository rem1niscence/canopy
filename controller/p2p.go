package controller

import (
	"bytes"
	"fmt"
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"time"
)

/*
	Syncing Process:

	1) Get the height and begin block params from the state_machine
	2) Get peer max_height from P2P
	3) Ask peers for a block at a time
	4) Validate each block by checking if block was signed by +2/3 maj (COMMIT message)
	5) Commit block against the state machine, if error fatal
	5) Do this until reach the max-peer-height
	6) Stay on top by listening to incoming block messages
*/

const (
	SyncTimeoutS         = 5
	MaxBlockReqPerWindow = 5
	BlockReqWindowS      = 10

	GoodBlockRep        = 3
	GoodTxRep           = 3
	TimeoutRep          = -1
	UnexpectedBlockRep  = -1
	InvalidTxRep        = -1
	NotValRep           = -3
	InvalidMsgRep       = -3
	InvalidBlockRep     = -3
	InvalidJustifyRep   = -3
	BlockReqExceededRep = -3
)

func (c *Controller) SingleNodeNetwork(committeeID uint64) bool {
	valSet, err := c.LoadCommittee(committeeID, c.FSM.Height())
	if err != nil {
		c.log.Error(err.Error())
		return false
	}
	return len(valSet.ValidatorSet.ValidatorSet) == 1 &&
		bytes.Equal(valSet.ValidatorSet.ValidatorSet[0].PublicKey, c.PublicKey)
}

func (c *Controller) Sync(committeeID uint64) {
	var reqRecipient []byte
	c.syncing.Store(true)
	c.log.Infof("Sync started 🔄 for committee %d", committeeID)
	if c.SingleNodeNetwork(committeeID) {
		if c.syncingDone(committeeID, 0) {
			return
		}
	}
	maxHeight, maxHeights := c.pollMaxHeight(committeeID, 1)
	for {
		if c.syncingDone(committeeID, maxHeight) {
			return
		}
		height := c.FSM.Height()
		for p, m := range maxHeights {
			if m > height+1 {
				reqRecipient, _ = lib.StringToBytes(p)
			}
		}
		c.SendBlockReqMsg(committeeID, false, reqRecipient)
		select {
		case msg := <-c.P2P.Inbox(Block):
			responder := msg.Sender.Address.PublicKey
			if !bytes.Equal(responder, reqRecipient) {
				c.P2P.ChangeReputation(responder, UnexpectedBlockRep)
				continue
			}
			blkResponseMsg, ok := msg.Message.(*lib.BlockResponseMessage)
			if !ok {
				c.P2P.ChangeReputation(msg.Sender.Address.PublicKey, InvalidBlockRep)
				return
			}
			if _, _, err := c.handlePeerCert(msg.Sender.Address.PublicKey, blkResponseMsg); err != nil {
				continue
			}
			if blkResponseMsg.MaxHeight > maxHeight {
				maxHeight = blkResponseMsg.MaxHeight
			}
			c.P2P.ChangeReputation(responder, GoodBlockRep)
		case <-time.After(SyncTimeoutS * time.Second):
			c.P2P.ChangeReputation(reqRecipient, TimeoutRep)
			maxHeight, maxHeights = c.pollMaxHeight(committeeID, 1)
			continue
		}
	}
}

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

func (c *Controller) SendCertMsg(committeeID uint64, qc *lib.QuorumCertificate) {
	c.log.Debugf("Gossiping certificate: %s", lib.BytesToString(qc.ProposalHash))
	blockMessage := &lib.BlockResponseMessage{
		CommitteeId:         committeeID,
		MaxHeight:           c.FSM.Height(),
		BlockAndCertificate: qc,
	}
	if err := c.P2P.SendToChainPeers(committeeID, Block, blockMessage); err != nil {
		c.log.Errorf("unable to gossip block with err: %s", err.Error())
	}
	if err := c.P2P.SelfSend(c.PublicKey, Block, blockMessage); err != nil {
		c.log.Errorf("unable to self-send block with err: %s", err.Error())
	}
	c.log.Debugf("gossiping done")
}

func (c *Controller) SendBlockRespMsg(committeeID, maxHeight uint64, blockAndCert *lib.QuorumCertificate, toPubKey ...[]byte) {
	var err error
	if len(toPubKey) == 0 {
		if err = c.P2P.SendToChainPeers(committeeID, Block, &lib.BlockResponseMessage{
			CommitteeId:         committeeID,
			MaxHeight:           maxHeight,
			BlockAndCertificate: blockAndCert,
		}); err != nil {
			c.log.Error(err.Error())
		}
	} else {
		for _, pk := range toPubKey {
			if err = c.P2P.SendTo(pk, Block, &lib.BlockResponseMessage{
				CommitteeId:         committeeID,
				MaxHeight:           maxHeight,
				BlockAndCertificate: blockAndCert,
			}); err != nil {
				c.log.Error(err.Error())
			}
		}
	}
}

func (c *Controller) SendBlockReqMsg(committeeID uint64, heightOnly bool, toPubKey ...[]byte) {
	var err error
	if len(toPubKey) == 0 {
		if err = c.P2P.SendToChainPeers(committeeID, BlockRequest, &lib.BlockRequestMessage{
			CommitteeId: committeeID,
			Height:      c.FSM.Height(),
			HeightOnly:  heightOnly,
		}); err != nil {
			c.log.Error(err.Error())
		}
	} else {
		for _, pk := range toPubKey {
			if err = c.P2P.SendTo(pk, BlockRequest, &lib.BlockRequestMessage{
				CommitteeId: committeeID,
				Height:      c.FSM.Height(),
				HeightOnly:  heightOnly,
			}); err != nil {
				c.log.Error(err.Error())
			}
		}
	}
}

func (c *Controller) SendProposalTx(committeeID uint64, qc *lib.QuorumCertificate) {
	c.log.Debugf("Sending proposal txn for: %s", lib.BytesToString(qc.ProposalHash))
	tx, err := types.NewProposalTx(c.PrivateKey, qc, 0)
	if err != nil {
		c.log.Errorf("Creating auto-proposal-txn failed with err: %s", err.Error())
		return
	}
	bz, err := tx.GetBytes()
	if err != nil {
		c.log.Errorf("Marshalling auto-proposal-txn failed with err: %s", err.Error())
		return
	}
	if err = c.SendTxMsg(committeeID, bz); err != nil {
		c.log.Errorf("Gossiping auto-proposal-txn failed with err: %s", err.Error())
		return
	}
}

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

func (c *Controller) ListenForBlock() {
	c.log.Debug("Listening for inbound blocks")
	cache := lib.NewMessageCache()
	for msg := range c.P2P.Inbox(Block) {
		ret := false
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
				ret = true
				return
			}
			if err != nil {
				c.log.Error(err.Error())
				c.P2P.ChangeReputation(msg.Sender.Address.PublicKey, InvalidBlockRep)
				return
			}
			c.Consensus[blkResponseMsg.CommitteeId].ResetBFTChan() <- time.Since(startTime)
			c.UpdateP2PMustConnect()
			c.SendCertMsg(blkResponseMsg.CommitteeId, qc)
		}()
		if ret {
			return
		}
	}
}

func (c *Controller) ListenForConsensus() {
	for msg := range c.P2P.Inbox(Cons) {
		if c.syncing.Load() {
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

func (c *Controller) ListenForTx() {
	cache := lib.NewMessageCache()
	for msg := range c.P2P.Inbox(Tx) {
		func() {
			c.Lock()
			defer c.Unlock()
			if !c.syncing.Load() {
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

func (c *Controller) ListenForBlockReq() {
	l := lib.NewLimiter(MaxBlockReqPerWindow, c.P2P.MaxPossiblePeers()*MaxBlockReqPerWindow, BlockReqWindowS)
	for {
		select {
		case msg := <-c.P2P.Inbox(BlockRequest):
			func() {
				c.Lock()
				defer c.Unlock()
				if c.syncing.Load() {
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
				c.SendBlockRespMsg(request.CommitteeId, plug.Height(), cert, senderID)
			}()
		case <-l.C():
			l.Reset()
		}
	}
}

func (c *Controller) StartListeners() {
	c.log.Debug("Listening for inbound txs, block requests, and consensus messages")
	go c.ListenForBlockReq()
	go c.ListenForConsensus()
	go c.ListenForTx()
}

func (c *Controller) handlePeerCert(senderID []byte, msg *lib.BlockResponseMessage) (qc *lib.QuorumCertificate, outOfSync bool, err lib.ErrorI) {
	qc, consensus := msg.BlockAndCertificate, c.Consensus[msg.CommitteeId]
	height, v := c.FSM.Height(), consensus.ValidatorSet
	handleErr := func(err error) {
		c.P2P.ChangeReputation(senderID, InvalidBlockRep)
		c.log.Error(err.Error())
	}
	if err = c.checkPeerQC(height, v, senderID, msg.BlockAndCertificate); err != nil {
		handleErr(err)
		return
	}
	if err = qc.Proposal.CheckBasic(msg.CommitteeId, height); err != nil {
		handleErr(err)
		return
	}
	plug, cons, e := c.GetPluginAndConsensus(msg.CommitteeId)
	if e != nil {
		handleErr(e)
		return
	}
	outOfSync, err = plug.CheckPeerProposal(msg.MaxHeight, qc.Proposal)
	if err != nil {
		handleErr(err)
		return
	}
	if err = plug.CommitCertificate(qc); err != nil {
		handleErr(err)
		return
	}
	cons.Height = plug.Height() + 1
	return
}

func (c *Controller) checkPeerQC(height uint64, v lib.ValidatorSet, senderID []byte, qc *lib.QuorumCertificate) lib.ErrorI {
	isPartialQC, err := qc.Check(v, height)
	if err != nil {
		c.P2P.ChangeReputation(senderID, InvalidJustifyRep)
		return err
	}
	if isPartialQC {
		c.P2P.ChangeReputation(senderID, InvalidJustifyRep)
		return lib.ErrNoMaj23()
	}
	if qc.Header.Phase != lib.Phase_PRECOMMIT_VOTE {
		c.P2P.ChangeReputation(senderID, InvalidJustifyRep)
		return lib.ErrWrongPhase()
	}
	return nil
}

func (c *Controller) UpdateP2PMustConnect() {
	m, s := make(map[string]*lib.PeerAddress), make([]*lib.PeerAddress, 0)
	validators, e := c.FSM.GetConsensusValidators()
	if e != nil {
		c.log.Errorf("unable to get must connect peers from consensus validators with error %s", e.Error())
		return
	}
	selfId, selfIsValidator := c.PublicKey, false
	for _, val := range validators.ValidatorSet {
		if bytes.Equal(selfId, val.PublicKey) {
			selfIsValidator = true
			continue
		}
		m[lib.BytesToString(val.PublicKey)] = &lib.PeerAddress{
			PublicKey:  val.PublicKey,
			NetAddress: val.NetAddress,
		}
	}
	if !selfIsValidator {
		m = make(map[string]*lib.PeerAddress)
	}
	for committeeID := range c.Consensus {
		committee, err := c.FSM.GetCommittee(committeeID)
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

func (c *Controller) pollMaxHeight(committeeID uint64, backoff int) (maxHeight uint64, maxHeights map[string]uint64) {
	c.log.Info("Polling all peers for max height")
	maxHeights = make(map[string]uint64)
	c.SendBlockReqMsg(committeeID, true)
	for {
		c.log.Debug("Waiting for peer max heights")
		select {
		case m := <-c.P2P.Inbox(Block):
			response, ok := m.Message.(*lib.BlockResponseMessage)
			if !ok {
				c.P2P.ChangeReputation(m.Sender.Address.PublicKey, InvalidMsgRep)
				continue
			}
			if response.MaxHeight > maxHeight {
				maxHeight = response.MaxHeight
			}
			maxHeights[lib.BytesToString(m.Sender.Address.PublicKey)] = response.MaxHeight
		case <-time.After(SyncTimeoutS * time.Second * time.Duration(backoff)):
			c.log.Warn("No heights received from peers. Trying again")
			if maxHeight == 0 { // genesis file is 0 and first height is 1
				return c.pollMaxHeight(committeeID, backoff+1)
			}
			return
		}
	}
}

func (c *Controller) syncingDone(committeeID, maxHeight uint64) bool {
	pluginHeight, err := c.GetHeight(committeeID)
	if err != nil {
		c.log.Error(err.Error())
	}
	if pluginHeight >= maxHeight {
		c.Consensus[committeeID].ResetBFTChan() <- time.Since(c.LoadLastCommitTime(committeeID, pluginHeight))
		c.syncing.Store(false)
		go c.ListenForBlock()
		return true
	}
	return false
}

const (
	BlockRequest = lib.Topic_BLOCK_REQUEST
	Block        = lib.Topic_BLOCK
	Tx           = lib.Topic_TX
	Cons         = lib.Topic_CONSENSUS
)
