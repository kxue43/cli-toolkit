package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"

	"github.com/kxue43/cli-toolkit/auth"
)

var (
	cmd = auth.AssumeRoleCmd{}

	helpMsg = `Usage: %s -mfa-serial=STRING -profile=STRING [flags] <RoleArn>

Run AWS CLI credential process by assuming a role.

Arguments:
  <RoleArn>    ARN of the IAM role to assume.

Flags:
`
)

func registerFlagsAndHelp() {
	flag.StringVar(&cmd.MFASerial, "mfa-serial", "", "ARN of the virtual MFA to use when assuming the role.")
	flag.StringVar(&cmd.Profile, "profile", "", "Source profile used for assuming the role.")
	flag.StringVar(&cmd.Region, "region", "us-east-1", "The regional STS service endpoint to call.")
	flag.StringVar(&cmd.RoleSessionName, "role-session-name", "ToolkitCLI", "Role session name.")
	flag.Int64Var(&cmd.DurationSeconds, "duration-seconds", 3600, "Role session duration seconds.")

	flag.Usage = func() {
		_, _ = fmt.Fprintf(flag.CommandLine.Output(), helpMsg, os.Args[0])

		flag.PrintDefaults()
	}
}

func main() {
	exitCode := 0

	defer func() { os.Exit(exitCode) }()

	registerFlagsAndHelp()

	flag.Parse()

	ttyDevice, err := os.OpenFile("/dev/tty", os.O_RDWR|os.O_SYNC, 0600)
	if err != nil {
		exitCode = 1

		return
	}

	defer func() { _ = ttyDevice.Close() }()

	tty := auth.NewTTY(ttyDevice, "toolkit-assume-role: ", 0)
	defer func() {
		if tty.FlushLogs() != nil {
			exitCode = 1
		}
	}()

	ctx := context.Background()

	err = cmd.ValidateInputs(flag.Args())
	if err != nil {
		tty.Print(err.Error())

		exitCode = 1

		return
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile(cmd.Profile), config.WithRegion(cmd.Region))
	if err != nil {
		tty.Printf("failed to load AWS SDK configuration: %s\n", err)

		exitCode = 1

		return
	}

	err = cmd.Init(tty, cfg)
	if err != nil {
		tty.Print(err.Error())
	}

	err = cmd.Run(ctx, os.Stdout)
	if err != nil {
		tty.Print(err.Error())

		exitCode = 1

		return
	}
}
