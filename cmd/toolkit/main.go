package main

import (
	"github.com/alecthomas/kong"

	"github.com/kxue43/cli-toolkit/scaffold"
	"github.com/kxue43/cli-toolkit/version"
)

func main() {
	var cli struct {
		StartTsCdkProject  scaffold.TsCdkProjectCmd  `cmd:"" name:"start-ts-cdk-project" help:"Start a TypeScript CDK project in the current directory."`
		StartPythonProject scaffold.PythonProjectCmd `cmd:"" name:"start-python-project" help:"Start a Python project in the current directory."`
		StartGoProject     scaffold.GoProjectCmd     `cmd:"" name:"start-go-project" help:"Start a Go project in the current directory."`
		Version            kong.VersionFlag          `name:"version" short:"v" help:"Show version information and quit."`
	}

	ctx := kong.Parse(
		&cli,
		kong.Name("toolkit"),
		kong.Description("Create starter files for Go, Python or AWS CDK projects."),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{Compact: true}),
		kong.Vars{"version": version.FromBuildInfo()},
	)

	err := ctx.Run()

	ctx.FatalIfErrorf(err)
}
