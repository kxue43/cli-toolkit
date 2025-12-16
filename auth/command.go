package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

type (
	AssumeRoleCmd struct {
		prompter        *Prompter
		cacher          *Cacher
		client          *sts.Client
		RoleArn         string
		MFASerial       string
		Profile         string
		Region          string
		RoleSessionName string
		DurationSeconds int64
	}

	Prompter struct {
		*log.Logger
		io.ReadWriter
	}
)

func NewPrompter(tty io.ReadWriter, prefix string, flag int) *Prompter {
	return &Prompter{ReadWriter: tty, Logger: log.New(tty, prefix, flag)}
}

func (c *Prompter) MFAToken() (code string, err error) {
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

func (a *AssumeRoleCmd) validate(args []string, logger *log.Logger) {
	if a.MFASerial == "" {
		logger.Fatal("-mfa-serial is required.")
	}

	if a.Profile == "" {
		logger.Fatal("-profile is required.")
	}

	if a.DurationSeconds > 14400 {
		logger.Fatal("-duration-seconds cannot exceed 14400, i.e. 4 hours.")
	}

	if len(args) == 0 {
		logger.Fatal("The <RoleArn> argument is required.")
	}

	a.RoleArn = args[0]
}

func (a *AssumeRoleCmd) Init(ctx context.Context, tty io.ReadWriter) {
	a.prompter = NewPrompter(tty, "toolkit-assume-role: ", 0)

	a.validate(flag.Args(), a.prompter.Logger)

	cfg, err := config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile(a.Profile), config.WithRegion(a.Region))
	if err != nil {
		a.prompter.Fatalf("failed to load AWS CLI/SDK configuration: %s\n", err)
	}

	a.client = sts.NewFromConfig(cfg)

	a.cacher = NewCacher(a.prompter.Logger)
	if a.cacher == nil {
		a.prompter.Printf("cache mode off: %s\n", err)
	}
}

func (a *AssumeRoleCmd) Run(ctx context.Context, dest io.Writer) {
	var err error

	// Output of the AWS CLI credential process.
	var output []byte

	if a.cacher != nil {
		output = a.cacher.Retrieve(a.RoleArn)
		if output != nil {
			_, err = dest.Write(output)
			if err != nil {
				a.prompter.Fatalf("failed to write credentials to destination: %s\n", err)
			}
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
		a.prompter.Fatalf("failed to retrieve STS credentials: %s\n", err)
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
			a.prompter.Fatal(err.Error())
		} else if err != nil {
			a.prompter.Print(err.Error())
		}

		_, err = dest.Write(output)
		if err != nil {
			a.prompter.Fatalf("failed to write credentials to destination: %s\n", err)
		}

		return
	}

	output, err = json.Marshal(&soutput)
	if err != nil {
		a.prompter.Fatalf("failed to marshal credential process output: %s\n", err)
	}

	_, err = dest.Write(output)
	if err != nil {
		a.prompter.Fatal(err.Error())
	}
}
