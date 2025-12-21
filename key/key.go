package key

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"github.com/zalando/go-keyring"
)

type (
	KeyringProvider struct {
		service string
		user    string
	}
)

func NewKeyringProvider(service, user string) KeyringProvider {
	return KeyringProvider{service: service, user: user}
}

func (p KeyringProvider) Write(key []byte) (err error) {
	var encoded string

	encoded, err = keyring.Get(p.service, p.user)
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("failed to retrieve encryption key: secret exists but cannot be read: %s", err.Error())
	} else if errors.Is(err, keyring.ErrNotFound) {
		internal := make([]byte, len(key))

		encoded, err = generateKey(internal)
		if err != nil {
			return err
		}

		err = keyring.Set(p.service, p.user, encoded)
		if err != nil {
			return fmt.Errorf("failed to save newly generated encryption key: %s", err.Error())
		}

		copy(key, internal)

		return nil
	}

	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return fmt.Errorf("failed to base64 decode saved encryption key: %s", err.Error())
	} else if len(decoded) != len(key) {
		return fmt.Errorf("saved encryption key has length %d while the input byte slice has length %d", len(decoded), len(key))
	}

	copy(key, decoded)

	return nil
}

func generateKey(key []byte) (string, error) {
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return "", fmt.Errorf("failed to generate encryption key: %s", err.Error())
	}

	return base64.StdEncoding.EncodeToString(key), nil
}
