package main

import (
	"github.com/alecthomas/kong"
	"github.com/kxue43/cli-toolkit/auth"
)

func main() {
	var cli struct {
		AssumeRole auth.AssumeRoleCmd `cmd:"" name:"assume-role" help:"Run AWS CLI credential process by assuming a role."`
	}

	ctx := kong.Parse(
		&cli,
		kong.Name("toolkit"),
		kong.Description("Personal CLI toolkit."),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{Compact: true}),
	)
	err := ctx.Run()
	ctx.FatalIfErrorf(err)
}
