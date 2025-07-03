package main

import (
	"io"
	"log"
	"os"

	"github.com/alecthomas/kong"

	"github.com/kxue43/cli-toolkit/auth"
)

func main() {
	var cli struct {
		AssumeRole auth.AssumeRoleCmd `cmd:"" name:"assume-role" help:"Run AWS CLI credential process by assuming a role."`
	}

	tty, err := os.OpenFile("/dev/tty", os.O_RDWR|os.O_SYNC, 0600)
	if err != nil {
		log.Fatalf("failed to open /dev/tty: %v", err)
	}

	defer func() {
		err = tty.Close()
		if err != nil {
			log.Fatalf("error closing /dev/tty: %v", err)
		}
	}()

	ctx := kong.Parse(
		&cli,
		kong.Name("toolkit"),
		kong.Description("Personal CLI toolkit."),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{Compact: true}),
		kong.BindTo(tty, (*io.ReadWriter)(nil)),
		kong.BindTo(os.Stdout, (*io.Writer)(nil)),
	)

	err = ctx.Run()

	ctx.FatalIfErrorf(err)
}
