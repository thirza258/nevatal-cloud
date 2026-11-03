// Package secret encrypts small secrets at rest with AES-256-GCM. The key is
// derived from the deployment's JWT secret, so losing or rotating that secret
// invalidates stored secrets (they must then be reset by their owners).
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

const keyContext = "nevatal-cloud/secrets/v1:"

// Cipher encrypts and decrypts short strings.
type Cipher struct {
	aead cipher.AEAD
}

// New returns a cipher keyed from master (the deployment JWT secret). An
// empty master yields a disabled cipher whose Encrypt fails.
func New(master string) *Cipher {
	if master == "" {
		return &Cipher{}
	}
	sum := sha256.Sum256([]byte(keyContext + master))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return &Cipher{}
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return &Cipher{}
	}
	return &Cipher{aead: aead}
}

// Enabled reports whether the cipher has a key.
func (c *Cipher) Enabled() bool { return c != nil && c.aead != nil }

// Encrypt returns base64(nonce || ciphertext).
func (c *Cipher) Encrypt(plain string) (string, error) {
	if !c.Enabled() {
		return "", errors.New("secret encryption is not configured")
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("secret nonce: %w", err)
	}
	out := c.aead.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(out), nil
}

// Decrypt reverses Encrypt.
func (c *Cipher) Decrypt(encoded string) (string, error) {
	if !c.Enabled() {
		return "", errors.New("secret encryption is not configured")
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("secret encoding: %w", err)
	}
	if len(raw) < c.aead.NonceSize() {
		return "", errors.New("secret is truncated")
	}
	nonce, ct := raw[:c.aead.NonceSize()], raw[c.aead.NonceSize():]
	plain, err := c.aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", errors.New("secret cannot be decrypted with this key")
	}
	return string(plain), nil
}
