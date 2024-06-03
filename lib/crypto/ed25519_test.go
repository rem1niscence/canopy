package crypto

import (
	"crypto/rand"
	"testing"
)

func TestED25519Bytes(t *testing.T) {
	for i := 0; i < 1000; i++ {
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
}

func TestED25519SignAndVerify(t *testing.T) {
	for i := 0; i < 1000; i++ {
		pk, err := NewEd25519PrivateKey()
		if err != nil {
			t.Fatal(err)
		}
		pubKey := pk.PublicKey()
		msg := make([]byte, 100)
		if _, err = rand.Read(msg); err != nil {
			t.Fatal(err)
		}
		signature := pk.Sign(msg)
		if !pubKey.VerifyBytes(msg, signature) {
			t.Fatal("verify bytes failed")
		}
		msg = make([]byte, 100)
		if _, err = rand.Read(msg); err != nil {
			t.Fatal(err)
		}
		if pubKey.VerifyBytes(msg, signature) {
			t.Fatal("verify bytes succeeded")
		}
	}
}
