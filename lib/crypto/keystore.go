package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/crypto/argon2"
	"os"
	"path/filepath"
)

const (
	KeyStoreName = "keystore.json"
)

type Keystore map[string]*EncryptedPrivateKey // address -> EncryptedPrivateKey

func NewKeystoreInMemory() *Keystore {
	ks := make(Keystore)
	return &ks
}

func NewKeystoreFromFile(dataDirPath string) (*Keystore, error) {
	path := filepath.Join(dataDirPath, KeyStoreName)
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return NewKeystoreInMemory(), nil
	}
	ksBz, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	ks := new(Keystore)
	return ks, json.Unmarshal(ksBz, ks)
}

func (ks *Keystore) Import(address []byte, encrypted *EncryptedPrivateKey) error {
	(*ks)[hex.EncodeToString(address)] = encrypted
	return nil
}

func (ks *Keystore) ImportRaw(privateKeyBytes []byte, password string) (address string, err error) {
	privateKey, err := NewPrivateKeyFromBytes(privateKeyBytes)
	if err != nil {
		return
	}
	publicKey := privateKey.PublicKey()
	encrypted, err := EncryptPrivateKey(publicKey.Bytes(), privateKeyBytes, []byte(password))
	if err != nil {
		return
	}
	address = publicKey.Address().String()
	(*ks)[address] = encrypted
	return
}

func (ks *Keystore) GetKey(address []byte, password string) (PrivateKeyI, error) {
	v, ok := (*ks)[hex.EncodeToString(address)]
	if !ok {
		return nil, fmt.Errorf("key not found")
	}
	return DecryptPrivateKey(v, []byte(password))
}

func (ks *Keystore) GetKeyGroup(address []byte, password string) (*KeyGroup, error) {
	v, ok := (*ks)[hex.EncodeToString(address)]
	if !ok {
		return nil, fmt.Errorf("key not found")
	}
	if password == "" {
		return nil, fmt.Errorf("invalid password")
	}
	pk, err := DecryptPrivateKey(v, []byte(password))
	if err != nil {
		return nil, err
	}
	return NewKeyGroup(pk), err
}

func (ks *Keystore) DeleteKey(address []byte) {
	delete(*ks, hex.EncodeToString(address))
}

func (ks *Keystore) SaveToFile(dataDirPath string) error {
	bz, err := json.MarshalIndent(ks, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dataDirPath, KeyStoreName), bz, os.ModePerm)
}

type EncryptedPrivateKey struct {
	PublicKey string `json:"publicKey"`
	Salt      string `json:"salt"`
	Encrypted string `json:"encrypted"`
}

func EncryptPrivateKey(publicKey, privateKey, password []byte) (*EncryptedPrivateKey, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	gcm, nonce, err := kdf(password, salt)
	if err != nil {
		return nil, err
	}
	return &EncryptedPrivateKey{
		PublicKey: hex.EncodeToString(publicKey),
		Salt:      hex.EncodeToString(salt),
		Encrypted: hex.EncodeToString(gcm.Seal(nil, nonce, privateKey, nil)),
	}, nil
}

func DecryptPrivateKey(epk *EncryptedPrivateKey, password []byte) (pk PrivateKeyI, err error) {
	salt, err := hex.DecodeString(epk.Salt)
	if err != nil {
		return nil, err
	}
	encrypted, err := hex.DecodeString(epk.Encrypted)
	if err != nil {
		return nil, err
	}
	gcm, nonce, err := kdf(password, salt)
	if err != nil {
		return nil, err
	}
	plainText, err := gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return nil, err
	}
	return NewPrivateKeyFromBytes(plainText)
}

func kdf(password, salt []byte) (gcm cipher.AEAD, nonce []byte, err error) {
	key := argon2.Key(password, salt, 3, 32*1024, 4, 32)
	block, err := aes.NewCipher(key)
	if err != nil {
		return
	}
	if gcm, err = cipher.NewGCM(block); err != nil {
		return
	}
	return gcm, key[:12], nil
}
