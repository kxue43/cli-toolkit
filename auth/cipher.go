package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"github.com/zalando/go-keyring"
)

type (
	KeyFunc func() ([]byte, error)

	Cipher struct {
		key []byte
	}
)

const (
	// 32 bytes for AES-256
	keySize = 32

	service = "kxue43.toolkit.assume-role"
	user    = "cache-encryption-key"
)

var (
	ErrCipher = errors.New("cipher failure")

	fromKeyring KeyFunc = keyringGet
)

func generateKey(size int) (key []byte, encoded string, err error) {
	key = make([]byte, size)

	if _, err = io.ReadFull(rand.Reader, key); err != nil {
		return nil, "", fmt.Errorf("%w: failed to generate encryption key: %s", ErrCipher, err.Error())
	}

	return key, base64.StdEncoding.EncodeToString(key), nil
}

func keyringGet() (key []byte, err error) {
	var secret string

	secret, err = keyring.Get(service, user)
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return nil, fmt.Errorf("%w: failed to retrieve encryption key: secret exists but cannot be read: %s", ErrCipher, err.Error())
	} else if errors.Is(err, keyring.ErrNotFound) {
		key, secret, err = generateKey(keySize)
		if err != nil {
			return nil, err
		}

		err = keyring.Set(service, user, secret)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to save newly generated encryption key: %s", ErrCipher, err.Error())
		}

		return key, nil
	}

	key, err = base64.StdEncoding.DecodeString(secret)
	if err != nil {
		return nil, fmt.Errorf("%w: saved encryption key has been corrupted: %s", ErrCipher, err.Error())
	}

	return key, nil
}

func NewCipher(fn KeyFunc) (Cipher, error) {
	key, err := fn()
	if err != nil {
		return Cipher{}, fmt.Errorf("%w: failed to get encryption key: %s", ErrCipher, err.Error())
	}

	return Cipher{key: key}, nil
}

func (c Cipher) Encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.key)
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

func (c Cipher) Decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.key)
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
