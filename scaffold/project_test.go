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

		err = os.RemoveAll(tempDir)
		require.NoError(t, err)
	}()

	cmd := GoProjectCmd{
		rootDir:             tempDir,
		ModulePath:          "module-path",
		GoVersion:           "1.24.5",
		GolangcilintVersion: "2.1.0",
	}

	err = cmd.Run()
	require.NoError(t, err)

	contents, err := os.ReadFile(filepath.Clean(filepath.Join(tempDir, ".github/workflows", "test-and-lint.yaml")))
	require.NoError(t, err)

	var workflow Workflow

	err = yaml.Unmarshal(contents, &workflow)
	require.NoError(t, err)

	assert.Equal(t, "^"+cmd.GoVersion, workflow.Jobs["test-and-lint"].Steps[1].With["go-version"])
	assert.Equal(t, "v"+cmd.GolangcilintVersion, workflow.Jobs["test-and-lint"].Steps[4].With["version"])

	contents, err = os.ReadFile(filepath.Clean(filepath.Join(tempDir, ".pre-commit-config.yaml")))
	require.NoError(t, err)

	var preCommitConfig PreCommitConfig

	err = yaml.Unmarshal(contents, &preCommitConfig)
	require.NoError(t, err)

	assert.Equal(t, "v"+cmd.GolangcilintVersion, preCommitConfig.Repos[0].Rev)
	assert.Equal(t, "v"+cmd.TartufoVersion, preCommitConfig.Repos[1].Rev)

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
