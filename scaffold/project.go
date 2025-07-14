package scaffold

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"text/template"
	"time"

	"golang.org/x/mod/modfile"

	"github.com/kxue43/cli-toolkit/jsonstream"
)

type (
	GoProjectCmd struct {
		rootDir             string
		ModulePath          string `arg:"" required:"" name:"ModulePath" help:"Module path for the project."`
		GoVersion           string `name:"go-version" default:"1.24.1" help:"Will appear in go.mod and GitHub Actions workflow."`
		GolangcilintVersion string `name:"golangci-lint-version" default:"LATEST" help:"Will appear in .pre-commit-config.yaml and GitHub Actions workflow."`
		TartufoVersion      string `name:"tartufo-version" default:"LATEST" help:"Will appear in .pre-commit-config.yaml."`
		TimeoutSeconds      int    `name:"timeout-seconds" default:"1" help:"Timeout scaffolding after this many seconds."`
	}

	PythonProjectCmd struct {
		rootDir           string
		ProjectName       string        `arg:"" required:"" name:"ProjectName" help:"Python project name."`
		Description       string        `name:"description" default:"PLACEHOLDER" help:"Short description of the project"`
		BlackVersion      string        `name:"black-version" default:"LATEST" help:"Will appear in .pre-commit-config.yaml and pyproject.toml."`
		Flake8Version     string        `name:"flake8-version" default:"LATEST" help:"Will appear in .pre-commit-config.yaml and pyproject.toml."`
		TartufoVersion    string        `name:"tartufo-version" default:"LATEST" help:"Will appear in .pre-commit-config.yaml."`
		IPyKernelVersion  string        `name:"ipykernel-version" default:"LATEST" help:"Will appear in pyproject.toml."`
		MypyVersion       string        `name:"mypy-version" default:"LATEST" help:"Will appear in pyproject.toml."`
		PytestVersion     string        `name:"pytest-version" default:"LATEST" help:"Will appear in pyproject.toml."`
		PytestMockVersion string        `name:"pytest-mock-version" default:"LATEST" help:"Will appear in pyproject.toml."`
		PytestCovVersion  string        `name:"pytest-cov-version" default:"LATEST" help:"Will appear in pyproject.toml."`
		SphinxVersion     string        `name:"sphinx-cov-version" default:"LATEST" help:"Will appear in pyproject.toml."`
		PythonVersion     PythonVersion `name:"python-version" required:"" help:"Python interpreter version. Only accept major and minor version, i.e. the 3.Y format."`
		TimeoutSeconds    int           `name:"timeout-seconds" default:"1" help:"Timeout scaffolding after this many seconds."`
	}

	WriteHook func(io.Writer) error

	PythonVersion struct {
		Major string
		Minor string
	}

	versionSetter struct {
		indirect *string
		scope    string
		name     string
		registry uint
	}

	setterFunc func(context.Context) error
)

const (
	github = 1 << iota
	pypi
	// npm
)

var (
	//go:embed ".python/*"
	pythonFS embed.FS

	//go:embed ".go/*"
	goFS embed.FS

	pypiURL = "https://pypi.org/pypi"

	githubAPIBaseURL = "https://api.github.com/repos"
)

func (pv *PythonVersion) UnmarshalText(text []byte) error {
	regex := regexp.MustCompile(`^3\.(\d+)$`)

	m := regex.FindStringSubmatch(string(text))
	if len(m) == 0 {
		return fmt.Errorf(`%s is not of the "3.(\d+)" format`, string(text))
	}

	pv.Major = "3"
	pv.Minor = m[1]

	return nil
}

func (pv *PythonVersion) String() string {
	return pv.Major + "." + pv.Minor
}

func (pv *PythonVersion) NumsOnly() string {
	return pv.Major + pv.Minor
}

