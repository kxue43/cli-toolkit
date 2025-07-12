package scaffold

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sync"
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
	// //go:embed ".python/*"
	// pythonFS embed.FS

	//go:embed ".go/*"
	goFS embed.FS

	pypiURL = "https://pypi.org/pypi"

	githubAPIBaseURL = "https://api.github.com/repos"
)

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

	tmplt, err := template.New("entry").Delims("{%", "%}").ParseFS(srcFS, srcFiles...)
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

func (c *GoProjectCmd) AfterApply() error {
	var err, err1 error

	c.rootDir, err = os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		defer wg.Done()

		c.GolangcilintVersion, err = GitHubProjectLatestReleaseTag("golangci", "golangci-lint")
		if err != nil {
			err = fmt.Errorf("failed to fetch the latest version of golangci-lint from GitHub: %w", err)
		}
	}()

	go func() {
		defer wg.Done()

		c.TartufoVersion, err1 = GitHubProjectLatestReleaseTag("godaddy", "tartufo")
		if err1 != nil {
			err1 = fmt.Errorf("failed to fetch the latest version of tartufo from GitHub: %w", err1)
		}
	}()

	wg.Wait()

	return errors.Join(err, err1)
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
