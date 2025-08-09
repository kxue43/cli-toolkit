package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/kxue43/cli-toolkit/version"
)

var (
	logger = log.New(os.Stderr, "toolkit-serve-static: ", 0)

	port        string
	showVersion bool

	helpMsg = `Usage: %s [flags] <DIR>

Serve static contents from the <DIR> folder.

Arguments:
  <DIR>    Directory that contains the static contents.

Flags:
`
)

func main() {
	flag.StringVar(&port, "port", "8090", "HTTP port of the local static files server.")
	flag.BoolVar(&showVersion, "version", false, "Show version information and quit.")

	flag.Usage = func() {
		_, _ = fmt.Fprintf(flag.CommandLine.Output(), helpMsg, os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()

	if showVersion {
		_, _ = fmt.Fprintln(flag.CommandLine.Output(), version.FromBuildInfo())

		os.Exit(0)
	}

	args := flag.Args()
	if len(args) == 0 {
		logger.Fatalf("The <DIR> argument is required.")
	}

	staticDir := args[0]

	logger.Fatal(http.ListenAndServe(":"+port, http.FileServer(http.Dir(staticDir))))
}
