package p2p

import (
	"crypto/cipher"
	"encoding/binary"
	"github.com/alecthomas/units"
	lib "github.com/ginchuco/ginchu/types"
	"github.com/ginchuco/ginchu/types/crypto"
	pool "github.com/libp2p/go-buffer-pool"
	"golang.org/x/sync/errgroup"
	"io"
	"math"
	"net"
	"sync"
	"time"
)

/*
	Handshake to encrypted connection:
	1) Obtaining shared secret using diffie hellman and x25519 curve (ECDH)
	2) HKDF used to derive the encrypt key, auth key, and nonce (uniqueness IV) from the shared secret
	3) ChaCha20-Poly1305 AEAD scheme uses #2 for encrypt/decrypt/authenticate
*/

type EncryptedConn struct {
	conn    net.Conn
	receive internalState
	send    internalState

	peerPubKey crypto.PublicKeyI
}

type internalState struct {
	sync.Mutex
	aead   cipher.AEAD
	unread []byte // holds extra bytes that weren't read into `data` due to length
	nonce  *[crypto.AEADNonceSize]byte
}

func NewHandshake(conn net.Conn, privateKey crypto.PrivateKeyI) (c *EncryptedConn, err error) {
	ephemeralPublic, ephemeralPrivate := crypto.GenerateCurve25519Keypair()
	peerEphemeralPublic, err := keySwap(conn, ephemeralPublic)
	if err != nil {
		return
	}
	secret, err := crypto.SharedSecret(peerEphemeralPublic[:], ephemeralPrivate[:])
	if err != nil {
		return
	}
	sendAEAD, receiveAEAD, challenge, err := crypto.HKDFSecretsAndChallenge(secret, ephemeralPublic, peerEphemeralPublic)
	if err != nil {
		return
	}
	c = &EncryptedConn{
		conn:    conn,
		receive: newInternalState(receiveAEAD),
		send:    newInternalState(sendAEAD),
	}
	peer, err := signatureSwap(c, &lib.Signature{
		PublicKey: privateKey.PublicKey().Bytes(),
		Signature: privateKey.Sign(challenge[:]),
	})
	c.peerPubKey, err = lib.PublicKeyFromBytes(peer.PublicKey)
	if err != nil {
		return
	}
	if !c.peerPubKey.VerifyBytes(challenge[:], peer.Signature) {
		return nil, ErrFailedChallenge()
	}
	return
}

func (c *EncryptedConn) Write(data []byte) (n int, err error) {
	c.send.Lock()
	defer c.send.Unlock()
	chunk, dataLen, chunkLen := []byte(nil), len(data), 0
	for 0 < dataLen {
		encryptedBuffer, plainTextBuffer := pool.Get(crypto.EncryptedFrameSize), pool.Get(crypto.FrameSize)
		if dataLen < crypto.MaxDataSize {
			chunk = data
			data = nil
		} else {
			chunk = data[:crypto.MaxDataSize]
			data = data[crypto.MaxDataSize:]
		}
		dataLen, chunkLen = len(data), len(chunk)
		binary.LittleEndian.PutUint32(plainTextBuffer, uint32(chunkLen))             // data length header
		copy(plainTextBuffer[crypto.LengthHeaderSize:], chunk)                       // body
		c.send.aead.Seal(encryptedBuffer[:0], c.send.nonce[:], plainTextBuffer, nil) // encrypt
		incrementNonce(c.send.nonce)                                                 // increment nonce
		if _, err = c.conn.Write(encryptedBuffer); err != nil {                      // write
			return
		}
		n += chunkLen             // update bytes written
		pool.Put(encryptedBuffer) // put buffers back
		pool.Put(plainTextBuffer) // put buffers back
	}
	return
}

func (c *EncryptedConn) Read(data []byte) (n int, err error) {
	c.receive.Lock()
	defer c.receive.Unlock()
	if bzRead, hadUnread := c.checkUnread(data); hadUnread {
		return bzRead, nil
	}
	encryptedBuffer, plainTextBuffer := pool.Get(crypto.EncryptedFrameSize), pool.Get(crypto.FrameSize)
	defer func() { pool.Put(plainTextBuffer); pool.Put(encryptedBuffer) }()
	if _, err = io.ReadFull(c.conn, encryptedBuffer); err != nil {
		return
	}
	if _, err = c.receive.aead.Open(plainTextBuffer[:0], c.receive.nonce[:], encryptedBuffer, nil); err != nil {
		return n, ErrConnDecryptFailed()
	}
	incrementNonce(c.receive.nonce)
	chunkLength := binary.LittleEndian.Uint32(plainTextBuffer) // read the length header
	if chunkLength > crypto.MaxDataSize {
		return 0, ErrChunkLargerThanMax()
	}
	chunk := plainTextBuffer[crypto.LengthHeaderSize : crypto.LengthHeaderSize+chunkLength]
	n = copy(data, chunk)
	return c.populateUnread(n, chunk) // if any bytes not read, hold them in receive.unread
}

