package p2p

import (
	"bytes"
	"fmt"
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

	GoodPeerBookResponseBoost       = 3
	PeerBookRequestTimeoutSlash     = -1
	PeerUnexpectedMessageSlash      = -1
	PeerInvalidMessageSlash         = -3
	PeerMaxPeerBookRequestsExceeded = -3
	PeerMaxBookLenExceeded          = -3
)

type PeerBook struct {
	sync.RWMutex
	book     []*BookPeer
	bookSize int
}

func (p *PeerBook) StartChurnManagement(dialAndDisconnect func(a *lib.PeerAddress) lib.ErrorI) {
	for {
		time.Sleep(CrawlAndCleanBookFrequency)
		p.RLock()
		bookCopy := make([]*BookPeer, len(p.book))
		copy(bookCopy, p.book)
		p.RUnlock()
		for _, peer := range bookCopy {
			if err := dialAndDisconnect(peer.Address); err != nil {
				p.AddFailedDialAttempt(peer.Address.PublicKey)
			}
		}
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

func (p *PeerBook) Add(peer *BookPeer) {
	p.Lock()
	defer p.Unlock()
	i, found := p.getIndex(peer.Address.PublicKey)
	if found {
		return
	}
	p.bookSize++
	p.book = append(p.book, new(BookPeer))
	p.book[i] = peer
}

func (p *PeerBook) Remove(publicKey []byte) {
	p.Lock()
	defer p.Unlock()
	i, found := p.getIndex(publicKey)
	if !found {
		return
	}
	p.delAtIndex(i)
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
		peerInfo, err := p.SendToPeer(lib.Topic_PEERS_REQUEST, &PeerBookRequestMessage{})
		if err != nil {
			continue
		}
		select {
		case msg := <-p.ReceiveChannel(lib.Topic_PEERS_RESPONSE):
			senderID := msg.Sender.Address.PublicKey
			peerBookResponseMsg, ok := msg.Message.(*PeerBookResponseMessage)
			if !ok {
				p.ChangeReputation(senderID, PeerInvalidMessageSlash)
				continue
			}
			if !bytes.Equal(msg.Sender.Address.PublicKey, peerInfo.Address.PublicKey) {
				p.ChangeReputation(senderID, PeerUnexpectedMessageSlash)
				continue
			}
			if len(peerBookResponseMsg.Book) > MaxPeerBookLen {
				p.ChangeReputation(senderID, PeerMaxBookLenExceeded)
				continue
			}
			p.book.Lock()
			for _, b := range peerBookResponseMsg.Book {
				p.book.Add(b)
			}
			p.book.Unlock()
			p.ChangeReputation(senderID, GoodPeerBookResponseBoost)
		case <-time.After(PeerBookRequestTimeoutS):
			p.ChangeReputation(peerInfo.Address.PublicKey, PeerBookRequestTimeoutSlash)
			continue
		}
	}
}

func (p *P2P) ListenForPeerBookRequests() {
	l := lib.NewLimiter(MaxPeerBookRequestsPerWindow, p.MaxPossiblePeers()*MaxPeerBookRequestsPerWindow, PeerBookRequestWindowS)
	for {
		select {
		case msg := <-p.ReceiveChannel(lib.Topic_PEERS_REQUEST):
			senderID := msg.Sender.Address.PublicKey
			sender := lib.BytesToString(senderID)
			blocked, totalBlock := l.NewRequest(sender)
			if blocked {
				p.ChangeReputation(senderID, PeerMaxPeerBookRequestsExceeded)
				continue
			}
			if totalBlock {
				continue // dos defensive
			}
			if _, ok := msg.Message.(*PeerBookRequestMessage); !ok {
				p.ChangeReputation(senderID, PeerInvalidMessageSlash)
				continue
			}
			p.book.RLock()
			err := p.SendTo(senderID, lib.Topic_PEERS_RESPONSE, &PeerBookResponseMessage{
				Book: p.book.book,
			})
			p.book.RUnlock()
			if err != nil {
				fmt.Println(err.Error()) // log error
			}
		case <-l.C():
			l.Reset()
		}
	}
}
func (p *P2P) ListenForPeerJoin() {
	l := lib.NewLimiter(MaxPeerBookRequestsPerWindow, p.MaxPossiblePeers()*MaxPeerBookRequestsPerWindow, PeerBookRequestWindowS)
	for {
		select {
		case msg := <-p.ReceiveChannel(lib.Topic_PEERS_REQUEST):
			senderID := msg.Sender.Address.PublicKey
			sender := lib.BytesToString(senderID)
			blocked, totalBlock := l.NewRequest(sender)
			if blocked {
				p.ChangeReputation(senderID, PeerMaxPeerBookRequestsExceeded)
				continue
			}
			if totalBlock {
				continue // dos defensive
			}
			if _, ok := msg.Message.(*PeerBookRequestMessage); !ok {
				p.ChangeReputation(senderID, PeerInvalidMessageSlash)
				continue
			}
			p.book.RLock()
			err := p.SendTo(senderID, lib.Topic_PEERS_RESPONSE, &PeerBookResponseMessage{
				Book: p.book.book,
			})
			p.book.RUnlock()
			if err != nil {
				fmt.Println(err.Error()) // log error
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
