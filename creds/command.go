// Package creds implements AWS credential process with caching.
// Cache files are saved on disk and encrypted via AES-GCM with the encryption key stored in the operating system's "native" credentials store.
// For example, Keychain is used on macOS.
package creds

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/kxue43/cli-toolkit/terminal"
)

type (
	ProcessInput struct {
		RoleArn         string
		MFASerial       string
		Profile         string
		Region          string
		RoleSessionName string
		DurationSeconds int64
	}

	ProcessOutput struct {
		AccessKeyId     string `json:"AccessKeyId"`
		SecretAccessKey string `json:"SecretAccessKey"`
		SessionToken    string `json:"SessionToken"`
		Expiration      string `json:"Expiration"`
		Version         int    `json:"Version"`
	}

	Processor struct {
		logger    logger
		cacher    *cacher
		retriever *stscreds.AssumeRoleProvider
		roleArn   string
	}

	logger interface {
		Printf(string, ...any)
		Println(...any)
	}

	KeyProvider interface {
		Write([]byte) error
	}

	mfaPrompter struct {
		io.ReadWriter
	}
)

func (c mfaPrompter) token() (code string, err error) {
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

func NewProcessor(input ProcessInput, tty *terminal.TTY, cfg aws.Config, kp KeyProvider) *Processor {
	var err error

	p := Processor{}

	prompter := mfaPrompter{ReadWriter: tty}

	p.logger = tty

	p.cacher, err = newCacher(p.logger, kp)
	if err != nil {
		p.logger.Println(err.Error())
	}

	p.retriever = stscreds.NewAssumeRoleProvider(sts.NewFromConfig(cfg), input.RoleArn, func(o *stscreds.AssumeRoleOptions) {
		o.RoleSessionName = input.RoleSessionName
		o.Duration = time.Second * time.Duration(input.DurationSeconds)
		o.SerialNumber = aws.String(input.MFASerial)
		o.TokenProvider = prompter.token
	})

	p.roleArn = input.RoleArn

	return &p
}

// Non-nil returned error means failure.
func (a *Processor) Run(ctx context.Context, dest io.Writer) (err error) {
	// Output of the AWS CLI credential process.
	var output []byte

	if a.cacher != nil {
		output = a.cacher.retrieve(a.roleArn)
		if output != nil {
			_, err = dest.Write(output)
			if err != nil {
				return fmt.Errorf("failed to write credentials to destination: %s", err.Error())
			}

			return
		}
	}

	stsCreds, err := a.retriever.Retrieve(ctx)
	if err != nil {
		return fmt.Errorf("failed to retrieve STS credentials: %s", err.Error())
	}

	// structured output
	soutput := ProcessOutput{
		AccessKeyId:     stsCreds.AccessKeyID,
		SecretAccessKey: stsCreds.SecretAccessKey,
		SessionToken:    stsCreds.SessionToken,
		Expiration:      stsCreds.Expires.Format(time.RFC3339),
		Version:         1,
	}

	if a.cacher != nil {
		output, err = a.cacher.save(a.roleArn, &soutput)
		if errors.Is(err, ErrInvalidCredential) {
			return err
		} else if err != nil {
			a.logger.Println(err.Error())
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
