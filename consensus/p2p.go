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
	SyncTimeoutS              = 5
	MaxBlockRequestsPerWindow = 5
	BlockRequestWindowS       = 10

	PeerGoodBlock                  = 3
	PeerGoodTx                     = 3
	PeerTimeoutSlash               = -1
	PeerUnexpectedBlockSlash       = -1
	PeerInvalidBlockHeightSlash    = -1
	PeerInvalidTxSlash             = -1
	PeerNotValidator               = -3
	PeerInvalidMessageSlash        = -3
	PeerInvalidBlockSlash          = -3
	PeerInvalidJustifySlash        = -3
	PeerBlockRequestsExceededSlash = -3
)

func (c *Consensus) Sync() {
	var qc *QC
	syncDone, maxHeight := c.StartIgnoreListeners(), c.PollPeersMaxHeight(1)
	for {
		height, bb := c.FSM.Height(), c.FSM.BeginBlockParams
		vs, err := lib.NewValidatorSet(bb.ValidatorSet)
		if err != nil {
			c.log.Fatalf("NewValidatorSet() failed with err: %s", err)
		}
		if height >= maxHeight {
			syncDone <- struct{}{}
			go c.StartListeners()
			return
		}
		peer, err := c.P2P.SendToPeer(lib.Topic_BLOCK_REQUEST, &lib.BlockRequestMessage{Height: height + 1})
		if err != nil {
			continue
		}
		select {
		case msg := <-c.P2P.ReceiveChannel(lib.Topic_BLOCK):
			senderID := msg.Sender.Address.PublicKey
			if !bytes.Equal(senderID, peer.Address.PublicKey) {
				c.P2P.ChangeReputation(senderID, PeerUnexpectedBlockSlash)
				continue
			}
			maxHeight, qc, err = c.ValidatePeerBlock(height, msg, vs)
			if err != nil {
				continue
			}
			if err = c.CommitBlock(qc); err != nil {
				c.log.Fatalf("unable to commit synced block at height %d: %s", height+1, err.Error())
			}
			c.P2P.ChangeReputation(senderID, PeerGoodBlock)
		case <-time.After(SyncTimeoutS * time.Second):
			c.P2P.ChangeReputation(peer.Address.PublicKey, PeerTimeoutSlash)
			continue
		}
	}
}

func (c *Consensus) PollPeersMaxHeight(backoff int) (maxHeight uint64) {
	if err := c.P2P.SendToAll(lib.Topic_BLOCK_REQUEST, &lib.BlockRequestMessage{HeightOnly: true}); err != nil {
		panic(err)
	}
	for {
		select {
		case m := <-c.P2P.ReceiveChannel(lib.Topic_BLOCK):
			response, ok := m.Message.(*lib.BlockResponseMessage)
			if !ok {
				c.P2P.ChangeReputation(m.Sender.Address.PublicKey, PeerInvalidMessageSlash)
				continue
			}
			if response.MaxHeight > maxHeight {
				maxHeight = response.MaxHeight
			}
		case <-time.After(SyncTimeoutS * time.Second * time.Duration(backoff)):
			if maxHeight == 0 { // genesis file is 0 and first height is 1
				return c.PollPeersMaxHeight(backoff + 1)
			}
			return
		}
	}
}

func (c *Consensus) GossipBlock(blockAndCertificate *QC) {
	if err := c.P2P.SendToAll(lib.Topic_BLOCK, &lib.BlockResponseMessage{
		MaxHeight:           blockAndCertificate.Header.Height,
		BlockAndCertificate: blockAndCertificate,
	}); err != nil {
		c.log.Error(fmt.Sprintf("unable to gossip block with err: %s", err.Error()))
	}
}

func (c *Consensus) ListenForNewBlock() {
	app := c
	cache := lib.NewMessageCache()
	for msg := range c.P2P.ReceiveChannel(lib.Topic_BLOCK) {
		func() {
			c.Lock()
			defer c.Unlock()
			if ok := cache.Add(msg); !ok {
				return
			}
			_, qc, err := c.ValidatePeerBlock(c.Height, msg, c.ValidatorSet)
			if err != nil {
				c.P2P.ChangeReputation(msg.Sender.Address.PublicKey, PeerInvalidBlockSlash)
				return
			}
			if err = app.CommitBlock(qc); err != nil {
				c.log.Fatalf("unable to commit block at height %d: %s", qc.Header.Height, err.Error())
			}
			if err = c.P2P.SendToAll(lib.Topic_BLOCK, msg.Message); err != nil {
				c.log.Error(fmt.Sprintf("unable to gossip block with err: %s", err.Error()))
			}
		}()
	}
}

func (c *Consensus) GossipNewTx(tx []byte) {
	if err := c.HandleTransaction(tx); err != nil {
		return
	}
	if err := c.P2P.SendToAll(lib.Topic_TX, &lib.TxMessage{Tx: tx}); err != nil {
		c.log.Error(fmt.Sprintf("unable to gossip tx with err: %s", err.Error()))
	}
}

