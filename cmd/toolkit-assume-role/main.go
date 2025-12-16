package main

import (
	"context"
	"flag"
	"fmt"
	"os"

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

		tty, err := os.OpenFile("/dev/tty", os.O_RDWR|os.O_SYNC, 0600)
		if err != nil {
			_, _ = fmt.Fprintf(flag.CommandLine.Output(), "\nNot ready to run as credential process: cannot open /dev/tty: %s\n", err)

			os.Exit(1)
		}

		defer func() { _ = tty.Close() }()

		_, _ = fmt.Fprint(flag.CommandLine.Output(), "\nReady to run as credential process!\n")
	}
}

func main() {
	registerFlagsAndHelp()

	flag.Parse()

	tty, err := os.OpenFile("/dev/tty", os.O_RDWR|os.O_SYNC, 0600)
	if err != nil {
		os.Exit(1)
	}

	defer func() { _ = tty.Close() }()

	ctx := context.Background()

	cmd.Init(ctx, tty)

	cmd.Run(ctx, os.Stdout)
}
