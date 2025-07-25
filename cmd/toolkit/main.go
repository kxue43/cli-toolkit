package main

import (
	"github.com/alecthomas/kong"

	"github.com/kxue43/cli-toolkit/scaffold"
)

func main() {
	var cli struct {
		StartGoProject     scaffold.GoProjectCmd     `cmd:"" name:"start-go-project" help:"Start a Go project in the current directory."`
		StartPythonProject scaffold.PythonProjectCmd `cmd:"" name:"start-python-project" help:"Start a Python project in the current directory."`
		StartTsCdkProject  scaffold.TsCdkProjectCmd  `cmd:"" name:"start-ts-cdk-project" help:"Start a TypeScript CDK project in the current directory."`
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
