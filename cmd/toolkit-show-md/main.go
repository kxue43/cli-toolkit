package main

import (
	"bytes"
	_ "embed"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"text/template"

	"github.com/pkg/browser"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

var (
	logger = log.New(os.Stderr, "toolkit-show-md: ", 0)

	helpMsg = `Usage: %s  <PATH>

Convert Markdown file to GitHub style HTML and display HTML in the default browser.

Arguments:
  <PATH>    Path to the Markdown file to convert.
`
	//go:embed .github.style.tmplt
	gitHubMarkdownTemplate []byte
)

func main() {
	var stat os.FileInfo

	var err error

	flag.Usage = func() {
		_, _ = fmt.Fprintf(flag.CommandLine.Output(), helpMsg, os.Args[0])
	}

	flag.Parse()

	args := flag.Args()
	if len(args) != 1 {
		flag.Usage()
	}

	inpath := filepath.Clean(args[0])

	if stat, err = os.Stat(inpath); os.IsNotExist(err) || stat.IsDir() {
		logger.Fatalf("The input path %q doesn't exist or is a directory.", inpath)
	}

	mdContents, err := os.ReadFile(filepath.Clean(inpath))
	if err != nil {
		logger.Fatalf("Failed to read the contents of the input file at %q: %s", inpath, err)
	}

	tmplt, err := template.New("github.style").Parse(string(gitHubMarkdownTemplate))
	if err != nil {
		logger.Fatalf("Failed to load template file: %s", err)
	}

	md := goldmark.New(goldmark.WithExtensions(extension.GFM))

	var html, out bytes.Buffer

	err = md.Convert(mdContents, &html)
	if err != nil {
		logger.Fatalf("Encountered error while converting Markdown to HTML: %s", err)
	}

	err = tmplt.Execute(&out, html.String())
	if err != nil {
		logger.Fatalf("Failed to insert converted HTML into template: %s", err)
	}

	err = browser.OpenReader(&out)
	if err != nil {
		logger.Fatalf("Failed to open rendered HTML in default browser: %s", err)
	}
}
