package scaffold

import (
	"io"
	"os"
	"path/filepath"
	"testing"

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

	fd1, err := os.Open(filepath.Clean(filepath.Join(tempDir, ".github/workflows", "test-and-lint.yaml")))
	require.NoError(t, err)

	defer func() { _ = fd1.Close() }()

	contents1, err := io.ReadAll(fd1)
	require.NoError(t, err)

	var workflow Workflow

	err = yaml.Unmarshal(contents1, &workflow)
	require.NoError(t, err)

	assert.Equal(t, "^"+cmd.GoVersion, workflow.Jobs["test-and-lint"].Steps[1].With["go-version"])
	assert.Equal(t, "v"+cmd.GolangcilintVersion, workflow.Jobs["test-and-lint"].Steps[4].With["version"])

	fd2, err := os.Open(filepath.Clean(filepath.Join(tempDir, ".golangci.yaml")))
	require.NoError(t, err)

	defer func() { _ = fd2.Close() }()

	fd3, err := os.Open(filepath.Clean(filepath.Join("data", ".golangci.yaml")))
	require.NoError(t, err)

	defer func() { _ = fd3.Close() }()

	contents2, err := io.ReadAll(fd2)
	require.NoError(t, err)

	contents3, err := io.ReadAll(fd3)
	require.NoError(t, err)

	assert.Equal(t, contents3, contents2)
}
