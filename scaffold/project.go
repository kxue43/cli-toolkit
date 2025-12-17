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
		ModulePath          string          `arg:"" required:"" name:"ModulePath" help:"Module path for the project."`
		GoVersion           string          `name:"go-version" default:"1.24.1" help:"Will appear in go.mod and GitHub Actions workflow."`
		GolangcilintVersion SemVer          `name:"golangci-lint-version" default:"LATEST" help:"Version of golangci-lint, a pre-commit hook."`
		TartufoVersion      SemVer          `name:"tartufo-version" default:"LATEST" help:"Version of tartufo, a pre-commit hook."`
		VersionSetters      []VersionSetter `kong:"-"`
		TimeoutSeconds      int             `name:"timeout-seconds" default:"1" help:"Timeout scaffolding after this many seconds."`
	}

	PythonProjectCmd struct {
		rootDir           string
		ProjectName       string          `arg:"" required:"" name:"ProjectName" help:"Python project name."`
		Description       string          `name:"description" default:"PLACEHOLDER" help:"Short description of the project"`
		TartufoVersion    SemVer          `name:"tartufo-version" default:"LATEST" help:"Version of tartufo, a pre-commit hook."`
		RuffVersion       SemVer          `name:"ruff-version" default:"LATEST" help:"Version of ruff, a linting dependency."`
		MypyVersion       SemVer          `name:"mypy-version" default:"LATEST" help:"Version of mypy, a linting dependency."`
		PytestVersion     SemVer          `name:"pytest-version" default:"LATEST" help:"Version of pytest, a test dependency."`
		PytestMockVersion SemVer          `name:"pytest-mock-version" default:"LATEST" help:"Version of pytest-mock, a test dependency."`
		PytestCovVersion  SemVer          `name:"pytest-cov-version" default:"LATEST" help:"Version of pytest-cov, a test dependency."`
		SphinxVersion     SemVer          `name:"sphinx-version" default:"LATEST" help:"Version of sphinx, a doc dependency."`
		PythonVersion     PythonVersion   `name:"python-version" required:"" help:"Python 3 interpreter version. Only accept major and minor version, i.e. the 3.Y format."`
		VersionSetters    []VersionSetter `kong:"-"`
		TimeoutSeconds    int             `name:"timeout-seconds" default:"1" help:"Timeout scaffolding after this many seconds."`
	}

	TsCdkProjectCmd struct {
		rootDir                 string
		ProjectName             string          `arg:"" required:"" name:"ProjectName" help:"TypeScript CDK project name."`
		EslintVersion           SemVer          `name:"eslint-version" default:"LATEST" help:"Version of eslint, a dev dependency."`
		EslintJsVersion         SemVer          `name:"eslint-js-version" default:"LATEST" help:"Version of eslint-js, a dev dependency."`
		TypeScriptEslintVersion SemVer          `name:"typescript-eslint-version" default:"LATEST" help:"Version of typescript-eslint, a dev dependency."`
		VitestVersion           SemVer          `name:"vitest-version" default:"LATEST" help:"Version of vitest, a test dependency."`
		VitestCoverageV8Version SemVer          `name:"vitest-coverage-v8-version" default:"LATEST" help:"Version of vitest-coverage, a test dependency."`
		AwsCdkCliVersion        SemVer          `name:"aws-cdk-cli-version" default:"LATEST" help:"Version of the AWS CDK CLI."`
		AwsCdkLibVersion        SemVer          `name:"aws-cdk-lib-version" default:"LATEST" help:"Version of the AWS CDK library."`
		ConstructsVersion       SemVer          `name:"constructs-version" default:"LATEST" help:"Version of the constructs library."`
		EsbuildVersion          SemVer          `name:"esbuild-version" default:"LATEST" help:"Version of esbuild, a dev dependency."`
		PrettierVersion         SemVer          `name:"prettier-version" default:"LATEST" help:"Version of prettier, a dev dependency."`
		TsxVersion              SemVer          `name:"tsx-version" default:"LATEST" help:"Version of tsx, a dev dependency."`
		TypeScriptVersion       SemVer          `name:"typescript-version" default:"LATEST" help:"Version of typescript."`
		AwsCdkAssertVersion     SemVer          `name:"aws-cdk-assert-version" default:"LATEST" help:"Version of aws-cdk-assert, a test dependency."`
		YamlVersion             SemVer          `name:"yaml-version" default:"LATEST" help:"Version of yaml, a test dependency."`
		NodejsVersion           NodejsVersion   `name:"nodejs-version" required:"" help:"Only accept major and minor version, i.e. the X.Y format."`
		VersionSetters          []VersionSetter `kong:"-"`
		TimeoutSeconds          int             `name:"timeout-seconds" default:"1" help:"Timeout scaffolding after this many seconds."`
	}

	WriteHook func(io.Writer) error

	PythonVersion struct {
		Major string
		Minor string
	}

	NodejsVersion struct {
		Major string
		Minor string
	}

	SemVer struct {
		major  string
		minor  string
		bugfix string
		set    bool
	}

	Registry byte

	VersionSetter struct {
		Indirect *SemVer
		Scope    string
		Name     string
		Registry Registry
	}

	setterFunc func(context.Context) error
)

