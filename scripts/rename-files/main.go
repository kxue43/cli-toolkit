package main

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
)

func HandleOne(root string) {
	err1 := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("failed to access path at %q: %w", path, err)
		}

		if d.IsDir() {
			return nil
		}

		parent := filepath.Dir(path)
		basename := filepath.Base(path)
		basename += ".tmplt"

		dest := filepath.Clean(filepath.Join(parent, basename))

		err = os.Rename(path, dest)
		if err != nil {
			return fmt.Errorf("failed to rename file %q: %w", path, err)
		}

		return nil
	})
	if err1 != nil {
		log.Printf("Encountered error while handlig %q: %s", root, err1)
	}
}

func main() {
	for _, root := range os.Args[1:] {
		HandleOne(root)
	}
}
