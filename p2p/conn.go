package p2p

import (
	"bufio"
	"github.com/alecthomas/units"
	"github.com/ginchuco/ginchu/lib"
	limiter "github.com/mxk/go-flowrate/flowrate"
	"google.golang.org/protobuf/proto"
	"net"
	"sync"
	"time"
)

const (
	maxDataChunkSize    = 1024                   // maximum size of the chunk of bytes in a packet
	maxPacketSize       = 1031                   // maximum size of the full packet maxDataChunkSize + header data
	pingInterval        = 30 * time.Second       // how often a ping is to be sent
	sendInterval        = 100 * time.Millisecond // the minimum time between sends
	pongTimeoutDuration = 20 * time.Second       // how long the sender of a ping waits for a pong before throwing an error
	queueSendTimeout    = 10 * time.Second       // how long a message waits to be queued before throwing an error
	dataFlowRatePerS    = 500 * units.KB         // the maximum number of bytes that may be sent or received per second per MultiConn
	maxMessageSize      = 10 * units.Megabyte    // the maximum total size of a message once all the packets are added up
	maxChanSize         = 100                    // maximum number of items in a channel before blocking
	maxQueueSize        = 100                    // maximum number of items in a queue before blocking

	maxMessageExceededSlash = -10 // slash for sending a 'Message (sum of Packets)' above the allowed maximum size
	unknownMessageSlash     = -3  // unknown message type is received
	badStreamSlash          = -3  // unknown stream id is received
	badPacketSlash          = -1  // bad packet is received
	noPongSlash             = -1  // no pong received
)

// MultiConn: A rate-limited, multiplexed connection that utilizes a series streams with varying priority for sending and receiving
type MultiConn struct {
	conn          net.Conn                    // underlying connection
	Address       *lib.PeerAddress            // authenticated peer information
	streams       map[lib.Topic]*Stream       // multiple independent bi-directional communication channels
	quitSending   chan struct{}               // signal to quit
	quitReceiving chan struct{}               // signal to quit
	sendPong      chan struct{}               // signal to send keep alive message
	receivedPong  chan struct{}               // signal that received keep alive message
	onError       func(error, []byte, string) // callback to call if peer errors
	error         sync.Once                   // thread safety to ensure MultiConn.onError is only called once
	p2p           *P2P                        // a pointer reference to the P2P module
	log           lib.LoggerI                 // logging
}

// NewConnection() creates and starts a new instance of a MultiConn
func (p *P2P) NewConnection(conn net.Conn) (*MultiConn, lib.ErrorI) {
	// establish an encrypted connection using the handshake
	eConn, err := NewHandshake(conn, p.meta, p.privateKey)
	if err != nil {
		return nil, err
	}
	c := &MultiConn{
		conn:          eConn,
		Address:       eConn.Address,
		streams:       p.NewStreams(),
		quitSending:   make(chan struct{}, maxChanSize),
		quitReceiving: make(chan struct{}, maxChanSize),
		sendPong:      make(chan struct{}, maxChanSize),
		receivedPong:  make(chan struct{}, maxChanSize),
		onError:       p.OnPeerError,
		error:         sync.Once{},
		p2p:           p,
		log:           p.log,
	}
	_ = c.conn.SetReadDeadline(time.Time{})
	_ = c.conn.SetWriteDeadline(time.Time{})
	// start the connection service
	c.Start()
	return c, err
}

// Start() begins send and receive services for a MultiConn
func (c *MultiConn) Start() {
	go c.startSendService()
	go c.startReceiveService()
}

// Stop() sends exit signals for send and receive loops and closes the connection
func (c *MultiConn) Stop() {
	c.p2p.log.Warnf("Stopping peer %s", lib.BytesToString(c.Address.PublicKey))
	c.quitReceiving <- struct{}{}
	c.quitSending <- struct{}{}
	close(c.quitSending)
	close(c.quitReceiving)
	_ = c.conn.Close()
}

// Send() queues the sending of a message to a specific Stream
func (c *MultiConn) Send(topic lib.Topic, msg *Envelope) (ok bool) {
	stream, ok := c.streams[topic]
	if !ok {
		return
	}
	bz, err := lib.Marshal(msg)
	if err != nil {
		return false
	}
	ok = stream.queueSend(bz)
	return
}

