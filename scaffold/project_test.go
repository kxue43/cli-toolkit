package scaffold

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/BurntSushi/toml"
	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	Workflow struct {
		Jobs map[string]struct {
			RunsOn string `yaml:"runs-on"`
			Steps  []struct {
				With map[string]string `yaml:"with"`
				Name string            `yaml:"name"`
				Uses string            `yaml:"uses"`
			}
		} `yaml:"jobs"`
	}

	PreCommitConfig struct {
		Repos []struct {
			Repo  string `yaml:"repo"`
			Rev   string `yaml:"rev"`
			Hooks []struct {
				Id              string `yaml:"id"`
				LanguageVersion string `yaml:"language_version"`
			} `yaml:"hooks"`
		} `yaml:"repos"`
	}

	PyProjectToml struct {
		Project struct {
			Name           string `toml:"name"`
			Description    string `toml:"description"`
			RequiresPython string `toml:"requires-python"`
		} `toml:"project"`

		Tool struct {
			SetupTools struct {
				PackageData map[string][]string `toml:"package-data"`
			} `toml:"setuptools"`
			Poetry struct {
				Group map[string]struct {
					Dependencies map[string]string `toml:"dependencies"`
				} `toml:"group"`
				Dependencies struct {
					Python string `toml:"python"`
				} `toml:"dependencies"`
			} `toml:"poetry"`
			Black struct {
				TargetVersion []string `toml:"target-version"`
			} `toml:"black"`
		} `toml:"tool"`
	}

	PackageJson struct {
		DevDependencies map[string]string `json:"devDependencies"`
		Dependencies    map[string]string `json:"dependencies"`
		Name            string            `json:"name"`
		Repository      struct {
			Url string `json:"url"`
		} `json:"repository"`
	}
)

