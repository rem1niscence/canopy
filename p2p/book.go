package p2p

import (
	"bytes"
	"github.com/ginchuco/ginchu/lib"
	"math/rand"
	"sort"
	"sync"
	"time"
)

const (
	MaxFailedDialAttempts        = 5
	MaxPeerBookLen               = 50000
	MaxPeerBookRequestsPerWindow = 2
	PeerBookRequestWindowS       = 120
	PeerBookRequestTimeoutS      = 5
	CrawlAndCleanBookFrequency   = time.Hour

	GoodPeerBookRespRep   = 3
	PeerBookReqTimeoutRep = -1
	UnexpectedMsgRep      = -1
	InvalidMsgRep         = -3
	ExceedMaxPBReqRep     = -3
	ExceedMaxPBLenRep     = -3
)

type PeerBook struct {
	sync.RWMutex
	book     []*BookPeer
	bookSize int
	log      lib.LoggerI
}

func (p *P2P) StartPeerBookService() {
	go p.ListenForPeerBookRequests()
	go p.SendPeerBookRequests()
	go p.book.StartChurnManagement(p.DialAndDisconnect)
}

func (p *PeerBook) StartChurnManagement(dialAndDisconnect func(a *lib.PeerAddress) lib.ErrorI) {
	for {
		p.RLock()
		bookCopy := make([]*BookPeer, len(p.book))
		copy(bookCopy, p.book)
		p.RUnlock()
		for _, peer := range bookCopy {
			if err := dialAndDisconnect(peer.Address); err != nil {
				p.AddFailedDialAttempt(peer.Address.PublicKey)
			} else {
				p.ResetFailedDialAttempts(peer.Address.PublicKey)
			}
		}
		time.Sleep(CrawlAndCleanBookFrequency)
	}
}

func (p *PeerBook) GetRandom() *BookPeer {
	p.RLock()
	defer p.RUnlock()
	if p.bookSize == 0 {
		return nil
	}
	return p.book[rand.Intn(p.bookSize)]
}

func (p *PeerBook) GetAll() (res []*BookPeer) {
	p.RLock()
	defer p.RUnlock()
	res = append(res, p.book...)
	return
}

func (p *PeerBook) Add(peer *BookPeer) {
	p.Lock()
	defer p.Unlock()
	i, found := p.getIndex(peer.Address.PublicKey)
	if found {
		return
	}
	p.bookSize++
	p.book = append(p.book, new(BookPeer))
	copy(p.book[i+1:], p.book[i:])
	p.book[i] = peer
}

func (p *PeerBook) Has(publicKey []byte) bool {
	p.Lock()
	defer p.Unlock()
	_, found := p.getIndex(publicKey)
	return found
}

func (p *PeerBook) Remove(publicKey []byte) {
	p.Lock()
	defer p.Unlock()
	i, found := p.getIndex(publicKey)
	if !found {
		return
	}
	p.log.Debugf("Removing peer %s from PeerBook", lib.BytesToString(publicKey))
	p.delAtIndex(i)
}

func (p *PeerBook) ResetFailedDialAttempts(publicKey []byte) {
	p.Lock()
	defer p.Unlock()
	i, found := p.getIndex(publicKey)
	if !found {
		return
	}
	peer := p.book[i]
	peer.ConsecutiveFailedDial = 0
	p.book[i] = peer
}

func (p *PeerBook) AddFailedDialAttempt(publicKey []byte) {
	p.Lock()
	defer p.Unlock()
	i, found := p.getIndex(publicKey)
	if !found {
		return
	}
	peer := p.book[i]
	peer.ConsecutiveFailedDial++
	if peer.ConsecutiveFailedDial >= MaxFailedDialAttempts {
		p.log.Debugf("Removing peer %s from PeerBook after max failed dial", lib.BytesToString(publicKey))
		p.delAtIndex(i)
		return
	}
	p.book[i] = peer
}

func (p *P2P) SendPeerBookRequests() {
	var doSleep bool
	for {
		if doSleep {
			time.Sleep(MaxPeerBookRequestsPerWindow * time.Second)
		} else {
			doSleep = true
		}
		peerInfo, err := p.SendToRandPeer(lib.Topic_PEERS_REQUEST, &PeerBookRequestMessage{})
		if peerInfo == nil || err != nil {
			continue
		}
		p.log.Debugf("Sent peer book request to %s", lib.BytesToString(peerInfo.Address.PublicKey))
		select {
		case msg := <-p.ReceiveChannel(lib.Topic_PEERS_RESPONSE):
			p.log.Debugf("Received peer book response from %s", lib.BytesToString(msg.Sender.Address.PublicKey))
			senderID := msg.Sender.Address.PublicKey
			peerBookResponseMsg, ok := msg.Message.(*PeerBookResponseMessage)
			if !ok {
				p.ChangeReputation(senderID, InvalidMsgRep)
				continue
			}
			if !bytes.Equal(msg.Sender.Address.PublicKey, peerInfo.Address.PublicKey) {
				p.ChangeReputation(senderID, UnexpectedMsgRep)
				continue
			}
			if len(peerBookResponseMsg.Book) > MaxPeerBookLen {
				p.ChangeReputation(senderID, ExceedMaxPBLenRep)
				continue
			}
			for _, b := range peerBookResponseMsg.Book {
				p.book.Add(b)
			}
			p.ChangeReputation(senderID, GoodPeerBookRespRep)
		case <-time.After(PeerBookRequestTimeoutS):
			p.ChangeReputation(peerInfo.Address.PublicKey, PeerBookReqTimeoutRep)
			continue
		}
	}
}

func (p *P2P) ListenForPeerBookRequests() {
	l := lib.NewLimiter(MaxPeerBookRequestsPerWindow, p.MaxPossiblePeers()*MaxPeerBookRequestsPerWindow, PeerBookRequestWindowS)
	for {
		select {
		case msg := <-p.ReceiveChannel(lib.Topic_PEERS_REQUEST):
			p.log.Debugf("Received peer book request from %s", lib.BytesToString(msg.Sender.Address.PublicKey))
			senderID := msg.Sender.Address.PublicKey
			sender := lib.BytesToString(senderID)
			blocked, totalBlock := l.NewRequest(sender)
			if blocked {
				p.ChangeReputation(senderID, ExceedMaxPBReqRep)
				continue
			}
			if totalBlock {
				continue // dos defensive
			}
			if _, ok := msg.Message.(*PeerBookRequestMessage); !ok {
				p.ChangeReputation(senderID, InvalidMsgRep)
				continue
			}
			p.book.RLock()
			err := p.SendTo(senderID, lib.Topic_PEERS_RESPONSE, &PeerBookResponseMessage{
				Book: p.book.book,
			})
			p.book.RUnlock()
			if err != nil {
				p.log.Error(err.Error()) // log error
			}
		case <-l.C():
			l.Reset()
		}
	}
}

func (p *PeerBook) getIndex(publicKey []byte) (int, bool) {
	i := sort.Search(p.bookSize, func(i int) bool {
		return bytes.Compare(p.book[i].Address.PublicKey, publicKey) >= 0
	})
	if i != p.bookSize && bytes.Equal(p.book[i].Address.PublicKey, publicKey) {
		return i, true
	}
	return i, false
}

func (p *PeerBook) delAtIndex(i int) {
	p.bookSize--
	p.book = append(p.book[:i], p.book[i+1:]...)
}
