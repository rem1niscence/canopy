package p2p

import (
	"bufio"
	"github.com/alecthomas/units"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	limiter "github.com/mxk/go-flowrate/flowrate"
	"google.golang.org/protobuf/proto"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const (
	maxPayloadSize      = 1024
	minReadBufferSize   = 1024
	pingInterval        = 30 * time.Second
	sendInterval        = 100 * time.Millisecond
	pongTimeoutDuration = 20 * time.Second
	queueSendTimeout    = 10 * time.Second
	sendRatePerS        = 500 * units.KB
	recRatePerS         = 500 * units.KB
	maxMessageSize      = 50 * units.Megabyte
	maxChannelCalls     = 10000
	maxQueueSize        = 100

	maxMessageExceededSlash = -10
	unknownMessageSlash     = -3
	badStreamSlash          = -3
	badPacketSlash          = -1
	noPongSlash             = -1
)

/*
	A rate-limited, multiplexed connection that utilizes a series streams with varying priority for sending and receiving
*/

type MultiConn struct {
	conn          net.Conn
	peerPublicKey []byte
	streams       map[lib.Topic]*Stream
	quitSending   chan struct{} // signal to quit
	quitReceiving chan struct{} // signal to quit
	sendPong      chan struct{}
	receivedPong  chan struct{}
	onError       func([]byte)
	error         sync.Once
	p2p           *P2P
	log           lib.LoggerI
}

func (p *P2P) NewConnection(conn net.Conn) (*MultiConn, lib.ErrorI) {
	eConn, err := NewHandshake(conn, p.privateKey)
	if err != nil {
		return nil, err
	}
	streams := p.NewStreams()
	for _, s := range streams {
		s.conn = eConn
	}
	c := &MultiConn{
		conn:          eConn,
		peerPublicKey: eConn.peerPubKey.Bytes(),
		streams:       streams,
		quitSending:   make(chan struct{}, maxChannelCalls),
		quitReceiving: make(chan struct{}, maxChannelCalls),
		sendPong:      make(chan struct{}, maxChannelCalls),
		receivedPong:  make(chan struct{}, maxChannelCalls),
		onError:       p.OnPeerError,
		error:         sync.Once{},
		p2p:           p,
		log:           p.log,
	}
	_ = c.conn.SetReadDeadline(time.Time{})
	_ = c.conn.SetWriteDeadline(time.Time{})
	c.Start()
	return c, err
}

func (c *MultiConn) Start() {
	go c.startSendLoop()
	go c.startReceiveLoop()
}

func (c *MultiConn) Stop() {
	c.p2p.log.Warnf("Stopping peer %s@%s", lib.BytesToString(c.peerPublicKey), c.conn.RemoteAddr().String())
	c.quitReceiving <- struct{}{}
	c.quitSending <- struct{}{}
	close(c.quitSending)
	close(c.quitReceiving)
	_ = c.conn.Close()
}

func (c *MultiConn) Send(topic lib.Topic, msg *Envelope) (ok bool) {
	stream, ok := c.streams[topic]
	if !ok {
		return
	}
	bz, err := lib.Marshal(msg)
	if err != nil {
		return false
	}
	if ok = stream.queueSend(bz); !ok {
		return
	}
	return
}

func (c *MultiConn) startSendLoop() {
	defer c.catchPanic()
	send, m := time.NewTicker(sendInterval), limiter.New(0, 0)
	ping, err := time.NewTicker(pingInterval), lib.ErrorI(nil)
	pongTimer, didntReceivePong := new(time.Timer), make(chan struct{}, maxChannelCalls)
	defer func() { close(didntReceivePong); pongTimer.Stop(); ping.Stop(); send.Stop(); m.Done() }()
	for {
		select {
		case <-send.C:
			if packet := c.getNextPacket(); packet != nil {
				err = c.send(packet, m)
			}
		case <-ping.C:
			if err = c.send(new(Ping), m); err != nil {
				break
			}
			pongTimer = time.AfterFunc(pongTimeoutDuration, func() { didntReceivePong <- struct{}{} })
		case <-c.sendPong:
			err = c.send(new(Pong), m)
		case <-c.receivedPong:
			_ = pongTimer.Stop()
			pongTimer = new(time.Timer)
		case <-didntReceivePong:
			err = ErrPongTimeout()
			if err != nil {
				c.Error(noPongSlash)
				return
			}
		case <-c.quitSending:
			return
		}
		if err != nil {
			c.log.Error(err.Error())
			c.Error()
			return
		}
	}
}

func (c *MultiConn) startReceiveLoop() {
	defer c.catchPanic()
	reader, m := *bufio.NewReaderSize(c.conn, minReadBufferSize), limiter.New(0, 0)
	defer func() { close(c.sendPong); close(c.receivedPong); m.Done() }()
	for {
		select {
		default:
			msg, err := c.receive(reader, m)
			if err != nil {
				c.log.Error(err.Error())
				c.Error()
				return
			}
			switch x := msg.(type) {
			case *Packet:
				stream, ok := c.streams[x.StreamId]
				if !ok {
					c.Error(badStreamSlash)
					return
				}
				info, _ := c.p2p.GetPeerInfo(c.peerPublicKey)
				if slash, er := stream.handlePacket(info, x); er != nil {
					c.Error(slash)
					return
				}
			case *Ping:
				c.sendPong <- struct{}{}
			case *Pong:
				c.receivedPong <- struct{}{}
			default:
				_ = ErrUnknownP2PMsg(x)
				c.Error(unknownMessageSlash)
				return
			}
		case <-c.quitReceiving:
			return
		}
	}
}

func (c *MultiConn) Error(reputationDelta ...int32) {
	if len(reputationDelta) == 1 {
		c.p2p.ChangeReputation(c.peerPublicKey, reputationDelta[0])
	}
	c.error.Do(func() { c.onError(c.peerPublicKey) })
}

var (
	maxPacketSize int
)

func (c *MultiConn) receive(reader bufio.Reader, m *limiter.Monitor) (proto.Message, lib.ErrorI) {
	if maxPacketSize == 0 {
		maxPacket, _ := lib.Marshal(&Packet{StreamId: lib.Topic_BLOCK, Eof: false, Bytes: make([]byte, maxPayloadSize)})
		maxPacketSize = len(maxPacket)
	}
	msg := new(Envelope)
	buffer := make([]byte, maxPacketSize)
	m.Limit(maxPacketSize, int64(recRatePerS), true)
	n, er := reader.Read(buffer)
	m.Update(n)
	if er != nil {
		return nil, ErrFailedRead(er)
	}
	if err := lib.Unmarshal(buffer[:n], msg); err != nil {
		return nil, err
	}
	return lib.FromAny(msg.Payload)
}

func (c *MultiConn) send(message proto.Message, m *limiter.Monitor) (err lib.ErrorI) {
	a, err := lib.NewAny(message)
	if err != nil {
		return err
	}
	bz, err := lib.Marshal(&Envelope{
		Payload: a,
	})
	m.Limit(maxPacketSize, int64(sendRatePerS), true)
	n, er := c.conn.Write(bz)
	if er != nil {
		return ErrFailedWrite(er)
	}
	m.Update(n)
	return
}

func (c *MultiConn) getNextPacket() *Packet {
	// ordered by stream priority
	for i := lib.Topic(0); i < lib.Topic_INVALID; i++ {
		stream := c.streams[i]
		if stream.hasStuffToSend() {
			return stream.nextPacket()
		}
	}
	return nil
}

func (c *MultiConn) catchPanic() {
	if r := recover(); r != nil {
		c.Error()
	}
}

type Stream struct {
	conn          *EncryptedConn
	topic         lib.Topic
	sendQueue     chan []byte
	sendQueueSize atomic.Int32
	upNextToSend  []byte
	receive       chan *lib.MessageWrapper
	receiving     []byte
}

func (s *Stream) queueSend(b []byte) bool {
	select {
	case s.sendQueue <- b:
		s.sendQueueSize.Add(1)
		return true
	case <-time.After(queueSendTimeout):
		return false
	}
}

func (s *Stream) hasStuffToSend() bool {
	if len(s.upNextToSend) != 0 {
		return true
	}
	if len(s.sendQueue) != 0 {
		s.upNextToSend = <-s.sendQueue
		return true
	}
	return false
}

func (s *Stream) nextPacket() (packet *Packet) {
	packet = &Packet{StreamId: s.topic}
	packet.Bytes, packet.Eof = s.chunkNextSend()
	return
}

func (s *Stream) chunkNextSend() (chunk []byte, eof bool) {
	if maxPayloadSize < len(s.upNextToSend) {
		chunk = s.upNextToSend[:maxPayloadSize]
		s.upNextToSend = s.upNextToSend[maxPayloadSize:]
	} else {
		s.sendQueueSize.Add(-1)
		chunk = s.upNextToSend
		eof, s.upNextToSend = true, nil
	}
	return
}

func (s *Stream) handlePacket(peerInfo *lib.PeerInfo, packet *Packet) (int32, lib.ErrorI) {
	if int(maxMessageSize) < len(s.receiving)+len(packet.Bytes) {
		s.receiving = make([]byte, 0, maxMessageSize)
		return maxMessageExceededSlash, ErrMaxMessageSize()
	}
	s.receiving = append(s.receiving, packet.Bytes...)
	if packet.Eof {
		var msg Envelope
		if err := lib.Unmarshal(s.receiving, &msg); err != nil {
			return badPacketSlash, err
		}
		payload, err := lib.FromAny(msg.Payload)
		if err != nil {
			return badPacketSlash, err
		}
		s.receive <- &lib.MessageWrapper{
			Message: payload,
			Hash:    crypto.Hash(s.receiving),
			Sender:  peerInfo,
		}
		s.receiving = make([]byte, 0, maxMessageSize)
	}
	return 0, nil
}