func (c *EncryptedConn) checkUnread(data []byte) (int, bool) {
	if len(c.receive.unread) > 0 {
		n := copy(data, c.receive.unread)
		c.receive.unread = c.receive.unread[n:]
		return n, true
	}
	return 0, false
}

func (c *EncryptedConn) populateUnread(bytesRead int, chunk []byte) (int, error) {
	if bytesRead < len(chunk) { // next call of read will read directly from unread
		c.receive.unread = make([]byte, len(chunk)-bytesRead)
		copy(c.receive.unread, chunk[bytesRead:])
	}
	return bytesRead, nil
}

func (c *EncryptedConn) Close() error                       { return c.conn.Close() }
func (c *EncryptedConn) LocalAddr() net.Addr                { return c.conn.LocalAddr() }
func (c *EncryptedConn) RemoteAddr() net.Addr               { return c.conn.RemoteAddr() }
func (c *EncryptedConn) SetDeadline(t time.Time) error      { return c.conn.SetDeadline(t) }
func (c *EncryptedConn) SetReadDeadline(t time.Time) error  { return c.conn.SetReadDeadline(t) }
func (c *EncryptedConn) SetWriteDeadline(t time.Time) error { return c.conn.SetWriteDeadline(t) }

func keySwap(conn io.ReadWriter, ephemeralPublicKey *[32]byte) (peerEphemeralPublic *[32]byte, err error) {
	var g errgroup.Group
	peerEphemeralPublic = &[32]byte{}
	g.Go(func() error { return sendKey(conn, ephemeralPublicKey) })
	g.Go(func() error { return receiveKey(conn, peerEphemeralPublic) })
	if err = g.Wait(); err != nil {
		return nil, err
	}
	return
}

func signatureSwap(conn io.ReadWriter, signature *lib.Signature) (peerSig *lib.Signature, err error) {
	var g errgroup.Group
	g.Go(func() error { return sendSig(conn, signature) })
	g.Go(func() error { return receiveSig(conn, peerSig) })
	if err = g.Wait(); err != nil {
		return nil, err
	}
	return
}

func sendSig(conn io.ReadWriter, signature *lib.Signature) error {
	bz, err := lib.Marshal(signature)
	if err != nil {
		return err
	}
	if _, er := conn.Write(bz); er != nil {
		return er
	}
	return nil
}

func receiveSig(conn io.ReadWriter, signature *lib.Signature) error {
	buffer := make([]byte, units.Megabyte)
	if _, err := conn.Read(buffer); err != nil {
		return err
	}
	if err := lib.Unmarshal(buffer, signature); err != nil {
		return err
	}
	return nil
}

func sendKey(conn io.ReadWriter, ephemeralPublicKey *[32]byte) error {
	bz, err := lib.Marshal(ephemeralPublicKey)
	if err != nil {
		return err
	}
	if _, er := conn.Write(bz); er != nil {
		return er
	}
	return nil
}

func receiveKey(conn io.ReadWriter, ephemeralPublicKey *[32]byte) error {
	buffer := make([]byte, units.Megabyte)
	if _, err := conn.Read(buffer); err != nil {
		return err
	}
	if err := lib.Unmarshal(buffer, ephemeralPublicKey); err != nil {
		return err
	}
	if crypto.CheckBlacklist(ephemeralPublicKey) {
		return ErrIsBlacklisted()
	}
	return nil
}

// chacha20-poly1305 expects 12 byte nonce
func incrementNonce(nonce *[crypto.AEADNonceSize]byte) {
	counter := binary.LittleEndian.Uint64(nonce[4:])
	if counter == math.MaxUint64 {
		panic("overflow")
	}
	counter++
	binary.LittleEndian.PutUint64(nonce[4:], counter)
}

func newInternalState(aead cipher.AEAD) internalState {
	return internalState{
		Mutex: sync.Mutex{},
		aead:  aead,
		nonce: new([crypto.AEADNonceSize]byte),
	}
}
