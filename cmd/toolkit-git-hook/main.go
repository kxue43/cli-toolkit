package main

import (
	"fmt"
	"os"

	git "github.com/libgit2/git2go/v34"
)

func main() {
	repo, _ := git.OpenRepository(".git")

	fmt.Fprintf(os.Stderr, "Hello - %s\n", repo.Path())
}