func TestProjects(t *testing.T) {
	handlerFactory := func(tmplt, version string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Helper()

			body := fmt.Sprintf(tmplt, version)

			_, err := w.Write([]byte(body))
			require.NoError(t, err)
		})
	}

	mux := http.NewServeMux()

	githubVersion := "v1.2.3"
	pypiVersion := "4.5.6"
	npmVersion := "7.8.9"

	mux.Handle("GET /{owner}/{repo}/releases/latest", handlerFactory(`{"tag_name": %q}`, githubVersion))
	mux.Handle("GET /{name}/json", handlerFactory(`{"info": {"version": %q}}`, pypiVersion))

	npmHandler := handlerFactory(`{"dist-tags": {"latest": %q}}`, npmVersion)

	mux.Handle("GET /{name}", npmHandler)
	mux.Handle("GET /@eslint/js", npmHandler)
	mux.Handle("GET /@vitest/coverage-v8", npmHandler)
	mux.Handle("GET /@aws-cdk/assert", npmHandler)

	ts := httptest.NewServer(mux)

	defer ts.Close()

	t.Run("GoProject", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "go")
		require.NoError(t, err)

		defer func() { _ = os.RemoveAll(tempDir) }()

		original := githubAPIURLPrefix
		githubAPIURLPrefix = ts.URL

		defer func() { githubAPIURLPrefix = original }()

		cmd := GoProjectCmd{
			ModulePath:          "module-path",
			GoVersion:           "1.24.5",
			GolangcilintVersion: "LATEST",
			TartufoVersion:      "LATEST",
			TimeoutSeconds:      5,
		}

		err = cmd.BeforeReset()
		require.NoError(t, err)

		err = cmd.AfterApply()
		require.NoError(t, err)

		cmd.rootDir = tempDir

		err = cmd.Run()
		require.NoError(t, err)

		contents, err := os.ReadFile(filepath.Clean(filepath.Join(tempDir, ".github", "workflows", "test-and-lint.yaml")))
		require.NoError(t, err)

		var workflow Workflow

		err = yaml.Unmarshal(contents, &workflow)
		require.NoError(t, err)

		assert.Equal(t, "^"+cmd.GoVersion, workflow.Jobs["test-and-lint"].Steps[1].With["go-version"])
		assert.Equal(t, githubVersion, workflow.Jobs["test-and-lint"].Steps[len(workflow.Jobs["test-and-lint"].Steps)-1].With["version"])

		contents, err = os.ReadFile(filepath.Clean(filepath.Join(tempDir, ".pre-commit-config.yaml")))
		require.NoError(t, err)

		var preCommitConfig PreCommitConfig

		err = yaml.Unmarshal(contents, &preCommitConfig)
		require.NoError(t, err)

		assert.Equal(t, githubVersion, preCommitConfig.Repos[0].Rev)
		assert.Equal(t, githubVersion, preCommitConfig.Repos[1].Rev)

		for _, hook := range preCommitConfig.Repos[0].Hooks {
			assert.True(t, strings.HasPrefix(hook.Id, "golangci-lint-"))
		}

		files := []string{".golangci.yaml", "Makefile", "tartufo.toml"}
		for _, name := range files {
			contents, err = os.ReadFile(filepath.Clean(filepath.Join(tempDir, name)))
			require.NoError(t, err)

			contents1, err := os.ReadFile(filepath.Clean(filepath.Join(".go", name+tmpltExt)))
			require.NoError(t, err)

			assert.Equal(t, contents1, contents)
		}
	})

	t.Run("PythonProject", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "python")
		require.NoError(t, err)

		defer func() { _ = os.RemoveAll(tempDir) }()

		o1 := githubAPIURLPrefix
		o2 := pypiURLPrefix
		githubAPIURLPrefix = ts.URL
		pypiURLPrefix = ts.URL

		defer func() {
			githubAPIURLPrefix = o1
			pypiURLPrefix = o2
		}()

		pythonVersion := PythonVersion{}
		err = pythonVersion.UnmarshalText([]byte("3.12"))
		require.NoError(t, err)

		assert.Equal(t, "3", pythonVersion.Major)
		assert.Equal(t, "12", pythonVersion.Minor)

		cmd := PythonProjectCmd{
			ProjectName:       "fs-walk",
			Description:       "description",
			BlackVersion:      "LATEST",
			Flake8Version:     "LATEST",
			TartufoVersion:    "LATEST",
			IPyKernelVersion:  "LATEST",
			MypyVersion:       "do-not-query",
			PytestVersion:     "LATEST",
			PytestMockVersion: "LATEST",
			PytestCovVersion:  "LATEST",
			SphinxVersion:     "LATEST",
			PythonVersion:     pythonVersion,
			TimeoutSeconds:    5,
		}

		err = cmd.BeforeReset()
		require.NoError(t, err)

		err = cmd.AfterApply()
		require.NoError(t, err)

		cmd.rootDir = tempDir

		err = cmd.Run()
		require.NoError(t, err)

		workflowFiles := []struct {
			name  string
			job   string
			index int
		}{
			{"publish-docs.yaml", "publish", 4},
			{"python-lint-on-pr.yaml", "lint", 1},
			{"unit-test.yaml", "unit-test", 4},
		}

		var contents []byte

		var workflow Workflow

		for _, file := range workflowFiles {
			contents, err = os.ReadFile(filepath.Clean(filepath.Join(tempDir, ".github", "workflows", file.name)))
			require.NoError(t, err)

			err = yaml.Unmarshal(contents, &workflow)
			require.NoError(t, err)

			assert.Equal(t, cmd.PythonVersion.String(), workflow.Jobs[file.job].Steps[file.index].With["python-version"])
		}

		contents, err = os.ReadFile(filepath.Clean(filepath.Join(tempDir, ".pre-commit-config.yaml")))
		require.NoError(t, err)

		var preCommitConfig PreCommitConfig

		err = yaml.Unmarshal(contents, &preCommitConfig)
		require.NoError(t, err)

		assert.Equal(t, githubVersion, preCommitConfig.Repos[0].Rev)
		assert.Equal(t, pypiVersion, preCommitConfig.Repos[1].Rev)
		assert.Equal(t, githubVersion, preCommitConfig.Repos[3].Rev)

		assert.Equal(t, "python"+cmd.PythonVersion.String(), preCommitConfig.Repos[0].Hooks[0].LanguageVersion)

		contents, err = os.ReadFile(filepath.Clean(filepath.Join(tempDir, "pyproject.toml")))
		require.NoError(t, err)

		var pyProjectToml PyProjectToml

		err = toml.Unmarshal(contents, &pyProjectToml)
		require.NoError(t, err)

		assert.Equal(t, cmd.ProjectName, pyProjectToml.Project.Name)
		assert.Equal(t, cmd.Description, pyProjectToml.Project.Description)
		assert.Equal(t, "~"+cmd.PythonVersion.String(), pyProjectToml.Project.RequiresPython)
		assert.Equal(t, "py.typed", pyProjectToml.Tool.SetupTools.PackageData[dashLower(cmd.ProjectName)][0])
		assert.Equal(t, "~"+cmd.PythonVersion.String(), pyProjectToml.Tool.Poetry.Dependencies.Python)
		assert.Equal(t, "^"+pypiVersion, pyProjectToml.Tool.Poetry.Group["develop"].Dependencies["ipykernel"])
		assert.Equal(t, githubVersion, pyProjectToml.Tool.Poetry.Group["linting"].Dependencies["black"])
		assert.Equal(t, pypiVersion, pyProjectToml.Tool.Poetry.Group["linting"].Dependencies["flake8"])
		assert.Equal(t, "^do-not-query", pyProjectToml.Tool.Poetry.Group["linting"].Dependencies["mypy"])
		assert.Equal(t, "^"+pypiVersion, pyProjectToml.Tool.Poetry.Group["test"].Dependencies["pytest"])
		assert.Equal(t, "^"+pypiVersion, pyProjectToml.Tool.Poetry.Group["test"].Dependencies["pytest-mock"])
		assert.Equal(t, "^"+pypiVersion, pyProjectToml.Tool.Poetry.Group["test"].Dependencies["pytest-cov"])
		assert.Equal(t, "^"+pypiVersion, pyProjectToml.Tool.Poetry.Group["docs"].Dependencies["sphinx"])
		assert.Equal(t, "py"+cmd.PythonVersion.NumsOnly(), pyProjectToml.Tool.Black.TargetVersion[0])

		var contents1 []byte

		constantFiles := []string{
			".flake8",
			".gitignore",
			"mypy.ini",
			"poetry.toml",
			"tartufo.toml",
			filepath.Join("docs", "_static", "custom.css"),
			filepath.Join("docs", ".air.toml"),
			filepath.Join("docs", "Makefile"),
			filepath.Join("docs", "README.md"),
		}
		for _, name := range constantFiles {
			contents, err = os.ReadFile(filepath.Clean(filepath.Join(tempDir, name)))
			require.NoError(t, err)

			contents1, err = os.ReadFile(filepath.Clean(filepath.Join(".python", name+tmpltExt)))
			require.NoError(t, err)

			assert.Equal(t, contents1, contents)
		}

		fd, err := os.Open(filepath.Clean(filepath.Join(tempDir, "docs", "index.rst")))
		require.NoError(t, err)

		defer func(fd *os.File) { _ = fd.Close() }(fd)

		scanner := bufio.NewScanner(fd)
		scanner.Scan()
		assert.Equal(t, cmd.ProjectName, scanner.Text())

		fd, err = os.Open(filepath.Clean(filepath.Join(tempDir, "docs", "conf.py")))
		require.NoError(t, err)

		defer func(fd *os.File) { _ = fd.Close() }(fd)

		scanner = bufio.NewScanner(fd)

		for range 13 {
			scanner.Scan()
		}

		assert.Equal(t, fmt.Sprintf("project = %q", cmd.ProjectName), scanner.Text())

		for range 62 {
			scanner.Scan()
		}

		assert.Equal(t, fmt.Sprintf(`    result = f"https://github.com/kxue43/%s/blob/main/{filename}.py{anchor}"`, cmd.ProjectName), scanner.Text())
	})

	t.Run("TsCdkProject", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "ts-cdk")
		require.NoError(t, err)

		defer func() { _ = os.RemoveAll(tempDir) }()

		original := npmAPIBURLPrefix
		npmAPIBURLPrefix = ts.URL

		defer func() { npmAPIBURLPrefix = original }()

		nodejsVersion := NodejsVersion{}
		err = nodejsVersion.UnmarshalText([]byte("20.19"))

		require.NoError(t, err)

		assert.Equal(t, "20", nodejsVersion.Major)
		assert.Equal(t, "19", nodejsVersion.Minor)

		cmd := TsCdkProjectCmd{
			ProjectName:             "adhoc",
			EslintVersion:           "LATEST",
			EslintJsVersion:         "LATEST",
			TypeScriptEslintVersion: "LATEST",
			VitestVersion:           "LATEST",
			VitestCoverageV8Version: "LATEST",
			AwsCdkCliVersion:        "LATEST",
			EsbuildVersion:          "LATEST",
			PrettierVersion:         "LATEST",
			TsxVersion:              "LATEST",
			TypeScriptVersion:       "LATEST",
			AwsCdkAssertVersion:     "LATEST",
			AwsCdkLibVersion:        "LATEST",
			ConstructsVersion:       "LATEST",
			YamlVersion:             "LATEST",
			NodejsVersion:           nodejsVersion,
			TimeoutSeconds:          5,
		}

		err = cmd.BeforeReset()
		require.NoError(t, err)

		err = cmd.AfterApply()
		require.NoError(t, err)

		cmd.rootDir = tempDir

		err = cmd.Run()
		require.NoError(t, err)

		var contents, contents1 []byte

		constantFiles := []string{
			".gitattributes",
			".gitignore",
			".npmignore",
			".prettierignore",
			".prettierrc.json",
			"cdk.json",
			"eslint.config.js",
			"tsconfig.json",
			"vitest.config.ts",
			filepath.Join("src", "main.ts"),
			filepath.Join("test", "setUp.ts"),
			filepath.Join("test", "utils.ts"),
			filepath.Join("test", "vitest.d.ts"),
		}
		for _, name := range constantFiles {
			contents, err = os.ReadFile(filepath.Clean(filepath.Join(tempDir, name)))
			require.NoError(t, err)

			contents1, err = os.ReadFile(filepath.Clean(filepath.Join(".ts.cdk", name+tmpltExt)))
			require.NoError(t, err)

			assert.Equal(t, contents1, contents)
		}

		var packageJson PackageJson

		contents, err = os.ReadFile(filepath.Clean(filepath.Join(tempDir, "package.json")))
		require.NoError(t, err)

		err = json.Unmarshal(contents, &packageJson)
		require.NoError(t, err)

		assert.Equal(t, cmd.ProjectName, packageJson.Name)
		assert.Equal(t, fmt.Sprintf("https://github.com/kxue43/%s", cmd.ProjectName), packageJson.Repository.Url)

		for _, vs := range cmd.VersionSetters {
			if version, ok := packageJson.DevDependencies[vs.Name]; ok {
				assert.Equal(t, "^"+npmVersion, version)
			} else {
				assert.Equal(t, "^"+npmVersion, packageJson.Dependencies[vs.Name])
			}
		}

		assert.Equal(t, fmt.Sprintf("^%s.0", cmd.NodejsVersion.String()), packageJson.DevDependencies["@types/node"])
	})
}

