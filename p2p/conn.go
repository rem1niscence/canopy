package p2p

import (
	"bufio"
	"context"
	"net"
	"sync"
	"time"

	"github.com/alecthomas/units"
	"github.com/canopy-network/canopy/lib"
	limiter "github.com/mxk/go-flowrate/flowrate"
	"golang.org/x/sync/semaphore"
	"google.golang.org/protobuf/proto"
)

const (
	maxDataChunkSize    = 1024 - packetHeaderSize // maximum size of the chunk of bytes in a packet
	maxPacketSize       = 1024                    // maximum size of the full packet
	packetHeaderSize    = 47                      // the overhead of the protobuf packet header
	pingInterval        = 30 * time.Second        // how often a ping is to be sent
	sendInterval        = 100 * time.Millisecond  // the minimum time between sends
	pongTimeoutDuration = 20 * time.Second        // how long the sender of a ping waits for a pong before throwing an error
	queueSendTimeout    = 10 * time.Second        // how long a message waits to be queued before throwing an error
	dataFlowRatePerS    = 500 * units.KB          // the maximum number of bytes that may be sent or received per second per MultiConn
	maxMessageSize      = 10 * units.Megabyte     // the maximum total size of a message once all the packets are added up
	maxChanSize         = 1                       // maximum number of items in a channel before blocking
	maxQueueSize        = 1                       // maximum number of items in a queue before blocking
	maxConcurrency      = 50                      // max concurrency of packet send
	maxSendQueue        = 100                     // max send queue of packet size

	// "Peer Reputation Points" are actively maintained for each peer the node is connected to
	// These points allow a node to track peer behavior over its lifetime, allowing it to disconnect from faulty peers
	PollMaxHeightTimeoutS   = 1   // wait time for polling the maximum height of the peers
	SyncTimeoutS            = 5   // wait time to receive an individual block (certificate) from a peer during syncing
	MaxBlockReqPerWindow    = 20  // maximum block (certificate) requests per window per requester
	BlockReqWindowS         = 2   // the 'window of time' before resetting limits for block (certificate) requests
	GoodPeerBookRespRep     = 3   // reputation points for a good peer book response
	GoodBlockRep            = 3   // rep boost for sending us a valid block (certificate)
	GoodTxRep               = 3   // rep boost for sending us a valid transaction (certificate)
	BadPacketSlash          = -1  // bad packet is received
	NoPongSlash             = -1  // no pong received
	TimeoutRep              = -1  // rep slash for not responding in time
	UnexpectedBlockRep      = -1  // rep slash for sending us a block we weren't expecting
	PeerBookReqTimeoutRep   = -1  // slash for a non-response for a peer book request
	UnexpectedMsgRep        = -1  // slash for an unexpected message
	InvalidMsgRep           = -3  // slash for an invalid message
	ExceedMaxPBReqRep       = -3  // slash for exceeding the max peer book requests
	ExceedMaxPBLenRep       = -3  // slash for exceeding the size of the peer book message
	UnknownMessageSlash     = -3  // unknown message type is received
	BadStreamSlash          = -3  // unknown stream id is received
	InvalidTxRep            = -3  // rep slash for sending us an invalid transaction
	NotValRep               = -3  // rep slash for sending us a validator only message but not being a validator
	InvalidBlockRep         = -3  // rep slash for sending an invalid block (certificate) message
	InvalidJustifyRep       = -3  // rep slash for sending an invalid certificate justification
	BlockReqExceededRep     = -3  // rep slash for over-requesting blocks (certificates)
	MaxMessageExceededSlash = -10 // slash for sending a 'Message (sum of Packets)' above the allowed maximum size
)

// sendNotifier: holds channels for send notifications of concurrent processes
type sendNotifier struct {
	notifySendQueue chan struct{} // queue for packets to be sent

}

// newSendNotifier creates a new instance of sendNotifier with constant config default values
func newSendNotifier() *sendNotifier {
	return &sendNotifier{
		notifySendQueue: make(chan struct{}, maxSendQueue),
	}
}

func (n *sendNotifier) notifySend() bool {
	select {
	case n.notifySendQueue <- struct{}{}:
		return true
	case <-time.After(queueSendTimeout):
		return false
	}
}

// MultiConn: A rate-limited, multiplexed connection that utilizes a series streams with varying priority for sending and receiving
type MultiConn struct {
	conn          net.Conn                    // underlying connection
	Address       *lib.PeerAddress            // authenticated peer information
	streams       map[lib.Topic]*Stream       // multiple independent bi-directional communication channels
	quitSending   chan struct{}               // signal to quit
	quitReceiving chan struct{}               // signal to quit
	sendPong      chan struct{}               // signal to send keep alive message
	receivedPong  chan struct{}               // signal that received keep alive message
	sendErrChan   chan lib.ErrorI             // signal of an error in concurrent send processes
	writeCh       chan []byte                 // channel to ensure c.conn.Write is not called concurrently
	sendNotifier  *sendNotifier               // notifier structure for concurrent sends
	onError       func(error, []byte, string) // callback to call if peer errors
	error         sync.Once                   // thread safety to ensure MultiConn.onError is only called once
	p2p           *P2P                        // a pointer reference to the P2P module
	log           lib.LoggerI                 // logging
}

