package scaffold

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/template"

	"golang.org/x/mod/modfile"
)

type (
	GoProjectCmd struct {
		rootDir             string
		ModulePath          string `arg:"" required:"" name:"ModulePath" help:"Module path for the project."`
		GoVersion           string `name:"go-version" default:"1.24.1" help:"Will appear in go.mod and GitHub Actions workflow."`
		GolangcilintVersion string `name:"golangci-lint-version" default:"2.2.1" help:"Will appear in GitHub Actions workflow."`
	}
)

var (
	//go:embed "data/go/.golangci.yaml"
	golangciYaml []byte

	//go:embed "data/go/test-and-lint.yaml"
	goTestAndLintYaml string
)

func WriteToFile(dir, name string, hook func(io.Writer) error) (err error) {
	fd, err := os.Create(filepath.Clean(filepath.Join(dir, name)))
	if err != nil {
		return fmt.Errorf("failed to create %q file: %w", name, err)
	}

	err = hook(fd)
	if err != nil {
		return fmt.Errorf("failed to write to %q: %w", name, err)
	}

	err = fd.Close()
	if err != nil {
		return fmt.Errorf("failed to close %q after writing: %w", name, err)
	}

	return nil
}

func (c *GoProjectCmd) AfterApply() error {
	var err error

	c.rootDir, err = os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	return nil
}

func (c *GoProjectCmd) Run() (err error) {
	err = WriteToFile(c.rootDir, ".golangci.yaml", func(fd io.Writer) error {
		_, err1 := fd.Write(golangciYaml)

		return err1
	})
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Clean(filepath.Join(c.rootDir, ".github/workflows")), 0750)
	if err != nil {
		return fmt.Errorf("failed to create .github/workflows directory: %w", err)
	}

	err = WriteToFile(c.rootDir, ".github/workflows/test-and-lint.yaml", func(fd io.Writer) error {
		t, err1 := template.New("test-and-lint.yaml").Parse(goTestAndLintYaml)
		if err1 != nil {
			return fmt.Errorf("failed to load template for test-and-lint.yaml: %w", err1)
		}

		return t.Execute(fd, c)
	})
	if err != nil {
		return err
	}

	err = WriteToFile(c.rootDir, "go.mod", func(fd io.Writer) error {
		goModFile := new(modfile.File)

		err1 := goModFile.AddModuleStmt(c.ModulePath)
		if err != nil {
			return fmt.Errorf("failed to format starter go.mod file: %w", err1)
		}

		err1 = goModFile.AddGoStmt(c.GoVersion)
		if err != nil {
			return fmt.Errorf("failed to format starter go.mod file: %w", err1)
		}

		contents, err1 := goModFile.Format()
		if err1 != nil {
			return fmt.Errorf("failed to format starter go.mod file: %w", err1)
		}

		_, err1 = fd.Write(contents)

		return err1
	})

	return err
}
