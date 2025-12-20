// Package creds implements AWS credential process with caching.
// Cache files are saved on disk and encrypted via AES-GCM with the encryption key stored in the operating system's "native" credentials store.
// For example, Keychain is used on macOS.
package creds

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/zalando/go-keyring"

	"github.com/kxue43/cli-toolkit/cipher"
	"github.com/kxue43/cli-toolkit/terminal"
)

type (
	logger interface {
		Printf(string, ...any)
		Print(...any)
	}

	AssumeRoleCmd struct {
		logger          logger
		prompter        prompter
		cacher          *cacher
		client          *sts.Client
		RoleArn         string
		MFASerial       string
		Profile         string
		Region          string
		RoleSessionName string
		DurationSeconds int64
	}

	prompter struct {
		io.ReadWriter
	}
)

const (
	service = "kxue43.toolkit.assume-role"
	user    = "cache-encryption-key"
)

var (
	ErrInvalidInput = errors.New("invalid CLI input")

	fromKeyring cipher.AesKeyFunc = keyringGet
)

func generateKey(key *[cipher.AesKeySize]byte) (string, error) {
	if _, err := io.ReadFull(rand.Reader, key[:]); err != nil {
		return "", fmt.Errorf("failed to generate encryption key: %s", err.Error())
	}

	return base64.StdEncoding.EncodeToString(key[:]), nil
}

func keyringGet(key *[cipher.AesKeySize]byte) error {
	secret, err := keyring.Get(service, user)
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("failed to retrieve encryption key: secret exists but cannot be read: %s", err.Error())
	} else if errors.Is(err, keyring.ErrNotFound) {
		secret, err = generateKey(key)
		if err != nil {
			return err
		}

		err = keyring.Set(service, user, secret)
		if err != nil {
			return fmt.Errorf("failed to save newly generated encryption key: %s", err.Error())
		}

		return nil
	}

	decoded, err := base64.StdEncoding.DecodeString(secret)
	if err != nil || len(decoded) != len(key) {
		return fmt.Errorf("saved encryption key has been corrupted: %s", err.Error())
	}

	copy(key[:], decoded)

	return nil
}

func (c prompter) MFAToken() (code string, err error) {
	_, err = io.WriteString(c, "MFA code: ")
	if err != nil {
		return "", fmt.Errorf("failed to prompt for MFA code: %w", err)
	}

	buf := make([]byte, 256)

	n, err := c.Read(buf)
	if err != nil {
		return "", fmt.Errorf("failed to read MFA code from user input: %w", err)
	}

	return string(bytes.TrimSpace(buf[:n])), nil
}

// Non-nil returned error wraps [ErrInvalidInput].
func (a *AssumeRoleCmd) ValidateInputs(args []string) error {
	if a.MFASerial == "" {
		return fmt.Errorf("%w: -mfa-serial is required", ErrInvalidInput)
	}

	if a.Profile == "" {
		return fmt.Errorf("%w: -profile is required", ErrInvalidInput)
	}

	if a.DurationSeconds > 14400 {
		return fmt.Errorf("%w: -duration-seconds cannot exceed 14400, i.e. 4 hours", ErrInvalidInput)
	}

	if len(args) == 0 {
		return fmt.Errorf("%w: the <RoleArn> argument is required", ErrInvalidInput)
	}

	a.RoleArn = args[0]

	return nil
}

// Non-nil returned error wraps [ErrCacheInit].
func (a *AssumeRoleCmd) Init(tty *terminal.TTY, cfg aws.Config) (err error) {
	a.prompter = prompter{ReadWriter: tty}

	a.logger = tty

	a.client = sts.NewFromConfig(cfg)

	a.cacher, err = newCacher(a.logger, fromKeyring)

	return err
}

// Non-nil returned error means failure.
func (a *AssumeRoleCmd) Run(ctx context.Context, dest io.Writer) (err error) {
	// Output of the AWS CLI credential process.
	var output []byte

	if a.cacher != nil {
		output = a.cacher.Retrieve(a.RoleArn)
		if output != nil {
			_, err = dest.Write(output)
			if err != nil {
				return fmt.Errorf("failed to write credentials to destination: %s", err.Error())
			}

			return
		}
	}

	provider := stscreds.NewAssumeRoleProvider(a.client, a.RoleArn, func(o *stscreds.AssumeRoleOptions) {
		o.RoleSessionName = a.RoleSessionName
		o.Duration = time.Second * time.Duration(a.DurationSeconds)
		o.SerialNumber = aws.String(a.MFASerial)
		o.TokenProvider = a.prompter.MFAToken
	})

	stsCreds, err := provider.Retrieve(ctx)
	if err != nil {
		return fmt.Errorf("failed to retrieve STS credentials: %s", err.Error())
	}

	// structured output
	soutput := CredentialProcessOutput{
		AccessKeyId:     stsCreds.AccessKeyID,
		SecretAccessKey: stsCreds.SecretAccessKey,
		SessionToken:    stsCreds.SessionToken,
		Expiration:      stsCreds.Expires.Format(time.RFC3339),
		Version:         1,
	}

	if a.cacher != nil {
		output, err = a.cacher.Save(a.RoleArn, &soutput)
		if errors.Is(err, ErrInvalidCredential) {
			return err
		} else if err != nil {
			a.logger.Print(err.Error())
		}
	}

	if output == nil {
		output, err = json.Marshal(&soutput)
		if err != nil {
			return fmt.Errorf("failed to marshal credential process output: %s", err.Error())
		}
	}

	_, err = dest.Write(output)
	if err != nil {
		return fmt.Errorf("failed to write credentials to destination: %s", err.Error())
	}

	return nil
}
