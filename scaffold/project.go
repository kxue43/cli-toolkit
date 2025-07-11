package scaffold

import (
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"text/template"

	"golang.org/x/mod/modfile"

	"github.com/kxue43/cli-toolkit/jsonstream"
)

type (
	GoProjectCmd struct {
		rootDir             string
		ModulePath          string `arg:"" required:"" name:"ModulePath" help:"Module path for the project."`
		GoVersion           string `name:"go-version" default:"1.24.1" help:"Will appear in go.mod and GitHub Actions workflow."`
		GolangcilintVersion string `name:"golangci-lint-version" default:"LATEST" help:"Will appear in .pre-commit-config.yaml GitHub Actions workflow."`
		TartufoVersion      string `name:"tartufo-version" default:"LATEST" help:"Will appear in .pre-commit-config.yaml."`
	}

	WriteHook func(io.Writer) error
)

var (
	//go:embed "data/go/.golangci.yaml"
	golangciYaml []byte

	//go:embed "data/go/test-and-lint.yaml"
	goTestAndLintYaml string

	//go:embed "data/go/.pre-commit-config.yaml"
	goPreCommitConfigYaml string

	//go:embed "data/go/Makefile"
	goMakefile []byte

	//go:embed "data/go/tartufo.toml"
	goTartufoToml []byte

	pypiURL = "https://pypi.org/pypi"

	githubAPIBaseURL = "https://api.github.com/repos"
)

func FromData(data []byte) WriteHook {
	return func(fd io.Writer) error {
		_, err := fd.Write(data)

		return err
	}
}

func FromTemplate(name, text string, data any) WriteHook {
	return func(fd io.Writer) error {
		t, err := template.New(name).Parse(text)
		if err != nil {
			return fmt.Errorf("failed to load template for %s: %w", name, err)
		}

		return t.Execute(fd, data)
	}
}

func ToModFile(modulePath, goVersion string) WriteHook {
	return func(fd io.Writer) error {
		goModFile := new(modfile.File)

		err := goModFile.AddModuleStmt(modulePath)
		if err != nil {
			return fmt.Errorf("failed to format starter go.mod file: %w", err)
		}

		err = goModFile.AddGoStmt(goVersion)
		if err != nil {
			return fmt.Errorf("failed to format starter go.mod file: %w", err)
		}

		contents, err := goModFile.Format()
		if err != nil {
			return fmt.Errorf("failed to format starter go.mod file: %w", err)
		}

		_, err = fd.Write(contents)

		return err
	}
}

func WriteToFile(dir, name string, hook WriteHook) (err error) {
	fd, err := os.Create(filepath.Clean(filepath.Join(dir, name)))
	if err != nil {
		return fmt.Errorf("failed to create %q file: %w", name, err)
	}

	defer func() { _ = fd.Close() }()

	err = hook(fd)
	if err != nil {
		return fmt.Errorf("failed to write to %q: %w", name, err)
	}

	return nil
}

func landFromPublicEndpoint(url, path string) (value string, err error) {
	resp, err := http.Get(url) //nolint:gosec // url is only provided by own code
	if err != nil {
		return "", fmt.Errorf("failed to GET from endpoint %q: %w", url, err)
	}

	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if rc := resp.StatusCode; rc != 200 {
		return "", fmt.Errorf("failed to GET from endpoint %q, status code %d", url, rc)
	}

	angler, err := jsonstream.NewAngler(resp.Body, path)
	if err != nil {
		return "", fmt.Errorf("error from jsonstream.NewAngler: %w", err)
	}

	v, err := angler.Land()
	if err != nil {
		return "", fmt.Errorf(`failed to get the value at the %q path from the response body: %w`, path, err)
	}

	value, ok := v.(string)
	if !ok {
		return "", fmt.Errorf(`the value at the %q path is not string`, path)
	}

	return value, nil
}

func GitHubProjectLatestReleaseTag(owner, repo string) (tag string, err error) {
	return landFromPublicEndpoint(fmt.Sprintf("%s/%s/%s/releases/latest", githubAPIBaseURL, owner, repo), ".tag_name")
}

func PyPIPackageLatestVersion(name string) (version string, err error) {
	return landFromPublicEndpoint(fmt.Sprintf("%s/%s/json", pypiURL, name), ".info.version")
}

func (c *GoProjectCmd) AfterApply() error {
	var err error

	c.rootDir, err = os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	version, err := GitHubProjectLatestReleaseTag("golangci", "golangci-lint")
	if err != nil {
		return fmt.Errorf("failed to fetch the latest version of golangci-lint from GitHub: %w", err)
	}

	c.GolangcilintVersion = version

	version, err = GitHubProjectLatestReleaseTag("godaddy", "tartufo")
	if err != nil {
		return fmt.Errorf("failed to fetch the latest version of tartufo from GitHub: %w", err)
	}

	c.TartufoVersion = version

	return nil
}

func (c *GoProjectCmd) Run() (err error) {
	err = os.MkdirAll(filepath.Clean(filepath.Join(c.rootDir, ".github/workflows")), 0750)
	if err != nil {
		return fmt.Errorf("failed to create .github/workflows directory: %w", err)
	}

	if err = WriteToFile(c.rootDir, ".github/workflows/test-and-lint.yaml", FromTemplate("test-and-lint.yaml", goTestAndLintYaml, c)); err != nil {
		return err
	}

	if err = WriteToFile(c.rootDir, ".golangci.yaml", FromData(golangciYaml)); err != nil {
		return err
	}

	if err = WriteToFile(c.rootDir, "go.mod", ToModFile(c.ModulePath, c.GoVersion)); err != nil {
		return err
	}

	if err = WriteToFile(c.rootDir, "Makefile", FromData(goMakefile)); err != nil {
		return err
	}

	if err = WriteToFile(c.rootDir, "tartufo.toml", FromData(goTartufoToml)); err != nil {
		return err
	}

	if err = WriteToFile(c.rootDir, ".pre-commit-config.yaml", FromTemplate(".pre-commit-config.yaml", goPreCommitConfigYaml, c)); err != nil {
		return err
	}

	return nil
}
