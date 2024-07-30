package app

import (
	"bytes"
	"fmt"
	"github.com/ginchuco/ginchu/bft"
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

func (c *Controller) SingleNodeNetwork() bool {
	valSet := c.GetValSet()
	return len(valSet.ValidatorSet.ValidatorSet) == 1 &&
		bytes.Equal(valSet.ValidatorSet.ValidatorSet[0].PublicKey, c.PublicKey)
}

func (c *Controller) Sync() {
	var reqRecipient []byte
	c.syncing.Store(true)
	c.log.Info("Sync started 🔄")
	if c.SingleNodeNetwork() {
		if c.syncingDone(0) {
			return
		}
	}
	maxHeight, maxHeights := c.pollMaxHeight(1)
	for {
		if c.syncingDone(maxHeight) {
			return
		}
		height, valSet := c.Consensus.Height, c.Consensus.ValidatorSet
		for p, m := range maxHeights {
			if m > height+1 {
				reqRecipient, _ = lib.StringToBytes(p)
			}
		}
		if err := c.P2P.SendTo(reqRecipient, BlockRequest, &lib.BlockRequestMessage{Height: height + 1}); err != nil {
			maxHeight, maxHeights = c.pollMaxHeight(1)
			continue
		}
		select {
		case msg := <-c.P2P.ReceiveChannel(Block):
			responder := msg.Sender.Address.PublicKey
			if !bytes.Equal(responder, reqRecipient) {
				c.P2P.ChangeReputation(responder, UnexpectedBlockRep)
				continue
			}
			m, qc, _, err := c.validatePeerBlock(height, msg, valSet)
			if err != nil {
				continue
			}
			if m > maxHeight {
				maxHeight = m
			}
			if err = c.commitBlock(qc); err != nil {
				c.log.Fatalf("unable to commit synced block at height %d: %s", height+1, err.Error())
			}
			c.P2P.ChangeReputation(responder, GoodBlockRep)
		case <-time.After(SyncTimeoutS * time.Second):
			c.P2P.ChangeReputation(reqRecipient, TimeoutRep)
			maxHeight, maxHeights = c.pollMaxHeight(1)
			continue
		}
	}
}

func (c *Controller) SendToReplicas(msg lib.Signable) {
	if err := msg.Sign(c.PrivateKey); err != nil {
		c.log.Error(err.Error())
		return
	}
	if err := c.P2P.SendToValidators(msg); err != nil {
		c.log.Error(err.Error())
		return
	}
	if err := c.P2P.SelfSend(c.PublicKey, Cons, msg); err != nil {
		c.log.Error(err.Error())
		return
	}
}

func (c *Controller) SendToProposer(msg lib.Signable) {
	if err := msg.Sign(c.PrivateKey); err != nil {
		c.log.Error(err.Error())
		return
	}
	if c.Consensus.SelfIsProposer() {
		if err := c.P2P.SelfSend(c.PublicKey, Cons, msg); err != nil {
			c.log.Error(err.Error())
		}
		return
	}
	if err := c.P2P.SendTo(c.Consensus.ProposerKey, Cons, msg); err != nil {
		c.log.Error(err.Error())
		return
	}
}

func (c *Controller) ListenForBlock() {
	c.log.Debug("Listening for inbound blocks")
	cache := lib.NewMessageCache()
	for msg := range c.P2P.ReceiveChannel(Block) {
		ret := false
		func() {
			c.Lock()
			defer c.Unlock()
			startTime := time.Now()
			if ok := cache.Add(msg); !ok {
				return
			}
			c.log.Infof("Received new block from %s 📥", lib.BzToTruncStr(msg.Sender.Address.PublicKey))
			height, valSet := c.Consensus.Height, c.Consensus.ValidatorSet
			_, qc, outOfSync, err := c.validatePeerBlock(height, msg, valSet)
			if outOfSync {
				c.log.Warnf("Node fell out of sync")
				go c.Sync()
				ret = true
				return
			}
			if err != nil {
				c.log.Error(err.Error())
				c.P2P.ChangeReputation(msg.Sender.Address.PublicKey, InvalidBlockRep)
				return
			}
			if err = c.commitBlock(qc); err != nil {
				c.log.Fatalf("unable to commit block at height %d: %s", qc.Header.Height, err.Error())
			}
			c.resetBFT <- time.Since(startTime)
			c.notifyP2P(c.Consensus.ValidatorSet.ValidatorSet)
			c.GossipCertificate(qc)
		}()
		if ret {
			return
		}
	}
}

func (c *Controller) ListenForConsensus() {
	for msg := range c.P2P.ReceiveChannel(Cons) {
		if c.syncing.Load() {
			continue
		}
		func() {
			c.Lock()
			defer c.Unlock()
			vs, e := c.FSM.LoadValSet(c.Consensus.Height)
			if e != nil {
				c.log.Error(e.Error())
				return
			}
			if _, err := vs.GetValidator(msg.Sender.Address.PublicKey); err != nil {
				c.log.Error(err.Error())
				c.P2P.ChangeReputation(msg.Sender.Address.PublicKey, NotValRep)
				return
			}
			if err := c.Consensus.HandleMessage(msg.Message); err != nil {
				c.log.Error(err.Error())
				c.P2P.ChangeReputation(msg.Sender.Address.PublicKey, InvalidMsgRep)
				return
			}
		}()

	}
}

func (c *Controller) ListenForTx() {
	cache := lib.NewMessageCache()
	for msg := range c.P2P.ReceiveChannel(Tx) {
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
			if err := c.HandleTransaction(txMsg.Tx); err != nil {
				c.P2P.ChangeReputation(senderID, InvalidTxRep)
				return
			}
			c.P2P.ChangeReputation(senderID, GoodTxRep)
			if err := c.P2P.SendToAll(Tx, msg.Message); err != nil {
				c.log.Error(fmt.Sprintf("unable to gossip tx with err: %s", err.Error()))
			}
		}()

	}
}

