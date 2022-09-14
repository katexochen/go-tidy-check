package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
)

func main() {
	ok, err := check()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	if !ok {
		os.Exit(1)
	}
}

func check() (bool, error) {
	verbose := flag.Bool("v", false, "verbose debug output")
	diff := flag.Bool("d", false, "print diff")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var logger logger
	if *verbose {
		logger = debugLogger{}
	} else {
		logger = nopLogger{}
	}

	logger.Log("opening repository")
	repo, err := git.PlainOpenWithOptions("", &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return false, fmt.Errorf("opening repo: %w", err)
	}

	logger.Log("checking if repository is modified")
	modified, err := repoModified(repo)
	if err != nil {
		return false, fmt.Errorf("checking for existing modification: %w", err)
	}
	if modified {
		return false, errors.New("repo repo has uncommitted changes")
	}

	logger.Log("reading go.mod & go.sum")
	mod, sum, err := readFiles()
	if err != nil {
		return false, err
	}

	defer repoReset(repo, logger)

	logger.Log("running go mod tidy")
	tidyCmd := exec.CommandContext(ctx, "go", "mod", "tidy")
	if err := tidyCmd.Run(); err != nil {
		panic(err)
	}

	logger.Log("checking if go.mod and go.sum have been modified")
	modified, err = repoModified(repo)
	if err != nil {
		return false, fmt.Errorf("checking for modification: %w", err)
	}

	if !modified {
		return true, nil
	}

	fmt.Println("go nodule isn't tidy")

	if !*diff {
		return false, nil
	}

	logger.Log("generating diffs")
	if err := printDiffs(mod, sum); err != nil {
		return false, fmt.Errorf("printing diffs: %w", err)
	}

	return false, nil
}

func readFiles() (mod, sum []byte, err error) {
	mod, err = os.ReadFile("go.mod")
	if err != nil {
		return nil, nil, fmt.Errorf("reading go.mod: %w", err)
	}

	sum, err = os.ReadFile("go.sum")
	if err != nil {
		return nil, nil, fmt.Errorf("reading go.sum: %w", err)
	}

	return mod, sum, nil
}

func repoReset(repo *git.Repository, logger logger) error {
	wt, err := repo.Worktree()
	if err != nil {
		logger.Log("error: getting worktree:", err)
		return fmt.Errorf("getting worktree: %w", err)
	}

	logger.Log("resetting repository")
	if err := wt.Reset(&git.ResetOptions{Mode: git.HardReset}); err != nil {
		logger.Log("error: resetting worktree:", err)
		return fmt.Errorf("resetting worktree: %w", err)
	}

	logger.Log("cleaning repository")
	if err := wt.Clean(&git.CleanOptions{Dir: true}); err != nil {
		logger.Log("error: cleaning worktree:", err)
		return fmt.Errorf("cleaning worktree: %w", err)
	}

	logger.Log("repository successfully reset")
	return nil
}

func printDiffs(mod, sum []byte) error {
	mod2, err := os.ReadFile("go.mod")
	if err != nil {
		return fmt.Errorf("reading go.mod: %w", err)
	}

	sum2, err := os.ReadFile("go.sum")
	if err != nil {
		return fmt.Errorf("reading go.sum: %w", err)
	}

	if !bytes.Equal(mod, mod2) {
		edits := myers.ComputeEdits(span.URIFromPath("go.mod"), string(mod), string(mod2))
		fmt.Println(gotextdiff.ToUnified("a/go.mod", "b/go.mod", string(mod), edits))
	}

	if !bytes.Equal(sum, sum2) {
		edits := myers.ComputeEdits(span.URIFromPath("go.sum"), string(sum), string(sum2))
		fmt.Print(gotextdiff.ToUnified("a/go.sum", "b/go.sum", string(sum), edits))
	}

	return nil
}

func repoModified(repo *git.Repository) (bool, error) {
	wt, err := repo.Worktree()
	if err != nil {
		return false, fmt.Errorf("getting worktree: %w", err)
	}

	status, err := wt.Status()
	if err != nil {
		return false, fmt.Errorf("getting status: %w", err)
	}

	return !status.IsClean(), nil
}

// fileModified checks wether the file with the given name has been modified.
func fileModified(repo *git.Repository, name string) (bool, error) {
	wt, err := repo.Worktree()
	if err != nil {
		return false, fmt.Errorf("getting worktree: %w", err)
	}

	status, err := wt.Status()
	if err != nil {
		return false, fmt.Errorf("getting status: %w", err)
	}

	entry, ok := status[name]
	if !ok {
		return false, errors.New("file not found in worktree status")
	}

	return entry.Staging != git.Unmodified, nil
}