// startSendService() starts the main send service
// - converges and writes the send queue from all streams into the underlying tcp connection.
// - manages the keep alive protocol by sending pings and monitoring the receipt of the corresponding pong
func (c *MultiConn) startSendService() {
	defer lib.CatchPanic(c.log)
	send, m := time.NewTicker(sendInterval), limiter.New(0, 0)
	ping, err := time.NewTicker(pingInterval), lib.ErrorI(nil)
	pongTimer := time.NewTimer(pongTimeoutDuration)
	defer func() { lib.StopTimer(pongTimer); ping.Stop(); send.Stop(); m.Done() }()
	for {
		// select statement ensures the sequential coordination of the concurrent processes
		select {
		case <-send.C: // fires every 'sendInterval'
			if packet := c.getNextPacket(); packet != nil {
				err = c.sendWireBytes(packet, m)
			}
		case <-ping.C: // fires every 'pingInterval'
			// send a ping to the peer
			if err = c.sendWireBytes(new(Ping), m); err != nil {
				break
			}
			// set the pong timer to execute an Error function if the timer expires before receiving a pong
			pongTimer = time.AfterFunc(pongTimeoutDuration, func() {
				if e := ErrPongTimeout(); e != nil {
					c.Error(e, noPongSlash)
				}
			})
		case <-c.sendPong: // fires when receive service got a 'ping' message
			// send a pong
			err = c.sendWireBytes(new(Pong), m)
		case <-c.receivedPong: // fires when receive service got a 'pong' message
			// reset the pong timer
			lib.ResetTimer(pongTimer, pongTimeoutDuration)
		case <-c.quitSending: // fires when Stop() is called
			return
		}
		if err != nil {
			c.Error(err)
			return
		}
	}
}

// startReceiveService() starts the main receive service
// -
func (c *MultiConn) startReceiveService() {
	defer lib.CatchPanic(c.log)
	reader, m := *bufio.NewReaderSize(c.conn, maxPacketSize), limiter.New(0, 0)
	defer func() { close(c.sendPong); close(c.receivedPong); m.Done() }()
	for {
		select {
		default: // fires unless quit was signaled
			// waits until bytes are received from the conn
			msg, err := c.waitForAndHandleWireBytes(reader, m)
			if err != nil {
				c.Error(err)
				return
			}
			// handle different message types
			switch x := msg.(type) {
			case *Packet: // receive packet is a partial or full 'Message' with a Stream Topic designation and an EOF signal
				// load the proper stream
				stream, found := c.streams[x.StreamId]
				if !found {
					c.Error(ErrBadStream(), badStreamSlash)
					return
				}
				// get the peer info from the peer set
				info, e := c.p2p.GetPeerInfo(c.Address.PublicKey)
				if e != nil {
					c.Error(e)
					return
				}
				// handle the packet within the stream
				if slash, er := stream.handlePacket(info, x); er != nil {
					c.Error(er, slash)
					return
				}
			case *Ping: // receive ping message notifies the "send" service to respond with a 'pong' message
				c.sendPong <- struct{}{}
			case *Pong: // receive pong message notifies the "send" service to disable the 'pong timer exit'
				c.receivedPong <- struct{}{}
			default: // unknown type results in slash and exiting the service
				c.Error(ErrUnknownP2PMsg(x), unknownMessageSlash)
				return
			}
		case <-c.quitReceiving: // fires when quit is signaled
			return
		}
	}
}

// Error() when an error occurs on the MultiConn execute a callback. Optionally pass a reputation delta to slash the peer
func (c *MultiConn) Error(err error, reputationDelta ...int32) {
	if len(reputationDelta) == 1 {
		c.p2p.ChangeReputation(c.Address.PublicKey, reputationDelta[0])
	}
	// call onError() for the peer
	c.error.Do(func() { c.onError(err, c.Address.PublicKey, c.conn.RemoteAddr().String()) })
}

// waitForAndHandleWireBytes() a rate limited handler of inbound bytes from the wire.
// Blocks until bytes are received converts bytes into a proto.Message using an Envelope
func (c *MultiConn) waitForAndHandleWireBytes(reader bufio.Reader, m *limiter.Monitor) (proto.Message, lib.ErrorI) {
	// initialize the wrapper object
	msg := new(Envelope)
	// create a buffer up to the maximum packet size
	buffer := make([]byte, maxPacketSize)
	// restrict the instantaneous data flow to rate bytes per second
	// Limit() request maxPacketSize bytes from the limiter and the limiter
	// will block the execution until at or below the desired rate of flow
	m.Limit(maxPacketSize, int64(dataFlowRatePerS), true)
	// read up to maxPacketSize bytes
	n, er := reader.Read(buffer)
	if er != nil {
		return nil, ErrFailedRead(er)
	}
	// update the rate limiter with how many bytes were read
	m.Update(n)
	// unmarshal the buffer
	if err := lib.Unmarshal(buffer[:n], msg); err != nil {
		return nil, err
	}
	return lib.FromAny(msg.Payload)
}

