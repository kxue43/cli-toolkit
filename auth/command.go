package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
		ci              ClientInteractor
		cache           *cacheSaveRetriever
		stsClient       *sts.Client
		RoleArn         string `arg:"" required:"" name:"RoleArn" help:"ARN of the IAM role to assume."`
		MFASerial       string `required:"" name:"mfa-serial" help:"ARN of the virtual MFA to use when assuming the role."`
		Profile         string `required:"" name:"profile" help:"Source profile used for assuming the role."`
		Region          string `name:"region" default:"us-east-1" help:"The regional STS service endpoint to call."`
		RoleSessionName string `name:"role-session-name" default:"ToolkitCLI" help:"Role session name."`
		DurationSeconds int32  `name:"duration-seconds" default:"3600" help:"Role session duration seconds."`
		cacheModeOff    bool
	}

	ClientInteractor struct {
		io.ReadWriter
	}
)

func (c *ClientInteractor) PromptMFAToken() (code string, err error) {
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

func (c *ClientInteractor) NewLogger(prefix string, flag int) *log.Logger {
	return log.New(c.ReadWriter, prefix, flag)
}

func (a *AssumeRoleCmd) AfterApply(fd io.ReadWriter) error {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithSharedConfigProfile(a.Profile), config.WithRegion(a.Region))
	if err != nil {
		return fmt.Errorf("failed to load AWS CLI/SDK configuration: %w", err)
	}

	a.stsClient = sts.NewFromConfig(cfg)

	a.ci = ClientInteractor{fd}
	logger := a.ci.NewLogger("toolkit: ", 0)

	a.cache, err = NewCacheSaveRetriever(logger)
	if err != nil {
		a.cacheModeOff = true

		logger.Printf("cache mode off: %s\n", err)
	}

	return nil
}

func (a *AssumeRoleCmd) Run(dest io.Writer) (err error) {
	// Output of the AWS CLI credential process.
	var output []byte

	if !a.cacheModeOff {
		output, err = a.cache.Retrieve(a.RoleArn)
		if err == nil && output != nil {
			_, err = dest.Write(output)

			return err
		}
	}

	provider := stscreds.NewAssumeRoleProvider(a.stsClient, a.RoleArn, func(o *stscreds.AssumeRoleOptions) {
		o.RoleSessionName = a.RoleSessionName
		o.Duration = time.Second * time.Duration(a.DurationSeconds)
		o.SerialNumber = aws.String(a.MFASerial)
		o.TokenProvider = a.ci.PromptMFAToken
	})

	stsCreds, err := provider.Retrieve(context.TODO())
	if err != nil {
		return fmt.Errorf("failed to retrieve STS credentials: %w", err)
	}

	// structured output
	soutput := CredentialProcessOutput{
		AccessKeyId:     stsCreds.AccessKeyID,
		SecretAccessKey: stsCreds.SecretAccessKey,
		SessionToken:    stsCreds.SessionToken,
		Expiration:      stsCreds.Expires.Format(expirationLayout),
		Version:         1,
	}

	if !a.cacheModeOff {
		output, err = a.cache.Save(a.RoleArn, &soutput)
		if errors.Is(err, ErrInvalidCredential) {
			return err
		}

		_, err = dest.Write(output)

		return err
	}

	output, err = json.Marshal(&soutput)
	if err != nil {
		return fmt.Errorf("failed to marshal credential process output: %w", err)
	}

	_, err = dest.Write(output)

	return err
}
