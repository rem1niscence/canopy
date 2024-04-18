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

func (s *State) Sync() {
	p2p := s.P2P
	receiveChannel := p2p.ReceiveChannel(lib.Topic_BLOCK)
	maxHeight := s.PollPeersMaxHeight(receiveChannel, 1)
	for {
		height, bb := s.FSM.Height(), s.FSM.BeginBlockParams
		vs, err := lib.NewValidatorSet(bb.ValidatorSet)
		if err != nil {
			s.log.Fatalf("NewValidatorSet() failed with err: %s", err)
		}
		if height >= maxHeight {
			return
		}
		peer, err := p2p.SendToPeer(lib.Topic_BLOCK_REQUEST, &lib.BlockRequestMessage{Height: height + 1})
		if err != nil {
			s.log.Error(err.Error())
			continue
		}
		select {
		case msg := <-receiveChannel:
			senderID := msg.Sender.Address.PublicKey
			if !bytes.Equal(senderID, peer.Address.PublicKey) {
				p2p.ChangeReputation(senderID, PeerUnexpectedBlockSlash)
				continue
			}
			var qc *QuorumCertificate
			maxHeight, qc, err = s.ValidatePeerBlock(height, msg, vs)
			if err != nil {
				continue
			}
			if err = s.CommitBlock(qc); err != nil {
				s.log.Fatalf("unable to commit synced block at height %d: %s", height+1, err.Error())
			}
			p2p.ChangeReputation(senderID, PeerGoodBlock)
		case <-time.After(SyncTimeoutS * time.Second):
			p2p.ChangeReputation(peer.Address.PublicKey, PeerTimeoutSlash)
			continue
		}
	}
}

func (s *State) PollPeersMaxHeight(receiveChan chan *lib.MessageWrapper, backoff int) (maxHeight uint64) {
	if err := s.P2P.SendToAll(lib.Topic_BLOCK_REQUEST, &lib.BlockRequestMessage{HeightOnly: true}); err != nil {
		panic(err)
	}
	for {
		select {
		case m := <-receiveChan:
			response, ok := m.Message.(*lib.BlockResponseMessage)
			if !ok {
				s.P2P.ChangeReputation(m.Sender.Address.PublicKey, PeerInvalidMessageSlash)
				continue
			}
			if response.MaxHeight > maxHeight {
				maxHeight = response.MaxHeight
			}
		case <-time.After(SyncTimeoutS * time.Second * time.Duration(backoff)):
			if maxHeight == 0 {
				return s.PollPeersMaxHeight(receiveChan, backoff+1)
			}
			return
		}
	}
}

func (s *State) ListenForNewBlock() {
	app, p2p := s, s.P2P
	cache := lib.NewMessageCache()
	for msg := range p2p.ReceiveChannel(lib.Topic_BLOCK) {
		if ok := cache.Add(msg); !ok {
			continue
		}
		_, qc, err := s.ValidatePeerBlock(s.Height, msg, s.ValidatorSet)
		if err != nil {
			s.P2P.ChangeReputation(msg.Sender.Address.PublicKey, PeerInvalidBlockSlash)
			continue
		}
		if err = app.CommitBlock(qc); err != nil {
			s.log.Fatalf("unable to commit block at height %d: %s", qc.Header.Height, err.Error())
		}
		if err = p2p.SendToAll(lib.Topic_BLOCK, msg.Message); err != nil {
			s.log.Error(fmt.Sprintf("unable to gossip block with err: %s", err.Error()))
		}
	}
}

func (s *State) ListenForNewTx() {
	p2p := s.P2P
	cache := lib.NewMessageCache()
	for msg := range p2p.ReceiveChannel(lib.Topic_TX) {
		if ok := cache.Add(msg); !ok {
			continue
		}
		senderID := msg.Sender.Address.PublicKey
		txMsg, ok := msg.Message.(*lib.TxMessage)
		if !ok {
			p2p.ChangeReputation(senderID, PeerInvalidMessageSlash)
			continue
		}
		if txMsg == nil {
			p2p.ChangeReputation(senderID, PeerInvalidMessageSlash)
			continue
		}
		if err := s.HandleTransaction(txMsg.Tx); err != nil {
			p2p.ChangeReputation(senderID, PeerInvalidTxSlash)
			continue
		}
		p2p.ChangeReputation(senderID, PeerGoodTx)
		if err := p2p.SendToAll(lib.Topic_TX, msg.Message); err != nil {
			s.log.Error(fmt.Sprintf("unable to gossip tx with err: %s", err.Error()))
		}
	}
}