func ToModFile(modulePath, goVersion string) WriteHook {
	return func(fd io.Writer) error {
		goModFile := new(modfile.File)

		err := goModFile.AddModuleStmt(modulePath)
		if err != nil {
			return fmt.Errorf("failed to add module statement to go.mod file: %w", err)
		}

		err = goModFile.AddGoStmt(goVersion)
		if err != nil {
			return fmt.Errorf("failed to add go statement to go.mod file: %w", err)
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

func landFromPublicEndpoint(ctx context.Context, url, path string) (value string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to prepare GET request to endpoint %s: %w", url, err)
	}

	resp, err := http.DefaultClient.Do(req)
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

	v, err := angler.Land(ctx)
	if err != nil {
		return "", fmt.Errorf(`failed to get the value at the %q path from the response body: %w`, path, err)
	}

	value, ok := v.(string)
	if !ok {
		return "", fmt.Errorf(`the value at the %q path is not string`, path)
	}

	return value, nil
}

func GitHubProjectLatestReleaseTag(ctx context.Context, owner, repo string) (tag string, err error) {
	return landFromPublicEndpoint(ctx, fmt.Sprintf("%s/%s/%s/releases/latest", githubAPIBaseURL, owner, repo), ".tag_name")
}

func PyPIPackageLatestVersion(ctx context.Context, name string) (version string, err error) {
	return landFromPublicEndpoint(ctx, fmt.Sprintf("%s/%s/json", pypiURL, name), ".info.version")
}

func dashLower(s string) string {
	return strings.ReplaceAll(s, "-", "_")
}

func touch(path string) error {
	fd, err := os.Create(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("failed to create empty file %q: %w", path, err)
	}

	defer func() { _ = fd.Close() }()

	return nil
}

func writeFiles(dest string, srcFS embed.FS, srcPrefix string, data any) (err error) {
	var items []fs.DirEntry

	srcDirs := make([]string, 1, 2)
	srcFiles := make([]string, 0, 5)

	if _, err = srcFS.ReadDir(srcPrefix); err != nil {
		return fmt.Errorf("%q is not a directory of data files: %w", srcPrefix, err)
	}

	srcDirs[0] = srcPrefix

	var srcDir, newDestDir string

	for len(srcDirs) > 0 {
		srcDir = srcDirs[0]

		items, err = srcFS.ReadDir(srcDir)
		if err != nil {
			return fmt.Errorf("failed to open the relative directory %q in data files tree: %w", srcDir, err)
		}

		for _, item := range items {
			if item.IsDir() {
				newSrcDir := filepath.Clean(filepath.Join(srcDir, item.Name()))
				srcDirs = append(srcDirs, newSrcDir)

				newDestDir, err = filepath.Rel(srcPrefix, newSrcDir)
				if err != nil {
					return fmt.Errorf("erroneous directory %q from data files tree: %w", newSrcDir, err)
				}

				if err = os.MkdirAll(filepath.Clean(filepath.Join(dest, newDestDir)), 0750); err != nil {
					return fmt.Errorf("failed to create directory %q in destination folder: %w", newDestDir, err)
				}

				continue
			}

			srcFiles = append(srcFiles, filepath.Clean(filepath.Join(srcDir, item.Name())))
		}

		srcDirs = srcDirs[1:]
	}

	var wg sync.WaitGroup

	semaphore := make(chan struct{}, 7)
	out := make(chan error)

	tmplt, err := template.New("entry").Delims("{%", "%}").Funcs(template.FuncMap{"DashLower": dashLower}).ParseFS(srcFS, srcFiles...)
	if err != nil {
		return fmt.Errorf("failed to parse data files as templates: %w", err)
	}

	for _, srcFile := range srcFiles {
		wg.Add(1)

		go func() {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			destFile, err1 := filepath.Rel(srcPrefix, srcFile)
			if err1 != nil {
				out <- fmt.Errorf("name of the data file %q does not start with prefix %q: %w", srcFile, srcPrefix, err1)

				return
			}

			err1 = WriteToFile(dest, destFile, func(fd io.Writer) error {
				return tmplt.ExecuteTemplate(fd, filepath.Base(srcFile), data)
			})
			if err1 != nil {
				out <- fmt.Errorf("failed to create new file from template %q: %w", srcFile, err1)

				return
			}

			out <- nil
		}()
	}

	go func() {
		wg.Wait()

		close(out)
	}()

	var errs []error

	for err = range out {
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (vs *versionSetter) Func(ctx context.Context) (err error) {
	switch vs.registry {
	case github:
		*vs.indirect, err = GitHubProjectLatestReleaseTag(ctx, vs.scope, vs.name)
		if err != nil {
			return fmt.Errorf("failed to fetch the latest version of %s/%s from GitHub: %w", vs.scope, vs.name, err)
		}
	case pypi:
		*vs.indirect, err = PyPIPackageLatestVersion(ctx, vs.name)
		if err != nil {
			return fmt.Errorf("failed to fetch the latest version of %s from PyPI: %w", vs.name, err)
		}
	}

	return nil
}

func getSetterFuncs(vss []*versionSetter) []setterFunc {
	setterFuncs := make([]setterFunc, 0, len(vss))

	for _, vs := range vss {
		if *vs.indirect == "LATEST" {
			setterFuncs = append(setterFuncs, vs.Func)
		}
	}

	return setterFuncs
}

func setVersions(ctx context.Context, fns []setterFunc) error {
	var wg sync.WaitGroup

	semaphore := make(chan struct{}, 5)
	errs := make([]error, len(fns))

	for i, fn := range fns {
		wg.Add(1)

		go func() {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			errs[i] = fn(ctx)
		}()
	}

	wg.Wait()

	return errors.Join(errs...)
}

func (c *GoProjectCmd) AfterApply() (err error) {
	c.rootDir, err = os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	vss := []*versionSetter{
		{registry: github, scope: "golangci", name: "golangci-lint", indirect: &c.GolangcilintVersion},
		{registry: github, scope: "godaddy", name: "tartufo", indirect: &c.TartufoVersion},
	}

	setterFuncs := getSetterFuncs(vss)

	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Duration(c.TimeoutSeconds)*time.Second)

	defer cancelFunc()

	return setVersions(ctx, setterFuncs)
}

func (c *GoProjectCmd) Run() (err error) {
	if err = writeFiles(c.rootDir, goFS, ".go", c); err != nil {
		return err
	}

	if err = WriteToFile(c.rootDir, "go.mod", ToModFile(c.ModulePath, c.GoVersion)); err != nil {
		return err
	}

	return nil
}

func (c *PythonProjectCmd) AfterApply() (err error) {
	c.rootDir, err = os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	vss := []*versionSetter{
		{registry: github, scope: "psf", name: "black", indirect: &c.BlackVersion},
		{registry: github, scope: "godaddy", name: "tartufo", indirect: &c.TartufoVersion},
		{registry: pypi, name: "flake8", indirect: &c.Flake8Version},
		{registry: pypi, name: "ipykernel", indirect: &c.IPyKernelVersion},
		{registry: pypi, name: "mypy", indirect: &c.MypyVersion},
		{registry: pypi, name: "pytest", indirect: &c.PytestVersion},
		{registry: pypi, name: "pytest-mock", indirect: &c.PytestMockVersion},
		{registry: pypi, name: "pytest-cov", indirect: &c.PytestCovVersion},
		{registry: pypi, name: "Sphinx", indirect: &c.SphinxVersion},
	}

	setterFuncs := getSetterFuncs(vss)

	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Duration(c.TimeoutSeconds)*time.Second)

	defer cancelFunc()

	return setVersions(ctx, setterFuncs)
}

func (c *PythonProjectCmd) Run() (err error) {
	if err = writeFiles(c.rootDir, pythonFS, ".python", c); err != nil {
		return err
	}

	dir := filepath.Clean(filepath.Join(c.rootDir, "src", dashLower(c.ProjectName)))
	if err = os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create the 'src/%s' directory: %w", dashLower(c.ProjectName), err)
	}

	if err = touch(filepath.Join(dir, "__init__.py")); err != nil {
		return err
	}

	if err = touch(filepath.Join(dir, "py.typed")); err != nil {
		return err
	}

	return nil
}
