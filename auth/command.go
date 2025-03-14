package auth

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

type AssumeRoleCmd struct {
	RoleArn         string `arg:"" required:"" name:"RoleArn" help:"ARN of the IAM role to assume."`
	MFASerial       string `required:"" name:"mfa-serial" help:"ARN of the virtual MFA to use when assuming the role."`
	Profile         string `required:"" name:"profile" help:"Source profile used for assuming the role."`
	Region          string `name:"region" default:"us-east-1" help:"The regional STS service endpoint to call."`
	RoleSessionName string `name:"role-session-name" default:"ToolkitCLI" help:"Role session name."`
	DurationSeconds int    `name:"duration-seconds" default:"3600" help:"Role session duration seconds."`
}

func (a *AssumeRoleCmd) Run() error {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR|os.O_SYNC, 0666)
	if err != nil {
		return fmt.Errorf("failed to open /dev/tty file: %w", err)
	}
	defer tty.Close()
	logger := log.New(tty, "toolkit: ", 0)

	cache := NewCacheRetrieverSaver(logger)
	creds, got := cache.Retrieve(a.RoleArn)
	if got {
		_, err := os.Stdout.Write(creds)
		return err
	}

	config, err := config.LoadDefaultConfig(context.TODO(), config.WithSharedConfigProfile(a.Profile), config.WithRegion(a.Region))
	client := sts.NewFromConfig(config)
	provider := stscreds.NewAssumeRoleProvider(client, a.RoleArn, func(o *stscreds.AssumeRoleOptions) {
		o.RoleSessionName = a.RoleSessionName
		o.Duration = time.Second * time.Duration(a.DurationSeconds)
		o.SerialNumber = aws.String(a.MFASerial)
		o.TokenProvider = func() (string, error) {
			_, err := tty.WriteString("MFA code: ")
			if err != nil {
				return "", fmt.Errorf("failed to write to /dev/tty to prompt for MFA code: %w", err)
			}
			buf := make([]byte, 256)
			n, err := tty.Read(buf)
			if err != nil {
				return "", fmt.Errorf("failed to read MFA code from /dev/tty: %w", err)
			}
			return string(bytes.TrimSpace(buf[:n])), nil
		}
	})
	screds, err := provider.Retrieve(context.TODO())
	if err != nil {
		return fmt.Errorf("failed to retrieve STS credentials: %w", err)
	}
	output := CredentialProcessOutput{
		Version:         1,
		AccessKeyId:     screds.AccessKeyID,
		SecretAccessKey: screds.SecretAccessKey,
		SessionToken:    screds.SessionToken,
		Expiration:      screds.Expires.Format(expirationLayout),
	}
	contents := cache.Save(a.RoleArn, &output)
	_, err = os.Stdout.Write(contents)
	return err
}
