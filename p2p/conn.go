package p2p

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"sync"
	"time"

	"github.com/alecthomas/units"
	"github.com/canopy-network/canopy/lib"
	"github.com/google/uuid"
	limiter "github.com/mxk/go-flowrate/flowrate"
	"golang.org/x/sync/semaphore"
	"google.golang.org/protobuf/proto"
)

const (
	maxDataChunkSize    = 1024 - packetHeaderSize // maximum size of the chunk of bytes in a packet
	maxPacketSize       = 1024                    // maximum size of the full packet
	packetHeaderSize    = 103                     // the overhead of the protobuf packet header
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

// MultiConn: A rate-limited, multiplexed connection that utilizes a series streams with varying priority for sending and receiving
type MultiConn struct {
	conn            net.Conn                    // underlying connection
	Address         *lib.PeerAddress            // authenticated peer information
	streams         map[lib.Topic]*Stream       // multiple independent bi-directional communication channels
	quitSending     chan struct{}               // signal to quit
	quitReceiving   chan struct{}               // signal to quit
	sendPong        chan struct{}               // signal to send keep alive message
	receivedPong    chan struct{}               // signal that received keep alive message
	sendErrChan     chan lib.ErrorI             // signal of an error in concurrent send processes
	writeCh         chan []byte                 // channel to ensure c.conn.Write is not called concurrently
	notifySendQueue chan struct{}               // queue for notifying packets to be sent
	onError         func(error, []byte, string) // callback to call if peer errors
	error           sync.Once                   // thread safety to ensure MultiConn.onError is only called once
	p2p             *P2P                        // a pointer reference to the P2P module
	log             lib.LoggerI                 // logging
}

// NewConnection() creates and starts a new instance of a MultiConn
func (p *P2P) NewConnection(conn net.Conn) (*MultiConn, lib.ErrorI) {
	// establish an encrypted connection using the handshake
	eConn, err := NewHandshake(conn, p.meta, p.privateKey)
	if err != nil {
		return nil, err
	}
	c := &MultiConn{
		conn:            eConn,
		Address:         eConn.Address,
		streams:         p.NewStreams(),
		quitSending:     make(chan struct{}, maxChanSize),
		quitReceiving:   make(chan struct{}, maxChanSize),
		sendPong:        make(chan struct{}, maxChanSize),
		receivedPong:    make(chan struct{}, maxChanSize),
		sendErrChan:     make(chan lib.ErrorI, maxChanSize),
		writeCh:         make(chan []byte), // needs to be unbuffered so it doesn't call net.Conn concurrently
		notifySendQueue: make(chan struct{}, maxSendQueue),
		onError:         p.OnPeerError,
		error:           sync.Once{},
		p2p:             p,
		log:             p.log,
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

	chunks := split(bz, maxDataChunkSize)
	tPackets, er := uint64ToBytes(uint64(len(chunks)))
	if er != nil {
		return false
	}
	messageID := generateMessageID()
	for i, chunk := range chunks {
		pIndex, err := uint64ToBytes(uint64(i))
		if err != nil {
			return false
		}
		packet := &Packet{
			StreamId:     topic,
			MessageId:    messageID,
			PacketIndex:  pIndex,
			TotalPackets: tPackets,
			Bytes:        chunk,
		}

		ok = stream.queueSend(packet)
		ok = c.notifySend()
	}

	c.log.Debugf("Message ID: %s with %d packets queued", messageID, len(chunks))

	return
}

// split returns bytes splited to size up to the lim param
func split(buf []byte, lim int) [][]byte {
	var chunk []byte
	chunks := make([][]byte, 0, len(buf)/lim+1)
	for len(buf) >= lim {
		chunk, buf = buf[:lim], buf[lim:]
		chunks = append(chunks, chunk)
	}
	if len(buf) > 0 {
		chunks = append(chunks, buf[:])
	}
	return chunks
}

// generateMessageID generates a message ID with the UUID standard
func generateMessageID() string {
	return uuid.New().String()
}

// sendPacket sends the packet to peers, this function is intended to use concurrently
func (c *MultiConn) sendPacket(ctx context.Context, p *Packet, sem *semaphore.Weighted, m *limiter.Monitor) {
	sErr := sem.Acquire(context.Background(), 1)
	if sErr != nil {
		c.sendErrChan <- ErrSemaphoreFailed(sErr, p)
	}
	defer sem.Release(1)
	select {
	case <-ctx.Done(): // Exit immediately if the context is canceled (quit signal)
	default:
		c.log.Debugf("Send Packet(ID:%s, L:%d, MID:%s)", lib.Topic_name[int32(p.StreamId)], len(p.Bytes), p.MessageId)
		err := c.sendWireBytes(p, m)
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
		case <-c.notifySendQueue: // triggered each time a packet is sent to the queue, concurrency controlled by semaphore
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
		case _, open := <-c.sendPong: // fires when receive service got a 'ping' message
			// if the channel was closed
			if !open {
				// log the close
				c.log.Debugf("Pong channel closed, stopping")
				// exit
				return
			}
			// log the pong sending
			c.log.Debugf("Send Pong to: %s", lib.BytesToTruncatedString(c.Address.PublicKey))
			// send a pong
			err = c.sendWireBytes(new(Pong), m)
		case _, open := <-c.receivedPong: // fires when receive service got a 'pong' message
			// if the channel was closed
			if !open {
				// log the close
				c.log.Debugf("Receive pong channel closed, stopping")
				// exit
				return
			}
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
		select {
		case pkt := <-stream.sendQueue:
			return pkt
		default:
			continue // No packet available, try next stream
		}
	}
	return nil
}

// Stream: an independent, bidirectional communication channel that is scoped to a single topic.
// In a multiplexed connection there is typically more than one stream per connection
type Stream struct {
	topic        lib.Topic    // the subject and priority of the stream
	sendQueue    chan *Packet // a queue of unsent messages
	mu           sync.Mutex
	msgAssembler map[string]map[int][]byte    // collects and adds incoming packets until the entire message is received (total packets arrived) messageID -> packetIndex -> payload
	inbox        chan *lib.MessageAndMetadata // the channel where fully received messages are held for other parts of the app to read
	logger       lib.LoggerI
}

// queueSend() schedules the packet to be sent
func (s *Stream) queueSend(p *Packet) bool {
	select {
	case s.sendQueue <- p: // enqueue to the back of the line
		return true
	case <-time.After(queueSendTimeout): // may timeout if queue remains full
		return false
	}
}

// notifySend notifies Multiconn of a new packet added
func (c *MultiConn) notifySend() bool {
	select {
	case c.notifySendQueue <- struct{}{}:
		return true
	case <-time.After(queueSendTimeout):
		return false
	}
}

// handlePacket() merge the new packet with the previously received ones until the entire message is complete (EOF signal)
func (s *Stream) handlePacket(peerInfo *lib.PeerInfo, p *Packet) (int32, lib.ErrorI) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logger.Debugf("Received Packet(ID:%s, L:%d) from %s",
		lib.Topic_name[int32(p.StreamId)], len(p.Bytes), p.MessageId, lib.BytesToTruncatedString(peerInfo.Address.PublicKey))

	// need to convert from bytes to uint64 to fill the msgAssembler map
	tPackets, er := bytesToUint64(p.TotalPackets)
	if er != nil {
		return 0, ErrParseBytesFailed(er)
	}
	pIndex, er := bytesToUint64(p.PacketIndex)
	if er != nil {
		return 0, ErrParseBytesFailed(er)
	}

	// create map of given message ID if it is the first one and saved it afterwards
	_, ok := s.msgAssembler[p.MessageId]
	if !ok {
		s.msgAssembler[p.MessageId] = make(map[int][]byte, tPackets)
	}
	s.msgAssembler[p.MessageId][int(pIndex)] = p.Bytes

	// if a message has all the packets do the process
	if len(s.msgAssembler[p.MessageId]) == int(tPackets) {
		// after the message is processed it should be deleted from the assembler
		defer delete(s.msgAssembler, p.MessageId)
		// build bytes to unmarshal
		var allBytes []byte
		for i := 0; i < int(tPackets); i++ {
			allBytes = append(allBytes, s.msgAssembler[p.MessageId][i]...)
		}
		// if the addition of this new packet pushes the total message size above max
		if int(maxMessageSize) < len(allBytes) {
			return MaxMessageExceededSlash, ErrMaxMessageSize()
		}
		// unmarshall all the bytes into the universal wrapper
		var msg Envelope
		if err := lib.Unmarshal(allBytes, &msg); err != nil {
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
		s.logger.Debugf("Forwarded packet(s) to inbox: %s", lib.Topic_name[int32(p.StreamId)])
	}
	return 0, nil
}

// uint64ToBytes is a function to convert uint64 to bytes, needed for Packets header
func uint64ToBytes(n uint64) ([]byte, error) {
	buf := new(bytes.Buffer)
	// Convert the uint64 to bytes in little-endian byte order
	err := binary.Write(buf, binary.LittleEndian, n)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// bytesToUint64 is a function to convert bytes back to uint64, needed for Packets header
func bytesToUint64(b []byte) (uint64, error) {
	buf := bytes.NewReader(b)
	var n uint64
	// Read the bytes into the uint64 variable in little-endian byte order
	err := binary.Read(buf, binary.LittleEndian, &n)
	if err != nil {
		return 0, err
	}
	return n, nil
}