func (s *State) ListenForNewBlockRequests() {
	p2p := s.P2P
	l := lib.NewLimiter(MaxBlockRequestsPerWindow, p2p.MaxPossiblePeers()*MaxBlockRequestsPerWindow, BlockRequestWindowS)
	for {
		select {
		case msg := <-p2p.ReceiveChannel(lib.Topic_BLOCK_REQUEST):
			senderID := msg.Sender.Address.PublicKey
			blocked, totalBlock := l.NewRequest(lib.BytesToString(senderID))
			if blocked {
				p2p.ChangeReputation(senderID, PeerBlockRequestsExceededSlash)
				continue
			}
			if totalBlock {
				continue // dos defense
			}
			request, ok := msg.Message.(*lib.BlockRequestMessage)
			if !ok {
				p2p.ChangeReputation(senderID, PeerInvalidMessageSlash)
				continue
			}
			blocAndCertificate, err := s.FSM.GetBlockAndCertificate(request.Height)
			if err != nil {
				s.log.Error(err.Error())
				continue
			}
			if err = p2p.SendTo(senderID, lib.Topic_BLOCK, &lib.BlockResponseMessage{
				MaxHeight:           s.FSM.Height(),
				BlockAndCertificate: blocAndCertificate,
			}); err != nil {
				s.log.Error(err.Error())
				continue
			}
		case <-l.C():
			l.Reset()
		}
	}
}

func (s *State) ListenForValidatorMessages() {
	p2p := s.P2P
	for msg := range p2p.ReceiveChannel(lib.Topic_CONSENSUS) {
		if !msg.Sender.IsValidator {
			p2p.ChangeReputation(msg.Sender.Address.PublicKey, PeerNotValidator)
			continue
		}
		if err := s.HandleMessage(msg.Message); err != nil {
			p2p.ChangeReputation(msg.Sender.Address.PublicKey, PeerInvalidMessageSlash)
			s.log.Error(err.Error())
			continue
		}
	}
}

func (s *State) ValidatePeerBlock(height uint64, m *lib.MessageWrapper, v lib.ValidatorSetWrapper) (uint64, *QuorumCertificate, lib.ErrorI) {
	senderID := m.Sender.Address.PublicKey
	response, ok := m.Message.(*lib.BlockResponseMessage)
	if !ok {
		s.P2P.ChangeReputation(senderID, PeerInvalidMessageSlash)
		return 0, nil, ErrUnknownConsensusMsg(m.Message)
	}
	qc := response.BlockAndCertificate
	p2p := s.P2P
	isPartialQC, err := qc.Check(&lib.View{Height: height + 1}, v)
	if err != nil {
		p2p.ChangeReputation(senderID, PeerInvalidJustifySlash)
		return 0, nil, err
	}
	if isPartialQC {
		p2p.ChangeReputation(senderID, PeerInvalidJustifySlash)
		return 0, nil, lib.ErrNoMaj23()
	}
	if qc.Header.Phase != lib.Phase_PRECOMMIT_VOTE {
		p2p.ChangeReputation(senderID, PeerInvalidJustifySlash)
		return 0, nil, lib.ErrWrongPhase()
	}
	block := qc.Block
	if err = block.Check(); err != nil {
		p2p.ChangeReputation(senderID, PeerInvalidBlockSlash)
		return 0, nil, err
	}
	if block.BlockHeader.Height != height+1 {
		p2p.ChangeReputation(senderID, PeerInvalidBlockHeightSlash)
		return 0, nil, lib.ErrWrongHeight()
	}
	if block.BlockHeader.Height > response.MaxHeight {
		p2p.ChangeReputation(senderID, PeerInvalidBlockHeightSlash)
		return 0, nil, lib.ErrWrongMaxHeight()
	}
	return response.MaxHeight, qc, nil
}