// Stream: an independent, bidirectional communication channel that is scoped to a single topic.
// In a multiplexed connection there is typically more than one stream per connection
type Stream struct {
	topic            lib.Topic                    // the subject and priority of the stream
	sendQueue        chan []byte                  // a queue of unsent messages
	upNextToSend     []byte                       // a buffer holding unsent portions of the next message
	msgAssembler     []byte                       // collects and adds incoming packets until the entire message is received (EOF signal)
	inbox            chan *lib.MessageAndMetadata // the channel where fully received messages are held for other parts of the app to read
	maxDataChunkSize int                          // maximum size of the chunk of bytes in a packet
	sendNotifier     *sendNotifier                // notifier structure for concurrent sends
	logger           lib.LoggerI
}

// chunkNextSend() returns the next unsent chunk of bytes and if it's the final bytes of the msg
func (s *Stream) chunkNextSend() (chunk []byte, eof bool) {
	// If the remaining unsent bytes will fit in a single chunk
	if len(s.upNextToSend) <= s.maxDataChunkSize {
		chunk = s.upNextToSend          // set the chunk to the last bytes
		eof, s.upNextToSend = true, nil // signal message end and empty the upNext buffer
	} else {
		chunk = s.upNextToSend[:s.maxDataChunkSize]          // chunk the max number of bytes
		s.upNextToSend = s.upNextToSend[s.maxDataChunkSize:] // remove those bytes from upNext
		s.sendNotifier.notifySend()                          // notify new packet to general send
	}
	return
}