// sendWireBytes() a rate limited writer of outbound bytes to the wire
// wraps a proto.Message into a universal Envelope, then converts to bytes and
// sends them across the wire without violating the data flow rate limits
// message may be a Packet, a Ping or a Pong
func (c *MultiConn) sendWireBytes(message proto.Message, m *limiter.Monitor) (err lib.ErrorI) {
	// convert the proto.Message into a proto.Any
	a, err := lib.NewAny(message)
	if err != nil {
		return err
	}
	// wrap into an Envelope
	bz, err := lib.Marshal(&Envelope{
		Payload: a,
	})
	// restrict the instantaneous data flow to rate bytes per second
	// Limit() request maxPacketSize bytes from the limiter and the limiter
	// will block the execution until at or below the desired rate of flow
	m.Limit(maxPacketSize, int64(dataFlowRatePerS), true)
	// write bytes to the wire up to max packet size
	n, er := c.conn.Write(bz)
	if er != nil {
		return ErrFailedWrite(er)
	}
	// update the rate limiter with how many bytes were written
	m.Update(n)
	return
}

// getNextPacket() returns the next packet to send ordered by stream.Topic priority
func (c *MultiConn) getNextPacket() *Packet {
	// ordered by stream priority
	// NOTE: switching between streams mid 'Message' is not
	// a problem as each stream has a unique receiving buffer
	for i := lib.Topic(0); i < lib.Topic_INVALID; i++ {
		stream := c.streams[i]
		if stream.hasStuffToSend() {
			return stream.nextPacket()
		}
	}
	return nil
}

// Stream: an independent, bidirectional communication channel that is scoped to a single topic.
// In a multiplexed connection there is typically more than one stream per connection
type Stream struct {
	topic        lib.Topic                    // the subject and priority of the stream
	sendQueue    chan []byte                  // a queue of unsent messages
	upNextToSend []byte                       // a buffer holding unsent portions of the next message
	msgAssembler []byte                       // collects and adds incoming packets until the entire message is received (EOF signal)
	inbox        chan *lib.MessageAndMetadata // the channel where fully received messages are held for other parts of the app to read
}

// queueSend() schedules the bytes to be sent
// NOTE: at this phase these bytes are the entire message, not just a chunk/packet
func (s *Stream) queueSend(b []byte) bool {
	select {
	case s.sendQueue <- b: // enqueue to the back of the line
		return true
	case <-time.After(queueSendTimeout): // may timeout if queue remains full
		return false
	}
}

// hasStuffToSend() checks the stream to see if there's anything in the outbox
func (s *Stream) hasStuffToSend() bool {
	// if there's unsent parts of the next message
	if len(s.upNextToSend) != 0 {
		return true
	}
	// if there's unsent messages in the queue
	if len(s.sendQueue) != 0 {
		s.upNextToSend = <-s.sendQueue
		return true
	}
	// nothing to send
	return false
}

// nextPacket() creates a new packet from the next unsent chunk
func (s *Stream) nextPacket() (packet *Packet) {
	packet = &Packet{StreamId: s.topic}
	packet.Bytes, packet.Eof = s.chunkNextSend()
	return
}

// chunkNextSend() returns the next unsent chunk of bytes and if it's the final bytes of the msg
func (s *Stream) chunkNextSend() (chunk []byte, eof bool) {
	// If the remaining unsent bytes will fit in a single chunk
	if len(s.upNextToSend) <= maxDataChunkSize {
		chunk = s.upNextToSend          // set the chunk to the last bytes
		eof, s.upNextToSend = true, nil // signal message end and empty the upNext buffer
	} else {
		chunk = s.upNextToSend[:maxDataChunkSize]          // chunk the max number of bytes
		s.upNextToSend = s.upNextToSend[maxDataChunkSize:] // remove those bytes from upNext
	}
	return
}

// handlePacket() merge the new packet with the previously received ones until the entire message is complete (EOF signal)
func (s *Stream) handlePacket(peerInfo *lib.PeerInfo, packet *Packet) (int32, lib.ErrorI) {
	// if the addition of this new packet pushes the total message size above max
	if int(maxMessageSize) < len(s.msgAssembler)+len(packet.Bytes) {
		s.msgAssembler = make([]byte, 0, maxMessageSize)
		return maxMessageExceededSlash, ErrMaxMessageSize()
	}
	// combine this packet with the previously received ones
	s.msgAssembler = append(s.msgAssembler, packet.Bytes...)
	// if the packet is signalling message end
	if packet.Eof {
		// unmarshall all the bytes into the universal wrapper
		var msg Envelope
		if err := lib.Unmarshal(s.msgAssembler, &msg); err != nil {
			return badPacketSlash, err
		}
		// read the payload into a proto.Message
		payload, err := lib.FromAny(msg.Payload)
		if err != nil {
			return badPacketSlash, err
		}
		// wrap with metadata
		m := (&lib.MessageAndMetadata{
			Message: payload,
			Sender:  peerInfo,
		}).WithHash()
		// add to inbox for other parts of the app to read
		s.inbox <- m
		// reset receiving buffer
		s.msgAssembler = make([]byte, 0, maxMessageSize)
	}
	return 0, nil
}
