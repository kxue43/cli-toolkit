package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/kxue43/cli-toolkit/auth"
)

var (
	logger          = log.New(os.Stderr, "toolkit-assume-role: ", 0)
	roleArn         string
	mfaSerial       string
	profile         string
	region          string
	roleSessionName string
	durationSeconds int
	helpMsg         = `Usage: %s -mfa-serial=STRING -profile=STRING [flags] <RoleArn>

Run AWS CLI credential process by assuming a role.

Arguments:
  <RoleArn>    ARN of the IAM role to assume.

Flags:
`
)

func registerFlagsAndHelp() {
	flag.StringVar(&mfaSerial, "mfa-serial", "", "ARN of the virtual MFA to use when assuming the role.")
	flag.StringVar(&profile, "profile", "", "Source profile used for assuming the role.")
	flag.StringVar(&region, "region", "us-east-1", "The regional STS service endpoint to call.")
	flag.StringVar(&roleSessionName, "role-session-name", "ToolkitCLI", "Role session name.")
	flag.IntVar(&durationSeconds, "duration-seconds", 3600, "Role session duration seconds.")

	flag.Usage = func() {
		_, _ = fmt.Fprintf(flag.CommandLine.Output(), helpMsg, os.Args[0])

		flag.PrintDefaults()
	}
}

func validateInputs() {
	if mfaSerial == "" {
		logger.Fatalf("-mfa-serial is required.")
	}

	if profile == "" {
		logger.Fatalf("-profile is required.")
	}

	if durationSeconds > 14400 {
		logger.Fatal("-duration-seconds cannot exceed 14400, i.e. 4 hours.")
	}

	args := flag.Args()
	if len(args) == 0 {
		logger.Fatalf("The <RoleArn> argument is required.")
	}

	roleArn = args[0]
}

func main() {
	registerFlagsAndHelp()

	flag.Parse()

	validateInputs()

	tty, err := os.OpenFile("/dev/tty", os.O_RDWR|os.O_SYNC, 0600)
	if err != nil {
		logger.Fatalf("failed to open /dev/tty: %v", err)
	}

	cmd := auth.AssumeRoleCmd{
		RoleArn:         roleArn,
		MFASerial:       mfaSerial,
		Profile:         profile,
		Region:          region,
		RoleSessionName: roleSessionName,
		DurationSeconds: int32(durationSeconds),
	}

	err = cmd.AfterApply(tty)
	if err != nil {
		logger.Fatal(err.Error())
	}

	err = cmd.Run(os.Stdout)
	if err != nil {
		logger.Fatal(err.Error())
	}

	defer func() { _ = tty.Close() }()
}