func TestGetVersions(t *testing.T) {
	handlerFactory := func(registry, name, version string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			t.Helper()

			contents, err := os.ReadFile(fmt.Sprintf("testdata/%s.%s.resp.json", registry, name))
			require.NoError(t, err)

			tmplt, err := template.New("response").Parse(string(contents))
			require.NoError(t, err)

			err = tmplt.Execute(w, version)
			require.NoError(t, err)
		})
	}

	blackVersion := "25.1.0"
	golangciLintVersion := "v2.2.2"
	awsCdkLibVersion := "2.204.0"

	mux := http.NewServeMux()

	mux.Handle("GET /black/json", handlerFactory("pypi", "black", blackVersion))
	mux.Handle("GET /golangci/golangci-lint/releases/latest", handlerFactory("github", "golangci-lint", golangciLintVersion))
	mux.Handle("GET /aws-cdk-lib", handlerFactory("npm", "aws-cdk-lib", awsCdkLibVersion))

	ts := httptest.NewServer(mux)

	defer ts.Close()

	t.Run("PyPIPackageLatestVersion", func(t *testing.T) {
		original := pypiURLPrefix
		pypiURLPrefix = ts.URL

		defer func() {
			pypiURLPrefix = original
		}()

		v, err1 := PyPIPackageLatestVersion(context.Background(), "black")

		require.NoError(t, err1)
		assert.Equal(t, blackVersion, v)
	})

	t.Run("GitHubProjectLatestReleaseTag", func(t *testing.T) {
		original := githubAPIURLPrefix
		githubAPIURLPrefix = ts.URL

		defer func() {
			githubAPIURLPrefix = original
		}()

		v, err1 := GitHubProjectLatestReleaseTag(context.Background(), "golangci", "golangci-lint")

		require.NoError(t, err1)
		assert.Equal(t, golangciLintVersion, v)
	})

	t.Run("NPMPackageLatestVersion", func(t *testing.T) {
		original := npmAPIBURLPrefix
		npmAPIBURLPrefix = ts.URL

		defer func() {
			npmAPIBURLPrefix = original
		}()

		v, err1 := NPMPackageLatestVersion(context.Background(), "aws-cdk-lib")

		require.NoError(t, err1)
		assert.Equal(t, awsCdkLibVersion, v)
	})
}
