package crypto

import (
	"bytes"
	"crypto/cipher"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/hkdf"
	"io"
)

const (
	LengthHeaderSize   = 4
	MaxDataSize        = 1024
	ChallengeSize      = 32
	Poly1305TagSize    = 16
	FrameSize          = MaxDataSize + LengthHeaderSize
	EncryptedFrameSize = Poly1305TagSize + FrameSize
	AEADKeySize        = chacha20poly1305.KeySize
	AEADNonceSize      = chacha20poly1305.NonceSize
	TwoAEADKeySize     = 2 * AEADKeySize
	HKDFSize           = TwoAEADKeySize + ChallengeSize // 2 keys and challenge
)

func HKDFSecretsAndChallenge(dhSecret []byte, ePub, ePeerPub *[32]byte) (send cipher.AEAD, receive cipher.AEAD, challenge *[32]byte, err error) {
	hkdfReader := hkdf.New(Hasher, dhSecret, nil, nil)
	buffer := new([HKDFSize]byte)
	if _, err = io.ReadFull(hkdfReader, buffer[:]); err != nil {
		return
	}
	challenge, receiveSecret, sendSecret := new([ChallengeSize]byte), new([AEADKeySize]byte), new([AEADKeySize]byte)
	if bytes.Compare(ePub[:], ePeerPub[:]) < 0 {
		getTwoSecretsFromBuffer(buffer, receiveSecret, sendSecret)
	} else {
		getTwoSecretsFromBuffer(buffer, sendSecret, receiveSecret)
	}
	copy(challenge[:], buffer[TwoAEADKeySize:HKDFSize])
	send, err = chacha20poly1305.New(sendSecret[:])
	if err != nil {
		return
	}
	receive, err = chacha20poly1305.New(receiveSecret[:])
	if err != nil {
		return
	}
	return
}

func getTwoSecretsFromBuffer(buffer *[HKDFSize]byte, first, second *[32]byte) {
	copy(first[:], buffer[0:AEADKeySize])
	copy(second[:], buffer[AEADKeySize:TwoAEADKeySize])
}
