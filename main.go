package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
)

// Version is the git reference injected at build
var Version string

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	tidy, err := run(ctx)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	if !tidy {
		os.Exit(1)
	}
}

func run(ctx context.Context) (bool, error) {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "go-tidy-check checks if your modules are tidy.\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: \n  %s [flags] [PATH ...]\n\n", os.Args[0])
		fmt.Fprintln(flag.CommandLine.Output(), "Flags:")
		flag.PrintDefaults()
	}
	verbose := flag.Bool("v", false, "verbose debug output")
	version := flag.Bool("version", false, "print version and exit")
	diff := flag.Bool("d", false, "print diffs")
	flag.Parse()
	paths := flag.Args()

	if *version {
		fmt.Printf("go-tidy-check %s\n", Version)
		return true, nil
	}

	var logger logger
	if *verbose {
		logger = debugLogger{}
	} else {
		logger = nopLogger{}
	}

	if len(paths) == 0 {
		paths = append(paths, "")
	}

	var result bool
	for _, path := range paths {
		tidy, err := check(ctx, path, *diff, logger)
		if err != nil {
			return false, fmt.Errorf("checking module %q: %w", path, err)
		}
		result = result || tidy
	}

	return result, nil
}

func check(ctx context.Context, path string, diff bool, logger logger) (bool, error) {
	if strings.HasSuffix(path, "/...") {
		logger.Log("timming trailing /... from module path %q", path)
		path = strings.TrimSuffix(path, "/...")
	}

	logger.Log("checking module %q", path)
	modPath := filepath.Join(path, "go.mod")
	sumPath := filepath.Join(path, "go.sum")

	logger.Log("opening repository")
	repo, err := git.PlainOpenWithOptions(path, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return false, fmt.Errorf("opening repo: %w", err)
	}

	logger.Log("checking if repository is modified")
	modified, err := repoModified(repo)
	if err != nil {
		return false, fmt.Errorf("checking for existing modification: %w", err)
	}
	if modified {
		return false, errors.New("repo has uncommitted changes")
	}

	logger.Log("reading %q & %q", modPath, sumPath)
	mod, sum, err := readFiles(modPath, sumPath)
	if err != nil {
		return false, err
	}

	defer repoReset(repo, logger)

	logger.Log("running go mod tidy")
	tidyCmd := exec.CommandContext(ctx, "go", "mod", "tidy")
	tidyCmd.Dir = path
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

	var pathOut string
	if path != "" {
		pathOut = path
		if !strings.HasPrefix(pathOut, "/") && !strings.HasPrefix(pathOut, ".") {
			pathOut = "./" + pathOut
		}
		pathOut = fmt.Sprintf(" in %q", pathOut)
	}
	fmt.Printf("go module%s isn't tidy\n", pathOut)

	if !diff {
		return false, nil
	}

	logger.Log("generating diffs")
	if err := printDiffs(modPath, sumPath, mod, sum); err != nil {
		return false, fmt.Errorf("printing diffs: %w", err)
	}

	return false, nil
}

func readFiles(modPath, sumPath string) (mod, sum []byte, err error) {
	mod, err = os.ReadFile(modPath)
	if err != nil {
		return nil, nil, fmt.Errorf("reading %q: %w", modPath, err)
	}

	sum, err = os.ReadFile(sumPath)
	if err != nil {
		return nil, nil, fmt.Errorf("reading %q: %w", sumPath, err)
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

func printDiffs(modPath, sumPath string, mod, sum []byte) error {
	mod2, sum2, err := readFiles(modPath, sumPath)
	if err != nil {
		return err
	}

	if !bytes.Equal(mod, mod2) {
		edits := myers.ComputeEdits(span.URIFromPath("go.mod"), string(mod), string(mod2))
		fmt.Println(gotextdiff.ToUnified(
			fmt.Sprintf("a/%s", modPath),
			fmt.Sprintf("b/%s", modPath),
			string(mod),
			edits,
		))

	}

	if !bytes.Equal(sum, sum2) {
		edits := myers.ComputeEdits(span.URIFromPath("go.sum"), string(sum), string(sum2))
		fmt.Print(gotextdiff.ToUnified(
			fmt.Sprintf("a/%s", sumPath),
			fmt.Sprintf("b/%s", sumPath),
			string(sum),
			edits,
		))
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
