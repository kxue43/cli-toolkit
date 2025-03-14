package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"regexp"

	"github.com/alecthomas/kong"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

//go:embed github_style.html
var template []byte

type CLI struct {
	InputPath  string `arg:"" help:"Absolute path to the input Markdown file."`
	OutputPath string `arg:"" help:"Absolute path to the output HTML files."`
}

func (c *CLI) Run() error {
	source, err := os.ReadFile(c.InputPath)
	if err != nil {
		return fmt.Errorf("failed to read input Markdown file at %q: %w", c.InputPath, err)
	}
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithXHTML(),
		),
	)
	var buf bytes.Buffer
	if err := md.Convert(source, &buf); err != nil {
		return fmt.Errorf("failed to convert Markdown file to HTML: %w", err)
	}
	regex := regexp.MustCompile("BODY_PLACEHOLDER")
	contents := regex.ReplaceAll(template, buf.Bytes())
	err = os.WriteFile(c.OutputPath, contents, 0644)
	return err
}

func main() {
	var cli CLI

	ctx := kong.Parse(
		&cli,
		kong.Name("mdmak"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{Compact: true}),
	)
	err := ctx.Run()
	ctx.FatalIfErrorf(err)
}
