package crypto

import (
	"crypto/rand"
	"testing"
)

func TestED25519Bytes(t *testing.T) {
	// private key testing
	privateKey, err := NewEd25519PrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	privKeyBz := privateKey.Bytes()
	privateKey2 := NewED25519PrivateKeyFromBytes(privKeyBz)
	if !privateKey.Equals(privateKey2) {
		t.Fatalf("wanted %s, got %s", privateKey, privateKey2)
	}
	// public key testing
	pubKey := privateKey.PublicKey()
	pubKeyBz := pubKey.Bytes()
	pubKey2 := NewED25519PubKeyFromBytes(pubKeyBz)
	if !pubKey.Equals(pubKey2) {
		t.Fatalf("wanted %s got %s", pubKey, pubKey2)
	}
	// address testing
	address := pubKey.Address()
	addressBz := address.Bytes()
	address2 := NewAddressFromBytes(addressBz)
	if !address.Equals(address2) {
		t.Fatalf("wanted %s got %s", address, address2)
	}
}

func TestED25519SignAndVerify(t *testing.T) {
	// create the private key
	pk, err := NewEd25519PrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	// get the public key paired with the private key
	pubKey := pk.PublicKey()
	// create a random 100 byte message to sign
	msg := make([]byte, 100)
	if _, err = rand.Read(msg); err != nil {
		t.Fatal(err)
	}
	// sign the message using the private key
	signature := pk.Sign(msg)
	if !pubKey.VerifyBytes(msg, signature) {
		t.Fatal("verify bytes failed")
	}
	// create a new random 100 byte message
	msg = make([]byte, 100)
	if _, err = rand.Read(msg); err != nil {
		t.Fatal(err)
	}
	// ensure the verification fails
	if pubKey.VerifyBytes(msg, signature) {
		t.Fatal("verify bytes succeeded")
	}
}
