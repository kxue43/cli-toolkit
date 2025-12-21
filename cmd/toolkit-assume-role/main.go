package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"

	"github.com/kxue43/cli-toolkit/creds"
	"github.com/kxue43/cli-toolkit/key"
	"github.com/kxue43/cli-toolkit/terminal"
)

var (
	input = creds.ProcessInput{}

	helpMsg = `Usage: %s -mfa-serial=STRING -profile=STRING [flags] <RoleArn>

Run AWS CLI credential process by assuming a role.

Arguments:
  <RoleArn>    ARN of the IAM role to assume.

Flags:
`
)

func registerFlagsAndHelp() {
	flag.StringVar(&input.MFASerial, "mfa-serial", "", "ARN of the virtual MFA to use when assuming the role.")
	flag.StringVar(&input.Profile, "profile", "", "Source profile used for assuming the role.")
	flag.StringVar(&input.Region, "region", "us-east-1", "The regional STS service endpoint to call.")
	flag.StringVar(&input.RoleSessionName, "role-session-name", "ToolkitCLI", "Role session name.")
	flag.Int64Var(&input.DurationSeconds, "duration-seconds", 3600, "Role session duration seconds.")

	flag.Usage = func() {
		_, _ = fmt.Fprintf(flag.CommandLine.Output(), helpMsg, os.Args[0])

		flag.PrintDefaults()
	}
}

func validateInput(input creds.ProcessInput) error {
	if input.MFASerial == "" {
		return errors.New("-mfa-serial is required")
	}

	if input.Profile == "" {
		return errors.New("-profile is required")
	}

	if input.DurationSeconds > 14400 {
		return errors.New("-duration-seconds cannot exceed 14400, i.e. 4 hours")
	}

	if input.RoleArn == "" {
		return errors.New("the <RoleArn> argument is required")
	}

	return nil
}

func main() {
	exitCode := 0

	defer func() { os.Exit(exitCode) }()

	device, err := os.OpenFile("/dev/tty", os.O_RDWR|os.O_SYNC, 0600)
	if err != nil {
		exitCode = 1

		return
	}

	defer func() { _ = device.Close() }()

	tty := terminal.NewTTY(device, "toolkit-assume-role: ", 0)
	defer func() {
		if tty.FlushLogs() != nil {
			exitCode = 1
		}
	}()

	registerFlagsAndHelp()

	flag.Parse()

	if args := flag.Args(); len(args) > 0 {
		input.RoleArn = args[0]
	}

	err = validateInput(input)
	if err != nil {
		tty.Println(err.Error())

		exitCode = 1

		return
	}

	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile(input.Profile), config.WithRegion(input.Region))
	if err != nil {
		tty.Printf("failed to load AWS SDK configuration: %s\n", err)

		exitCode = 1

		return
	}

	kp := key.NewKeyringProvider("kxue43.toolkit.assume-role", "cache-encryption-key")

	processor := creds.NewProcessor(input, tty, cfg, kp)

	err = processor.Run(ctx, os.Stdout)
	if err != nil {
		tty.Println(err.Error())

		exitCode = 1

		return
	}
}