// NewConnection() creates and starts a new instance of a MultiConn
func (p *P2P) NewConnection(conn net.Conn) (*MultiConn, lib.ErrorI) {
	// establish an encrypted connection using the handshake
	eConn, err := NewHandshake(conn, p.meta, p.privateKey)
	if err != nil {
		return nil, err
	}
	n := newSendNotifier()
	c := &MultiConn{
		conn:          eConn,
		Address:       eConn.Address,
		streams:       p.NewStreams(n),
		quitSending:   make(chan struct{}, maxChanSize),
		quitReceiving: make(chan struct{}, maxChanSize),
		sendPong:      make(chan struct{}, maxChanSize),
		receivedPong:  make(chan struct{}, maxChanSize),
		sendErrChan:   make(chan lib.ErrorI, maxChanSize),
		writeCh:       make(chan []byte), // needs to be unbuffered so it doesn't call net.Conn concurrently
		sendNotifier:  n,
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
	go c.startWriteChannel()
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
	ok = c.sendNotifier.notifySend()
	return
}

// sendPacket sends the packet to peers, this function is intended to use concurrently
func (c *MultiConn) sendPacket(ctx context.Context, packet *Packet, sem *semaphore.Weighted, m *limiter.Monitor) {
	sErr := sem.Acquire(context.Background(), 1)
	if sErr != nil {
		c.sendErrChan <- ErrSemaphoreFailed(sErr, packet)
	}
	defer sem.Release(1)
	select {
	case <-ctx.Done(): // Exit immediately if the context is canceled (quit signal)
	default:
		c.log.Debugf("Send Packet(ID:%s, L:%d, E:%t)", lib.Topic_name[int32(packet.StreamId)], len(packet.Bytes), packet.Eof)
		err := c.sendWireBytes(packet, m)
		if err != nil {
			c.sendErrChan <- err
		}
	}
}

func (c *MultiConn) startWriteChannel() {
	defer lib.CatchPanic(c.log)
	for data := range c.writeCh {
		_, err := c.conn.Write(data)
		if err != nil {
			c.log.Error(ErrFailedWrite(err).Error())
			c.sendErrChan <- ErrFailedWrite(err)
		}
	}
}

func (c *MultiConn) queueWrite(b []byte) bool {
	select {
	case c.writeCh <- b: // enqueue to the back of the line
		return true
	case <-time.After(queueSendTimeout): // may timeout if queue remains full
		return false
	}
}

// startSendService() starts the main send service
// - converges and writes the send queue from all streams into the underlying tcp connection.
// - manages the keep alive protocol by sending pings and monitoring the receipt of the corresponding pong
func (c *MultiConn) startSendService() {
	defer lib.CatchPanic(c.log)
	m := limiter.New(0, 0)
	ping, err := time.NewTicker(pingInterval), lib.ErrorI(nil)
	pongTimer := time.NewTimer(pongTimeoutDuration)
	sem := semaphore.NewWeighted(maxConcurrency)
	ctx, cancel := context.WithCancel(context.Background())
	defer func() { lib.StopTimer(pongTimer); ping.Stop(); m.Done(); cancel() }()
	for {
		// select statement ensures the sequential coordination of the concurrent processes
		select {
		case <-c.sendNotifier.notifySendQueue: // triggered each time a packet is sent to the queue, concurrency controlled by semaphore
			if packet := c.getNextPacket(); packet != nil {
				go c.sendPacket(ctx, packet, sem, m)
			}
		case <-ping.C: // fires every 'pingInterval'
			c.log.Debugf("Send Ping to: %s", lib.BytesToTruncatedString(c.Address.PublicKey))
			// send a ping to the peer
			if err = c.sendWireBytes(new(Ping), m); err != nil {
				break
			}
			// reset the pong timer
			lib.StopTimer(pongTimer)
			// set the pong timer to execute an Error function if the timer expires before receiving a pong
			pongTimer = time.AfterFunc(pongTimeoutDuration, func() {
				if e := ErrPongTimeout(); e != nil {
					c.Error(e, NoPongSlash)
				}
			})
		case <-c.sendPong: // fires when receive service got a 'ping' message
			c.log.Debugf("Send Pong to: %s", lib.BytesToTruncatedString(c.Address.PublicKey))
			// send a pong
			err = c.sendWireBytes(new(Pong), m)
		case <-c.receivedPong: // fires when receive service got a 'pong' message
			// reset the pong timer
			lib.StopTimer(pongTimer)
		case <-c.quitSending: // fires when Stop() is called
			return
		case err = <-c.sendErrChan: // set error to sendErrChan sent value
		}
		if err != nil {
			c.Error(err)
			return
		}
	}
}

// startReceiveService() starts the main receive service
// - reads from the underlying tcp connection and 'routes' the messages to the appropriate streams
// - manages keep alive protocol by notifying the 'send service' of pings and pongs
func (c *MultiConn) startReceiveService() {
	defer lib.CatchPanic(c.log)
	reader, m := *bufio.NewReaderSize(c.conn, maxPacketSize), limiter.New(0, 0)
	defer func() { close(c.sendPong); close(c.receivedPong); m.Done() }()
	for {
		c.log.Debugf("Receive service ready for %s", lib.BytesToTruncatedString(c.Address.PublicKey))
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
				c.log.Debug("Received Packet")
				// load the proper stream
				stream, found := c.streams[x.StreamId]
				if !found {
					c.Error(ErrBadStream(), BadStreamSlash)
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
					c.log.Warnf(er.Error())
					c.Error(er, slash)
					return
				}
			case *Ping: // receive ping message notifies the "send" service to respond with a 'pong' message
				c.log.Debugf("Received ping from %s", lib.BytesToTruncatedString(c.Address.PublicKey))
				c.sendPong <- struct{}{}
			case *Pong: // receive pong message notifies the "send" service to disable the 'pong timer exit'
				c.log.Debugf("Received pong from %s", lib.BytesToTruncatedString(c.Address.PublicKey))
				c.receivedPong <- struct{}{}
			default: // unknown type results in slash and exiting the service
				c.Error(ErrUnknownP2PMsg(x), UnknownMessageSlash)
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
	//m.Limit(maxPacketSize, int64(dataFlowRatePerS), true)
	// read up to maxPacketSize bytes
	n, er := reader.Read(buffer)
	if er != nil {
		return nil, ErrFailedRead(er)
	}
	// update the rate limiter with how many bytes were read
	//m.Update(n)
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
	//m.Limit(maxPacketSize, int64(dataFlowRatePerS), true)
	// write bytes to the wire up to max packet size
	c.queueWrite(bz)
	// update the rate limiter with how many bytes were written
	//m.Update(n)
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

// handlePacket() merge the new packet with the previously received ones until the entire message is complete (EOF signal)
func (s *Stream) handlePacket(peerInfo *lib.PeerInfo, packet *Packet) (int32, lib.ErrorI) {
	msgAssemblerLen, packetLen := len(s.msgAssembler), len(packet.Bytes)
	s.logger.Debugf("Received Packet(ID:%s, L:%d, E:%t) from %s",
		lib.Topic_name[int32(packet.StreamId)], len(packet.Bytes), packet.Eof, lib.BytesToTruncatedString(peerInfo.Address.PublicKey))
	// if the addition of this new packet pushes the total message size above max
	if int(maxMessageSize) < msgAssemblerLen+packetLen {
		s.msgAssembler = s.msgAssembler[:0]
		return MaxMessageExceededSlash, ErrMaxMessageSize()
	}
	// combine this packet with the previously received ones
	s.msgAssembler = append(s.msgAssembler, packet.Bytes...)
	// if the packet is signalling message end
	if packet.Eof {
		// unmarshall all the bytes into the universal wrapper
		var msg Envelope
		if err := lib.Unmarshal(s.msgAssembler, &msg); err != nil {
			return BadPacketSlash, err
		}
		// read the payload into a proto.Message
		payload, err := lib.FromAny(msg.Payload)
		if err != nil {
			return BadPacketSlash, err
		}
		// wrap with metadata
		m := (&lib.MessageAndMetadata{
			Message: payload,
			Sender:  peerInfo,
		}).WithHash()
		// add to inbox for other parts of the app to read
		s.inbox <- m
		s.logger.Debugf("Forwarded packet(s) to inbox: %s", lib.Topic_name[int32(packet.StreamId)])
		// reset receiving buffer
		s.msgAssembler = s.msgAssembler[:0]
	}
	return 0, nil
}
