package consensus

import (
	"bytes"
	"fmt"
	lib "github.com/ginchuco/ginchu/types"
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
	SyncTimeoutS = 5

	PeerGoodBlock               = 3
	PeerGoodTx                  = 3
	PeerTimeoutSlash            = -1
	PeerUnexpectedBlockSlash    = -1
	PeerInvalidBlockHeightSlash = -1
	PeerInvalidTxSlash          = -1
	PeerNotValidator            = -3
	PeerInvalidMessageSlash     = -3
	PeerInvalidBlockSlash       = -3
	PeerInvalidJustifySlash     = -3
)

func (cs *ConsensusState) Sync() {
	app, p2p := cs.App, cs.P2P
	receiveChannel := p2p.ReceiveChannel(lib.Topic_BLOCK)
	for {
		height, maxHeight, bb := app.LatestHeight(), p2p.GetMaxPeerHeight(), app.GetBeginBlockParams()
		vs, err := lib.NewValidatorSet(bb.ValidatorSet)
		if err != nil {
			cs.log.Fatalf("NewValidatorSet() failed with err: %s", err)
		}
		if height >= maxHeight {
			return
		}
		peer := p2p.GetPeerWithHeight(height + 1)
		if peer == nil {
			// TODO handle
			panic(fmt.Sprintf("no peer with height %d found", height+1))
		}
		if err = p2p.SendTo(peer.PublicKey, lib.Topic_BLOCK_REQUEST, &lib.BlockRequestMessage{Height: height + 1}); err != nil {
			cs.log.Error(err.Error())
			continue
		}
		select {
		case msg := <-receiveChannel:
			if !bytes.Equal(msg.Sender.PublicKey, peer.PublicKey) {
				p2p.ChangeReputation(msg.Sender.PublicKey, PeerUnexpectedBlockSlash)
				continue
			}
			var qc *QuorumCertificate
			qc, err = cs.ValidatePeerBlock(height, msg, vs)
			if err != nil {
				continue
			}
			if err = app.CommitBlock(qc); err != nil {
				cs.log.Fatalf("unable to commit synced block at height %d: %s", height+1, err.Error())
			}
			p2p.ChangeReputation(peer.PublicKey, PeerGoodBlock)
		case <-time.After(SyncTimeoutS * time.Second):
			p2p.ChangeReputation(peer.PublicKey, PeerTimeoutSlash)
			continue
		}
	}
}

func (cs *ConsensusState) ListenForNewBlock() {
	app, p2p := cs.App, cs.P2P
	for msg := range p2p.ReceiveChannel(lib.Topic_BLOCK) {
		qc, err := cs.ValidatePeerBlock(cs.Height, msg, cs.ValidatorSet)
		if err != nil {
			cs.P2P.ChangeReputation(msg.Sender.PublicKey, PeerInvalidBlockSlash)
			continue
		}
		if err = app.CommitBlock(qc); err != nil {
			cs.log.Fatalf("unable to commit block at height %d: %s", qc.Header.Height, err.Error())
		}
	}
}

func (cs *ConsensusState) ListenForNewTx() {
	app, p2p := cs.App, cs.P2P
	for msg := range p2p.ReceiveChannel(lib.Topic_TX) {
		senderID := msg.Sender.PublicKey
		txMsg, ok := msg.Message.(*lib.TxMessage)
		if !ok {
			p2p.ChangeReputation(senderID, PeerInvalidMessageSlash)
			continue
		}
		if txMsg == nil {
			p2p.ChangeReputation(senderID, PeerInvalidMessageSlash)
			continue
		}
		if err := app.HandleTransaction(txMsg.Tx); err != nil {
			p2p.ChangeReputation(senderID, PeerInvalidTxSlash)
			continue
		}
		p2p.ChangeReputation(senderID, PeerGoodTx)
	}
}

func (cs *ConsensusState) ListenForNewBlockRequests() {
	app, p2p := cs.App, cs.P2P
	for msg := range p2p.ReceiveChannel(lib.Topic_BLOCK_REQUEST) {
		request, ok := msg.Message.(*lib.BlockRequestMessage)
		if !ok {
			p2p.ChangeReputation(msg.Sender.PublicKey, PeerInvalidMessageSlash)
			continue
		}
		blocAndCertificate, err := app.GetBlockAndCertificate(request.Height)
		if err != nil {
			cs.log.Error(err.Error())
			continue
		}
		if err = p2p.SendTo(msg.Sender.PublicKey, lib.Topic_BLOCK, &lib.BlockResponseMessage{
			BlockAndCertificate: blocAndCertificate,
		}); err != nil {
			cs.log.Error(err.Error())
			continue
		}
	}
}

func (cs *ConsensusState) ListenForValidatorMessages() {
	p2p := cs.P2P
	for msg := range p2p.ReceiveChannel(lib.Topic_CONSENSUS) {
		if !msg.Sender.IsValidator {
			p2p.ChangeReputation(msg.Sender.PublicKey, PeerNotValidator)
			continue
		}
		if err := cs.HandleMessage(msg.Message); err != nil {
			p2p.ChangeReputation(msg.Sender.PublicKey, PeerInvalidMessageSlash)
			cs.log.Error(err.Error())
			continue
		}
	}
}

func (cs *ConsensusState) ValidatePeerBlock(height uint64, msg *lib.MessageWrapper, vs lib.ValidatorSetWrapper) (*QuorumCertificate, lib.ErrorI) {
	senderID := msg.Sender.PublicKey
	response, ok := msg.Message.(*lib.BlockResponseMessage)
	if !ok {
		cs.P2P.ChangeReputation(senderID, PeerInvalidMessageSlash)
		return nil, ErrUnknownConsensusMsg(msg.Message)
	}
	qc := response.BlockAndCertificate
	p2p := cs.P2P
	isPartialQC, err := qc.Check(&lib.View{Height: height + 1}, vs)
	if err != nil {
		p2p.ChangeReputation(senderID, PeerInvalidJustifySlash)
		return nil, err
	}
	if isPartialQC {
		p2p.ChangeReputation(senderID, PeerInvalidJustifySlash)
		return nil, lib.ErrNoMaj23()
	}
	if qc.Header.Phase != lib.Phase_PRECOMMIT_VOTE {
		p2p.ChangeReputation(senderID, PeerInvalidJustifySlash)
		return nil, lib.ErrWrongPhase()
	}
	block := qc.Block
	if err = block.Check(); err != nil {
		p2p.ChangeReputation(senderID, PeerInvalidBlockSlash)
		return nil, err
	}
	if block.BlockHeader.Height != height+1 {
		p2p.ChangeReputation(senderID, PeerInvalidBlockHeightSlash)
		return nil, lib.ErrWrongHeight()
	}
	return qc, nil
}
