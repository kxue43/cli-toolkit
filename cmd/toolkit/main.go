package main

import (
	"io"
	"log"
	"os"

	"github.com/alecthomas/kong"

	"github.com/kxue43/cli-toolkit/auth"
	"github.com/kxue43/cli-toolkit/scaffold"
)

type LazyTTY struct {
	tty *os.File
}

func (l *LazyTTY) Init() {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR|os.O_SYNC, 0600)
	if err != nil {
		log.Fatalf("failed to open /dev/tty: %v", err)
	}

	l.tty = tty
}

func (l *LazyTTY) Read(p []byte) (n int, err error) {
	if l.tty == nil {
		l.Init()
	}

	return l.tty.Read(p)
}

func (l *LazyTTY) Write(p []byte) (n int, err error) {
	if l.tty == nil {
		l.Init()
	}

	return l.tty.Write(p)
}

func (l *LazyTTY) Close() {
	if l.tty != nil {
		if err := l.tty.Close(); err != nil {
			log.Fatalf("error closing /dev/tty: %v", err)
		}
	}
}

func main() {
	var cli struct {
		StartGoProject     scaffold.GoProjectCmd     `cmd:"" name:"start-go-project" help:"Start a Go project in the current directory."`
		StartPythonProject scaffold.PythonProjectCmd `cmd:"" name:"start-python-project" help:"Start a Python project in the current directory."`
		StartTsCdkProject  scaffold.TsCdkProjectCmd  `cmd:"" name:"start-ts-cdk-project" help:"Start a TypeScript CDK project in the current directory."`
		AssumeRole         auth.AssumeRoleCmd        `cmd:"" name:"assume-role" help:"Run AWS CLI credential process by assuming a role."`
	}

	tty := &LazyTTY{}
	defer tty.Close()

	ctx := kong.Parse(
		&cli,
		kong.Name("toolkit"),
		kong.Description("Personal CLI toolkit."),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{Compact: true}),
		kong.BindTo(tty, (*io.ReadWriter)(nil)),
		kong.BindTo(os.Stdout, (*io.Writer)(nil)),
	)

	err := ctx.Run()

	ctx.FatalIfErrorf(err)
}