const (
	GitHub Registry = iota
	PyPI
	NPM
)

var (
	//go:embed "all:.go/*"
	goFS embed.FS

	//go:embed "all:.ts.cdk/*"
	tsCdkFS embed.FS

	//go:embed "all:.python/*"
	pythonFS embed.FS

	githubAPIURLPrefix = "https://api.github.com/repos"

	npmAPIBURLPrefix = "https://registry.npmjs.org"

	pypiURLPrefix = "https://pypi.org/pypi"

	tmpltExt = ".tmplt"

	semVerRegex = regexp.MustCompile(`^(\d+)\.(\d+)(\..+)?$`)
)

func (pv *PythonVersion) UnmarshalText(text []byte) error {
	regex := regexp.MustCompile(`^3\.(\d+)$`)

	m := regex.FindStringSubmatch(string(text))
	if len(m) == 0 {
		return fmt.Errorf(`%s is not of the %s format`, string(text), regex)
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

func (nv *NodejsVersion) UnmarshalText(text []byte) error {
	regex := regexp.MustCompile(`^(\d+)\.(\d+)$`)

	m := regex.FindStringSubmatch(string(text))
	if len(m) == 0 {
		return fmt.Errorf(`%s is not of the %s format`, string(text), regex)
	}

	nv.Major = m[1]
	nv.Minor = m[2]

	return nil
}

func (nv *NodejsVersion) String() string {
	return nv.Major + "." + nv.Minor
}

func (sv *SemVer) String() string {
	return fmt.Sprintf("%s.%s.%s", sv.major, sv.minor, sv.bugfix)
}

func (sv *SemVer) MajorMinor() string {
	return fmt.Sprintf("%s.%s", sv.major, sv.minor)
}

func (sv *SemVer) UnmarshalText(text []byte) error {
	if strings.EqualFold(string(text), "LATEST") {
		return nil
	}

	m := semVerRegex.FindStringSubmatch(string(text))
	if len(m) == 0 {
		return fmt.Errorf(`%s is not of the %s format`, string(text), semVerRegex)
	}

	sv.major = m[1]
	sv.minor = m[2]
	sv.bugfix = "0"

	if m[3] != "" {
		sv.bugfix = strings.TrimPrefix(m[3], ".")
	}

	sv.set = true

	return nil
}

func (sv *SemVer) SetFromString(raw string) error {
	raw = strings.TrimPrefix(raw, "v")

	return sv.UnmarshalText([]byte(raw))
}

func (sv *SemVer) Set() bool {
	return sv.set
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
	return landFromPublicEndpoint(ctx, fmt.Sprintf("%s/%s/%s/releases/latest", githubAPIURLPrefix, owner, repo), ".tag_name")
}

func PyPIPackageLatestVersion(ctx context.Context, name string) (version string, err error) {
	return landFromPublicEndpoint(ctx, fmt.Sprintf("%s/%s/json", pypiURLPrefix, name), ".info.version")
}

func NPMPackageLatestVersion(ctx context.Context, name string) (version string, err error) {
	return landFromPublicEndpoint(ctx, fmt.Sprintf("%s/%s", npmAPIBURLPrefix, name), ".dist-tags.latest")
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

	tmplt, err := template.New("entry").Delims("{%", "%}").Funcs(template.FuncMap{"DashLower": dashLower}).ParseFS(srcFS, srcFiles...)
	if err != nil {
		return fmt.Errorf("failed to parse data files as templates: %w", err)
	}

	var wg sync.WaitGroup

	semaphore := make(chan struct{}, 7)
	out := make(chan error)

	for _, srcFile := range srcFiles {
		wg.Add(1)

		go func() {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			destItem, err1 := filepath.Rel(srcPrefix, srcFile)
			if err1 != nil {
				out <- fmt.Errorf("name of the data file %q does not start with prefix %q: %w", srcFile, srcPrefix, err1)

				return
			}

			err1 = WriteToFile(dest, strings.TrimSuffix(destItem, tmpltExt), func(fd io.Writer) error {
				return tmplt.ExecuteTemplate(fd, filepath.Base(srcFile), data)
			})
			if err1 != nil {
				out <- fmt.Errorf("failed to create new file from template %q: %w", srcFile, err1)

				return
			}
		}()
	}

	go func() {
		wg.Wait()

		close(out)
	}()

	var errs []error

	for err = range out {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

func (vs VersionSetter) Func(ctx context.Context) (err error) {
	var rawSpecifier string

	switch vs.Registry {
	case GitHub:
		rawSpecifier, err = GitHubProjectLatestReleaseTag(ctx, vs.Scope, vs.Name)
		if err != nil {
			return fmt.Errorf("failed to fetch the latest tag of %s/%s from GitHub: %w", vs.Scope, vs.Name, err)
		}
	case PyPI:
		rawSpecifier, err = PyPIPackageLatestVersion(ctx, vs.Name)
		if err != nil {
			return fmt.Errorf("failed to fetch the latest version of %s from PyPI: %w", vs.Name, err)
		}
	case NPM:
		rawSpecifier, err = NPMPackageLatestVersion(ctx, vs.Name)
		if err != nil {
			return fmt.Errorf("failed to fetch the latest version of %s from NPM: %w", vs.Name, err)
		}
	}

	err = vs.Indirect.SetFromString(rawSpecifier)
	if err != nil {
		return fmt.Errorf("failed to parse %q as a semantic version specifier: %s", rawSpecifier, err.Error())
	}

	return nil
}

func getSetterFuncs(vss []VersionSetter) []setterFunc {
	setterFuncs := make([]setterFunc, 0, len(vss))

	for i := range vss {
		if !vss[i].Indirect.Set() {
			setterFuncs = append(setterFuncs, vss[i].Func)
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

func (c *GoProjectCmd) BeforeReset() error {
	c.VersionSetters = []VersionSetter{
		{Registry: GitHub, Scope: "golangci", Name: "golangci-lint", Indirect: &c.GolangcilintVersion},
		{Registry: GitHub, Scope: "godaddy", Name: "tartufo", Indirect: &c.TartufoVersion},
	}

	return nil
}

func (c *GoProjectCmd) AfterApply() (err error) {
	c.rootDir, err = os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	setterFuncs := getSetterFuncs(c.VersionSetters)

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

func (c *PythonProjectCmd) BeforeReset() error {
	c.VersionSetters = []VersionSetter{
		{Registry: PyPI, Name: "ruff", Indirect: &c.RuffVersion},
		{Registry: PyPI, Name: "mypy", Indirect: &c.MypyVersion},
		{Registry: PyPI, Name: "pytest", Indirect: &c.PytestVersion},
		{Registry: PyPI, Name: "pytest-mock", Indirect: &c.PytestMockVersion},
		{Registry: PyPI, Name: "pytest-cov", Indirect: &c.PytestCovVersion},
		{Registry: PyPI, Name: "Sphinx", Indirect: &c.SphinxVersion},
		{Registry: GitHub, Scope: "godaddy", Name: "tartufo", Indirect: &c.TartufoVersion},
	}

	return nil
}

func (c *PythonProjectCmd) AfterApply() (err error) {
	c.rootDir, err = os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	setterFuncs := getSetterFuncs(c.VersionSetters)

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

func (c *TsCdkProjectCmd) BeforeReset() error {
	c.VersionSetters = []VersionSetter{
		{Registry: NPM, Name: "eslint", Indirect: &c.EslintVersion},
		{Registry: NPM, Name: "@eslint/js", Indirect: &c.EslintJsVersion},
		{Registry: NPM, Name: "typescript-eslint", Indirect: &c.TypeScriptEslintVersion},
		{Registry: NPM, Name: "vitest", Indirect: &c.VitestVersion},
		{Registry: NPM, Name: "@vitest/coverage-v8", Indirect: &c.VitestCoverageV8Version},
		{Registry: NPM, Name: "aws-cdk", Indirect: &c.AwsCdkCliVersion},
		{Registry: NPM, Name: "esbuild", Indirect: &c.EsbuildVersion},
		{Registry: NPM, Name: "prettier", Indirect: &c.PrettierVersion},
		{Registry: NPM, Name: "tsx", Indirect: &c.TsxVersion},
		{Registry: NPM, Name: "typescript", Indirect: &c.TypeScriptVersion},
		{Registry: NPM, Name: "@aws-cdk/assert", Indirect: &c.AwsCdkAssertVersion},
		{Registry: NPM, Name: "aws-cdk-lib", Indirect: &c.AwsCdkLibVersion},
		{Registry: NPM, Name: "constructs", Indirect: &c.ConstructsVersion},
		{Registry: NPM, Name: "yaml", Indirect: &c.YamlVersion},
	}

	return nil
}

func (c *TsCdkProjectCmd) AfterApply() (err error) {
	c.rootDir, err = os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	setterFuncs := getSetterFuncs(c.VersionSetters)

	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Duration(c.TimeoutSeconds)*time.Second)

	defer cancelFunc()

	return setVersions(ctx, setterFuncs)
}

func (c *TsCdkProjectCmd) Run() (err error) {
	if err = writeFiles(c.rootDir, tsCdkFS, ".ts.cdk", c); err != nil {
		return err
	}

	return nil
}
