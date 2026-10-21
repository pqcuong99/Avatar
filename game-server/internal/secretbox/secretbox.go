// Package secretbox encrypts small secrets (TOTP seeds) at rest with
// AES-256-GCM. The key never lives in the database (docs/09 §2).
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
)

type Box struct{ aead cipher.AEAD }

// New takes a base64 key of 32 bytes.
func New(b64 string) (*Box, error) {
	key, err := base64.StdEncoding.DecodeString(b64)
	if err != nil || len(key) != 32 {
		return nil, errors.New("secretbox: key must be 32 bytes, base64-encoded")
	}
	return fromKey(key)
}

// DevBox derives a fixed key for THANH_ENV=local only; secrets encrypted with
// it are not protected.
func DevBox() *Box {
	k := sha256.Sum256([]byte("project-thanh local development only"))
	b, _ := fromKey(k[:])
	return b
}

func fromKey(key []byte) (*Box, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

func (b *Box) Seal(plain []byte) []byte {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		panic(err)
	}
	return b.aead.Seal(nonce, nonce, plain, nil)
}

func (b *Box) Open(sealed []byte) ([]byte, error) {
	n := b.aead.NonceSize()
	if len(sealed) < n {
		return nil, fmt.Errorf("secretbox: ciphertext too short")
	}
	return b.aead.Open(nil, sealed[:n], sealed[n:], nil)
}
