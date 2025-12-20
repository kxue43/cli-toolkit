package cipher

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

type (
	AesKeyFunc func(*[AesKeySize]byte) error

	AesGcm struct {
		key [AesKeySize]byte
	}
)

const (
	AesKeySize = 32
)

var (
	ErrCipher = errors.New("cipher failure")
)

// Non-nil returned error wraps [ErrCipher].
func NewAesGcm(fn AesKeyFunc) (*AesGcm, error) {
	aes := AesGcm{}

	err := fn(&aes.key)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to get encryption key: %s", ErrCipher, err.Error())
	}

	return &aes, nil
}

// Non-nil returned error wraps [ErrCipher].
func (c *AesGcm) Encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.key[:])
	if err != nil {
		return nil, fmt.Errorf("%w: failed to initialize AES block cipher: %s", ErrCipher, err.Error())
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create GCM: %s", ErrCipher, err.Error())
	}

	// The GCM nonce size is fixed at 12 bytes.
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("%w: failed to initialize nonce: %s", ErrCipher, err.Error())
	}

	// The first return value is 'nonce + ciphertext + tag'.
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Non-nil returned error wraps [ErrCipher].
func (c *AesGcm) Decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.key[:])
	if err != nil {
		return nil, fmt.Errorf("%w: failed to initialize AES block cipher: %s", ErrCipher, err.Error())
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create GCM: %s", ErrCipher, err.Error())
	}

	// Extract nonce from the beginning of the ciphertext.
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("%w: ciphertext too short", ErrCipher)
	}

	nonce, ciphertextActual := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertextActual, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: AES-GCM authentication failure, the data have been tampered: %s", ErrCipher, err.Error())
	}

	return plaintext, nil
}