func (c *Controller) ListenForBlockReq() {
	l := lib.NewLimiter(MaxBlockReqPerWindow, c.P2P.MaxPossiblePeers()*MaxBlockReqPerWindow, BlockReqWindowS)
	for {
		select {
		case msg := <-c.P2P.ReceiveChannel(BlockRequest):
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
				blocAndCertificate, err := c.FSM.LoadBlockAndQC(request.Height)
				if err != nil {
					c.log.Error(err.Error())
					return
				}
				if err = c.P2P.SendTo(senderID, Block, &lib.BlockResponseMessage{
					MaxHeight:           c.Consensus.Height - 1,
					BlockAndCertificate: blocAndCertificate,
				}); err != nil {
					c.log.Error(err.Error())
					return
				}
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

func (c *Controller) NewTx(tx []byte) lib.ErrorI {
	if err := c.HandleTransaction(tx); err != nil {
		return err
	}
	if err := c.P2P.SendToAll(Tx, &lib.TxMessage{Tx: tx}); err != nil {
		c.log.Error(fmt.Sprintf("unable to gossip tx with err: %s", err.Error()))
	}
	return nil
}

func (c *Controller) validatePeerBlock(height uint64, m *lib.MessageWrapper, v lib.ValidatorSet) (
	max uint64, qc *lib.QuorumCertificate, outOfSync bool, err lib.ErrorI) {
	senderID := m.Sender.Address.PublicKey
	response, ok := m.Message.(*lib.BlockResponseMessage)
	if !ok {
		c.P2P.ChangeReputation(senderID, InvalidMsgRep)
		err = bft.ErrUnknownConsensusMsg(m.Message)
		return
	}
	qc = response.BlockAndCertificate
	if err = c.checkPeerQC(height, v, senderID, response.BlockAndCertificate); err != nil {
		return
	}
	var block *lib.Block
	block, outOfSync, err = c.checkPeerBlock(height, qc.Proposal, senderID)
	if outOfSync || err != nil {
		return
	}
	if block.BlockHeader.Height > response.MaxHeight {
		c.P2P.ChangeReputation(senderID, InvalidBlockRep)
		err = lib.ErrWrongMaxHeight()
		return
	}
	hash, _ := block.BlockHeader.SetHash()
	if !bytes.Equal(qc.ProposalHash, hash) {
		c.P2P.ChangeReputation(senderID, InvalidJustifyRep)
		err = bft.ErrMismatchProposalHash()
		return
	}
	max = response.MaxHeight
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

func (c *Controller) checkPeerBlock(height uint64, blk []byte, senderID []byte) (block *lib.Block, outOfSync bool, err lib.ErrorI) {
	block = new(lib.Block)
	if err = lib.Unmarshal(blk, block); err != nil {
		c.P2P.ChangeReputation(senderID, InvalidBlockRep)
		return nil, false, err
	}
	if err = block.Check(); err != nil {
		c.P2P.ChangeReputation(senderID, InvalidBlockRep)
		return nil, false, err
	}
	if height > block.BlockHeader.Height {
		c.P2P.ChangeReputation(senderID, InvalidBlockRep)
		return nil, false, lib.ErrWrongHeight()
	}
	outOfSync = height != block.BlockHeader.Height
	return
}

func (c *Controller) GossipCertificate(blockAndCertificate *lib.QuorumCertificate) {
	c.log.Debugf("Gossiping certificate: %s", lib.BytesToString(blockAndCertificate.ProposalHash))
	block := new(lib.Block)
	if err := lib.Unmarshal(blockAndCertificate.Proposal, block); err != nil {
		c.log.Errorf("unable to gossip block with err: %s", err.Error())
		return
	}
	blockMessage := &lib.BlockResponseMessage{
		MaxHeight:           block.BlockHeader.Height,
		BlockAndCertificate: blockAndCertificate,
	}
	if err := c.P2P.SendToAll(Block, blockMessage); err != nil {
		c.log.Errorf("unable to gossip block with err: %s", err.Error())
	}
	if err := c.P2P.SelfSend(c.PublicKey, Block, blockMessage); err != nil {
		c.log.Errorf("unable to self-send block with err: %s", err.Error())
	}
	c.log.Debugf("gossiping done")
}

func (c *Controller) notifyP2P(nextValidatorSet *lib.ConsensusValidators) {
	p := []*lib.PeerAddress(nil)
	for _, v := range nextValidatorSet.ValidatorSet {
		p = append(p, &lib.PeerAddress{PublicKey: v.PublicKey, NetAddress: v.NetAddress})
	}
	c.P2P.ValidatorsReceiver() <- p
}

func (c *Controller) pollMaxHeight(backoff int) (maxHeight uint64, maxHeights map[string]uint64) {
	c.log.Info("Polling all peers for max height")
	maxHeights = make(map[string]uint64)
	if err := c.P2P.SendToAll(BlockRequest, &lib.BlockRequestMessage{HeightOnly: true}); err != nil {
		panic(err)
	}
	for {
		c.log.Debug("Waiting for peer max heights")
		select {
		case m := <-c.P2P.ReceiveChannel(Block):
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
				return c.pollMaxHeight(backoff + 1)
			}
			return
		}
	}
}

func (c *Controller) syncingDone(maxHeight uint64) bool {
	if c.GetHeight() >= maxHeight {
		c.resetBFT <- time.Since(c.LoadLastCommitTime(maxHeight))
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
