package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"golang.org/x/crypto/argon2"
	"os"
	"path/filepath"
)

const (
	KeyStoreName = "keystore.json"
)

type Keystore map[string]*EncryptedPrivateKey // address -> EncryptedPrivateKey

func NewInMemory() *Keystore {
	ks := make(Keystore)
	return &ks
}

func NewKeystoreFromFile(dataDirPath string) (*Keystore, error) {
	path := filepath.Join(dataDirPath, KeyStoreName)
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return NewInMemory(), nil
	}
	ksBz, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	ks := new(Keystore)
	return ks, json.Unmarshal(ksBz, ks)
}

func (ks *Keystore) AddKey(privateKey PrivateKeyI, password string) error {
	encrypted, err := EncryptPrivateKey(privateKey.Bytes(), []byte(password))
	if err != nil {
		return err
	}
	(*ks)[privateKey.PublicKey().Address().String()] = encrypted
	return nil
}

func (ks *Keystore) GetKey(address, password string) (PrivateKeyI, error) {
	return DecryptPrivateKey((*ks)[address], []byte(password))
}

func (ks *Keystore) GetPublicKey(address, password string) (PublicKeyI, error) {
	private, err := ks.GetKey(address, password)
	if err != nil {
		return nil, err
	}
	return private.PublicKey(), nil
}

func (ks *Keystore) DeleteKey(address string) {
	delete(*ks, address)
}

func (ks *Keystore) SaveToFile(dataDirPath string) error {
	bz, err := json.MarshalIndent(ks, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dataDirPath, KeyStoreName), bz, os.ModePerm)
}

type EncryptedPrivateKey struct {
	Salt      string `json:"salt"`
	Encrypted string `json:"encrypted"`
}

func EncryptPrivateKey(privateKey, password []byte) (*EncryptedPrivateKey, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	gcm, nonce, err := kdf(password, salt)
	if err != nil {
		return nil, err
	}
	return &EncryptedPrivateKey{
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
