package consensus

import (
	"bytes"
	"fmt"
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

func (c *Consensus) Sync() {
	var reqRecipient []byte
	c.syncing.Store(true)
	maxHeight, maxHeights := c.pollMaxHeight(1)
	for {
		if c.syncingDone(maxHeight) {
			return
		}
		for p, m := range maxHeights {
			if m > c.Height+1 {
				reqRecipient, _ = lib.StringToBytes(p)
			}
		}
		if err := c.P2P.SendTo(reqRecipient, BlockRequest, &lib.BlockRequestMessage{Height: c.Height + 1}); err != nil {
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
			m, qc, _, err := c.validatePeerBlock(c.Height, msg, c.ValidatorSet)
			if err != nil {
				continue
			}
			if m > maxHeight {
				maxHeight = m
			}
			if err = c.CommitBlock(qc); err != nil {
				c.log.Fatalf("unable to commit synced block at height %d: %s", c.Height+1, err.Error())
			}
			c.P2P.ChangeReputation(responder, GoodBlockRep)
		case <-time.After(SyncTimeoutS * time.Second):
			c.P2P.ChangeReputation(reqRecipient, TimeoutRep)
			maxHeight, maxHeights = c.pollMaxHeight(1)
			continue
		}
	}
}

func (c *Consensus) ListenForBlock() {
	cache := lib.NewMessageCache()
	for msg := range c.P2P.ReceiveChannel(Block) {
		startTime := time.Now()
		if ok := cache.Add(msg); !ok {
			break
		}
		c.Lock()
		_, qc, outOfSync, err := c.validatePeerBlock(c.Height, msg, c.ValidatorSet)
		if outOfSync {
			c.Unlock()
			go c.Sync()
			break
		}
		if err != nil {
			c.Unlock()
			c.P2P.ChangeReputation(msg.Sender.Address.PublicKey, InvalidBlockRep)
			break
		}
		if err = c.CommitBlock(qc); err != nil {
			c.log.Fatalf("unable to commit block at height %d: %s", qc.Header.Height, err.Error())
		}
		c.newBlock <- time.Since(startTime)
		c.notifyP2P(c.ValidatorSet.ValidatorSet)
		c.gossipBlock(qc)
	}
}

func (c *Consensus) ListenForConsensus() {
	for msg := range c.P2P.ReceiveChannel(Cons) {
		if c.syncing.Load() {
			continue
		}
		if !msg.Sender.IsValidator {
			c.P2P.ChangeReputation(msg.Sender.Address.PublicKey, NotValRep)
			continue
		}
		if err := c.HandleMessage(msg.Message); err != nil {
			c.P2P.ChangeReputation(msg.Sender.Address.PublicKey, InvalidMsgRep)
			continue
		}
	}
}

func (c *Consensus) ListenForTx() {
	cache := lib.NewMessageCache()
	for msg := range c.P2P.ReceiveChannel(Tx) {
		if !c.syncing.Load() {
			continue
		}
		if ok := cache.Add(msg); !ok {
			continue
		}
		senderID := msg.Sender.Address.PublicKey
		txMsg, ok := msg.Message.(*lib.TxMessage)
		if !ok {
			c.P2P.ChangeReputation(senderID, InvalidMsgRep)
			continue
		}
		if txMsg == nil {
			c.P2P.ChangeReputation(senderID, InvalidMsgRep)
			continue
		}
		if err := c.HandleTransaction(txMsg.Tx); err != nil {
			c.P2P.ChangeReputation(senderID, InvalidTxRep)
			continue
		}
		c.P2P.ChangeReputation(senderID, GoodTxRep)
		if err := c.P2P.SendToAll(Tx, msg.Message); err != nil {
			c.log.Error(fmt.Sprintf("unable to gossip tx with err: %s", err.Error()))
		}
	}
}

func (c *Consensus) ListenForBlockReq() {
	l := lib.NewLimiter(MaxBlockReqPerWindow, c.P2P.MaxPossiblePeers()*MaxBlockReqPerWindow, BlockReqWindowS)
	for {
		select {
		case msg := <-c.P2P.ReceiveChannel(BlockRequest):
			if c.syncing.Load() {
				continue
			}
			senderID := msg.Sender.Address.PublicKey
			blocked, allBlocked := l.NewRequest(lib.BytesToString(senderID))
			if blocked {
				c.P2P.ChangeReputation(senderID, BlockReqExceededRep)
				continue
			}
			if allBlocked {
				continue // dos defense
			}
			request, ok := msg.Message.(*lib.BlockRequestMessage)
			if !ok {
				c.P2P.ChangeReputation(senderID, InvalidMsgRep)
				continue
			}
			blocAndCertificate, err := c.FSM.LoadBlockAndQC(request.Height)
			if err != nil {
				c.log.Error(err.Error())
				continue
			}
			c.Lock()
			height := c.FSM.Height()
			c.Unlock()
			if err = c.P2P.SendTo(senderID, Block, &lib.BlockResponseMessage{
				MaxHeight:           height,
				BlockAndCertificate: blocAndCertificate,
			}); err != nil {
				c.log.Error(err.Error())
				continue
			}
		case <-l.C():
			l.Reset()
		}
	}
}

func (c *Consensus) StartListeners() {
	go c.ListenForBlockReq()
	go c.ListenForConsensus()
	go c.ListenForTx()
}

func (c *Consensus) NewTx(tx []byte) {
	if err := c.HandleTransaction(tx); err != nil {
		return
	}
	if err := c.P2P.SendToAll(Tx, &lib.TxMessage{Tx: tx}); err != nil {
		c.log.Error(fmt.Sprintf("unable to gossip tx with err: %s", err.Error()))
	}
}

func (c *Consensus) validatePeerBlock(height uint64, m *lib.MessageWrapper, v lib.ValidatorSet) (max uint64, qc *QC, outOfSync bool, err lib.ErrorI) {
	senderID := m.Sender.Address.PublicKey
	response, ok := m.Message.(*lib.BlockResponseMessage)
	if !ok {
		c.P2P.ChangeReputation(senderID, InvalidMsgRep)
		err = ErrUnknownConsensusMsg(m.Message)
		return
	}
	qc = response.BlockAndCertificate
	if err = c.checkPeerQC(height, v, senderID, response.BlockAndCertificate); err != nil {
		return
	}
	if outOfSync, err = c.checkPeerBlock(height, qc.Block, senderID); outOfSync || err != nil {
		return
	}
	if qc.Block.BlockHeader.Height > response.MaxHeight {
		c.P2P.ChangeReputation(senderID, InvalidBlockRep)
		err = lib.ErrWrongMaxHeight()
		return
	}
	hash, _ := qc.Block.BlockHeader.SetHash()
	if !bytes.Equal(qc.BlockHash, hash) {
		c.P2P.ChangeReputation(senderID, InvalidJustifyRep)
		err = ErrMismatchBlockHash()
		return
	}
	max = response.MaxHeight
	return
}

func (c *Consensus) checkPeerQC(height uint64, v lib.ValidatorSet, senderID []byte, qc *lib.QuorumCertificate) lib.ErrorI {
	isPartialQC, err := qc.Check(height, v)
	if err != nil {
		c.P2P.ChangeReputation(senderID, InvalidJustifyRep)
		return err
	}
	if isPartialQC {
		c.P2P.ChangeReputation(senderID, InvalidJustifyRep)
		return lib.ErrNoMaj23()
	}
	if qc.Header.Phase != PrecommitVote {
		c.P2P.ChangeReputation(senderID, InvalidJustifyRep)
		return lib.ErrWrongPhase()
	}
	return nil
}

func (c *Consensus) checkPeerBlock(height uint64, block *lib.Block, senderID []byte) (outOfSync bool, err lib.ErrorI) {
	if err = block.Check(); err != nil {
		c.P2P.ChangeReputation(senderID, InvalidBlockRep)
		return false, err
	}
	if height > block.BlockHeader.Height {
		c.P2P.ChangeReputation(senderID, InvalidBlockRep)
		return false, lib.ErrWrongHeight()
	}
	outOfSync = height != block.BlockHeader.Height
	return
}

func (c *Consensus) gossipBlock(blockAndCertificate *QC) {
	if err := c.P2P.SendToAll(Block, &lib.BlockResponseMessage{
		MaxHeight:           c.Height,
		BlockAndCertificate: blockAndCertificate,
	}); err != nil {
		c.log.Error(fmt.Sprintf("unable to gossip block with err: %s", err.Error()))
	}
}

func (c *Consensus) notifyP2P(nextValidatorSet *lib.ConsensusValidators) {
	p := []*lib.PeerAddress(nil)
	for _, v := range nextValidatorSet.ValidatorSet {
		p = append(p, &lib.PeerAddress{PublicKey: v.PublicKey, NetAddress: v.NetAddress})
	}
	c.P2P.ValidatorsReceiver() <- p
}

func (c *Consensus) pollMaxHeight(backoff int) (maxHeight uint64, maxHeights map[string]uint64) {
	maxHeights = make(map[string]uint64)
	if err := c.P2P.SendToAll(BlockRequest, &lib.BlockRequestMessage{HeightOnly: true}); err != nil {
		panic(err)
	}
	for {
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
			if maxHeight == 0 { // genesis file is 0 and first height is 1
				return c.pollMaxHeight(backoff + 1)
			}
			return
		}
	}
}

func (c *Consensus) syncingDone(maxHeight uint64) bool {
	if c.Height >= maxHeight {
		c.syncDone <- struct{}{}
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
