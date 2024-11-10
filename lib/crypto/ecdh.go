package crypto

import (
	"crypto/ed25519"
	"crypto/sha512"
	"crypto/subtle"
	"filippo.io/edwards25519"
	"fmt"
	"golang.org/x/crypto/curve25519"
)

// https://cr.yp.to/ecdh.html

// Big picture: DH is used to establish a shared secret, and then HKDF is used to derive multiple keys from that secret for encryption

// SharedSecret function takes ed25519 public and private keys, converts them to Curve25519-compatible keys,
// and performs a Diffie-Hellman-style key exchange with X25519 - meaning both peers compute exact pseudorandom
// bytes from their peersPublicKey and their local private key without transmitting the secret over the wire
func SharedSecret(peerPublicKey, private []byte) ([]byte, error) {
	// convert the peer public key to Curve 25519
	xPub, err := Ed25519PublicKeyToCurve25519(peerPublicKey)
	if err != nil {
		return nil, err
	}
	// convert local private key to Curve 25519
	xPriv := Ed25519PrivateKeyToCurve25519(private)
	// generate a secret from the
	secret, err := curve25519.X25519(xPriv, xPub)
	if err != nil {
		return nil, err
	}
	// ensure the secret isn't an 'all-zero' byte array as this would be a weak or invalid key agreement
	if subtle.ConstantTimeCompare(secret[:], new([32]byte)[:]) == 1 {
		return nil, fmt.Errorf("all zero shared secret")
	}
	// return the diffie hellman secret
	return secret, nil
}

// Ed25519PrivateKeyToCurve25519 hashes the Ed25519 private key seed and extracts the first 32 bytes to form a
// Curve25519 scalar, which is compatible with Curve25519 operations
// This conversion allows the use of a single cryptographic key pair Ed25519 for both signing and key exchange
func Ed25519PrivateKeyToCurve25519(pk ed25519.PrivateKey) []byte {
	h := sha512.New()
	h.Write(pk.Seed())
	out := h.Sum(nil)
	return out[:curve25519.ScalarSize]
}

// Ed25519PublicKeyToCurve25519 interprets the Ed25519 public key as a point on the Edwards25519 curve and converts
// it to a Curve25519 public key in Montgomery form, suitable for X25519 encryption
// This conversion allows the use of a single cryptographic key pair Ed25519 for both signing and key exchange
func Ed25519PublicKeyToCurve25519(pk ed25519.PublicKey) ([]byte, error) {
	p, err := new(edwards25519.Point).SetBytes(pk)
	if err != nil {
		return nil, err
	}
	return p.BytesMontgomery(), nil
}

// x25519WeakPointBlacklist taken from lib-sodium
// https://github.com/jedisct1/libsodium/blob/985ad65bfb1563ca69e0bc0248e15da4f5cf575f/src/libsodium/crypto_scalarmult/curve25519/ref10/x25519_ref10.c
var x25519WeakPointBlacklist = [][32]byte{
	// 0 (order 4)
	{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	// 1 (order 1)
	{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	// 325606250916557431795983626356110631294008115727848805560023387167927233504
	//   (order 8)
	{0xe0, 0xeb, 0x7a, 0x7c, 0x3b, 0x41, 0xb8, 0xae, 0x16, 0x56, 0xe3,
		0xfa, 0xf1, 0x9f, 0xc4, 0x6a, 0xda, 0x09, 0x8d, 0xeb, 0x9c, 0x32,
		0xb1, 0xfd, 0x86, 0x62, 0x05, 0x16, 0x5f, 0x49, 0xb8, 0x00},
	// 39382357235489614581723060781553021112529911719440698176882885853963445705823
	//    (order 8)
	{0x5f, 0x9c, 0x95, 0xbc, 0xa3, 0x50, 0x8c, 0x24, 0xb1, 0xd0, 0xb1,
		0x55, 0x9c, 0x83, 0xef, 0x5b, 0x04, 0x44, 0x5c, 0xc4, 0x58, 0x1c,
		0x8e, 0x86, 0xd8, 0x22, 0x4e, 0xdd, 0xd0, 0x9f, 0x11, 0x57},
	// p-1 (order 2)
	{0xec, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f},
	// p (=0, order 4)
	{0xed, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f},
	// p+1 (=1, order 1)
	{0xee, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f},
}

// PubIsBlacklisted() prevents public keys that exploit vulnerabilities in 25519 -> x25519 conversion
// Reject small-order points early to prevent vulnerabilities from weak points that could be exploited in cryptographic operations,
// as recommended in the research (https://eprint.iacr.org/2017/806.pdf)
func PubIsBlacklisted(pubKey []byte) bool {
	for _, bl := range x25519WeakPointBlacklist {
		if subtle.ConstantTimeCompare(pubKey[:], bl[:]) == 1 {
			return true
		}
	}
	return false
}