func (c *Consensus) ListenForNewTx() {
	cache := lib.NewMessageCache()
	for msg := range c.P2P.ReceiveChannel(lib.Topic_TX) {
		if ok := cache.Add(msg); !ok {
			continue
		}
		senderID := msg.Sender.Address.PublicKey
		txMsg, ok := msg.Message.(*lib.TxMessage)
		if !ok {
			c.P2P.ChangeReputation(senderID, PeerInvalidMessageSlash)
			continue
		}
		if txMsg == nil {
			c.P2P.ChangeReputation(senderID, PeerInvalidMessageSlash)
			continue
		}
		if err := c.HandleTransaction(txMsg.Tx); err != nil {
			c.P2P.ChangeReputation(senderID, PeerInvalidTxSlash)
			continue
		}
		c.P2P.ChangeReputation(senderID, PeerGoodTx)
		if err := c.P2P.SendToAll(lib.Topic_TX, msg.Message); err != nil {
			c.log.Error(fmt.Sprintf("unable to gossip tx with err: %s", err.Error()))
		}
	}
}

func (c *Consensus) ListenForNewBlockRequests() {
	l := lib.NewLimiter(MaxBlockRequestsPerWindow, c.P2P.MaxPossiblePeers()*MaxBlockRequestsPerWindow, BlockRequestWindowS)
	for {
		select {
		case msg := <-c.P2P.ReceiveChannel(lib.Topic_BLOCK_REQUEST):
			senderID := msg.Sender.Address.PublicKey
			blocked, allBlocked := l.NewRequest(lib.BytesToString(senderID))
			if blocked {
				c.P2P.ChangeReputation(senderID, PeerBlockRequestsExceededSlash)
				continue
			}
			if allBlocked {
				continue // dos defense
			}
			request, ok := msg.Message.(*lib.BlockRequestMessage)
			if !ok {
				c.P2P.ChangeReputation(senderID, PeerInvalidMessageSlash)
				continue
			}
			blocAndCertificate, err := c.FSM.GetBlockAndCertificate(request.Height)
			if err != nil {
				c.log.Error(err.Error())
				continue
			}
			c.Lock()
			height := c.FSM.Height()
			c.Unlock()
			if err = c.P2P.SendTo(senderID, lib.Topic_BLOCK, &lib.BlockResponseMessage{
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

func (c *Consensus) ListenForValidatorMessages() {
	for msg := range c.P2P.ReceiveChannel(lib.Topic_CONSENSUS) {
		if !msg.Sender.IsValidator {
			c.P2P.ChangeReputation(msg.Sender.Address.PublicKey, PeerNotValidator)
			continue
		}
		if err := c.HandleMessage(msg.Message); err != nil {
			c.P2P.ChangeReputation(msg.Sender.Address.PublicKey, PeerInvalidMessageSlash)
			continue
		}
	}
}

func (c *Consensus) ValidatePeerBlock(height uint64, m *lib.MessageWrapper, v lib.ValidatorSet) (uint64, *QC, lib.ErrorI) {
	senderID := m.Sender.Address.PublicKey
	response, ok := m.Message.(*lib.BlockResponseMessage)
	if !ok {
		c.P2P.ChangeReputation(senderID, PeerInvalidMessageSlash)
		return 0, nil, ErrUnknownConsensusMsg(m.Message)
	}
	qc := response.BlockAndCertificate
	isPartialQC, err := qc.Check(height, v)
	if err != nil {
		c.P2P.ChangeReputation(senderID, PeerInvalidJustifySlash)
		return 0, nil, err
	}
	if isPartialQC {
		c.P2P.ChangeReputation(senderID, PeerInvalidJustifySlash)
		return 0, nil, lib.ErrNoMaj23()
	}
	if qc.Header.Phase != lib.Phase_PRECOMMIT_VOTE {
		c.P2P.ChangeReputation(senderID, PeerInvalidJustifySlash)
		return 0, nil, lib.ErrWrongPhase()
	}
	block := qc.Block
	if err = block.Check(); err != nil {
		c.P2P.ChangeReputation(senderID, PeerInvalidBlockSlash)
		return 0, nil, err
	}
	if block.BlockHeader.Height != height+1 {
		c.P2P.ChangeReputation(senderID, PeerInvalidBlockHeightSlash)
		return 0, nil, lib.ErrWrongHeight()
	}
	if block.BlockHeader.Height > response.MaxHeight {
		c.P2P.ChangeReputation(senderID, PeerInvalidBlockHeightSlash)
		return 0, nil, lib.ErrWrongMaxHeight()
	}
	return response.MaxHeight, qc, nil
}

func (c *Consensus) NotifyP2P(nextValidatorSet *lib.ConsensusValidators) {
	var p []*lib.PeerAddress
	for _, v := range nextValidatorSet.ValidatorSet {
		p = append(p, &lib.PeerAddress{PublicKey: v.PublicKey, NetAddress: v.NetAddress})
	}
	c.P2P.ValidatorsReceiver() <- p
}

func (c *Consensus) StartIgnoreListeners() (stop chan struct{}) {
	stop = make(chan struct{})
	go func() {
		for {
			select {
			case <-c.P2P.ReceiveChannel(lib.Topic_TX):
			case <-c.P2P.ReceiveChannel(lib.Topic_BLOCK_REQUEST):
			case <-c.P2P.ReceiveChannel(lib.Topic_CONSENSUS):
			case <-stop:
				return
			}
		}
	}()
	return
}

func (c *Consensus) StartListeners() {
	go c.ListenForNewBlock()
	go c.ListenForNewBlockRequests()
	go c.ListenForValidatorMessages()
	go c.ListenForNewTx()
}
