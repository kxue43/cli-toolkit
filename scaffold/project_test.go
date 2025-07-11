package scaffold

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	Step struct {
		With map[string]string `yaml:"with"`
		Name string            `yaml:"name"`
		Uses string            `yaml:"uses"`
	}

	Job struct {
		RunsOn string `yaml:"runs-on"`
		Steps  []Step
	}

	Workflow struct {
		Jobs map[string]Job `yaml:"jobs"`
	}

	PreCommitConfig struct {
		Repos []struct {
			Repo  string `yaml:"repo"`
			Rev   string `yaml:"rev"`
			Hooks []struct {
				Id string `yaml:"id"`
			} `yaml:"hooks"`
		} `yaml:"repos"`
	}
)

func TestGoProject(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cli-toolkit")
	require.NoError(t, err)

	defer func() {
		t.Helper()

		err1 := os.RemoveAll(tempDir)
		require.NoError(t, err1)
	}()

	tartufoVersion := "v5.0.2"
	golangciLintVersion := "v2.2.2"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Helper()

		var name, tag string

		path := r.URL.Path

		if strings.HasSuffix(path, "tartufo/releases/latest") {
			name = "tartufo"
			tag = tartufoVersion
		} else if strings.HasSuffix(path, "golangci-lint/releases/latest") {
			name = "golangci-lint"
			tag = golangciLintVersion
		} else {
			t.Errorf("received unexpected url path %q", path)
		}

		contents, err1 := os.ReadFile(fmt.Sprintf("testdata/github.%s.resp.json", name))
		require.NoError(t, err1)

		tmplt, err1 := template.New("response").Parse(string(contents))
		require.NoError(t, err1)

		err1 = tmplt.Execute(w, tag)
		require.NoError(t, err1)
	}))

	defer ts.Close()

	o1 := pypiURL
	o2 := githubAPIBaseURL
	pypiURL = ts.URL
	githubAPIBaseURL = ts.URL

	defer func() {
		pypiURL = o1
		githubAPIBaseURL = o2
	}()

	cmd := GoProjectCmd{
		ModulePath: "module-path",
		GoVersion:  "1.24.5",
	}

	err = cmd.AfterApply()
	require.NoError(t, err)

	cmd.rootDir = tempDir

	err = cmd.Run()
	require.NoError(t, err)

	contents, err := os.ReadFile(filepath.Clean(filepath.Join(tempDir, ".github/workflows", "test-and-lint.yaml")))
	require.NoError(t, err)

	var workflow Workflow

	err = yaml.Unmarshal(contents, &workflow)
	require.NoError(t, err)

	assert.Equal(t, "^"+cmd.GoVersion, workflow.Jobs["test-and-lint"].Steps[1].With["go-version"])
	assert.Equal(t, golangciLintVersion, workflow.Jobs["test-and-lint"].Steps[4].With["version"])

	contents, err = os.ReadFile(filepath.Clean(filepath.Join(tempDir, ".pre-commit-config.yaml")))
	require.NoError(t, err)

	var preCommitConfig PreCommitConfig

	err = yaml.Unmarshal(contents, &preCommitConfig)
	require.NoError(t, err)

	assert.Equal(t, golangciLintVersion, preCommitConfig.Repos[0].Rev)
	assert.Equal(t, tartufoVersion, preCommitConfig.Repos[1].Rev)

	for _, hook := range preCommitConfig.Repos[0].Hooks {
		assert.True(t, strings.HasPrefix(hook.Id, "golangci-lint-"))
	}

	files := []string{".golangci.yaml", "Makefile", "tartufo.toml"}
	for _, name := range files {
		contents, err = os.ReadFile(filepath.Clean(filepath.Join(tempDir, name)))
		require.NoError(t, err)

		contents1, err := os.ReadFile(filepath.Clean(filepath.Join("data/go", name)))
		require.NoError(t, err)

		assert.Equal(t, contents1, contents)
	}
}

func TestPyPIPackageLatestVersion(t *testing.T) {
	var path string

	pack := "black"
	version := "25.1.0"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Helper()

		path = r.URL.Path

		contents, err := os.ReadFile(fmt.Sprintf("testdata/pypi.%s.resp.json", pack))
		require.NoError(t, err)

		tmplt, err := template.New("response").Parse(string(contents))
		require.NoError(t, err)

		err = tmplt.Execute(w, version)
		require.NoError(t, err)
	}))

	defer ts.Close()

	original := pypiURL
	pypiURL = ts.URL

	defer func() {
		pypiURL = original
	}()

	v, err1 := PyPIPackageLatestVersion(pack)

	require.NoError(t, err1)
	assert.Equal(t, version, v)
	assert.Equal(t, fmt.Sprintf("/%s/json", pack), path)
}

func TestGitHubProjectLatestReleaseTag(t *testing.T) {
	var path string

	owner := "golangci"
	repo := "golangci-lint"
	tag := "v2.2.2"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Helper()

		path = r.URL.Path

		contents, err := os.ReadFile(fmt.Sprintf("testdata/github.%s.resp.json", repo))
		require.NoError(t, err)

		tmplt, err := template.New("response").Parse(string(contents))
		require.NoError(t, err)

		err = tmplt.Execute(w, tag)
		require.NoError(t, err)
	}))

	defer ts.Close()

	original := githubAPIBaseURL
	githubAPIBaseURL = ts.URL

	defer func() {
		githubAPIBaseURL = original
	}()

	v, err1 := GitHubProjectLatestReleaseTag(owner, repo)

	require.NoError(t, err1)
	assert.Equal(t, tag, v)
	assert.Equal(t, fmt.Sprintf("/%s/%s/releases/latest", owner, repo), path)
}
