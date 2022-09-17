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

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "go-tidy-check checks if your modules are tidy.\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: \n  %s [flags] [PATH ...]\n\n", os.Args[0])
		fmt.Fprintln(flag.CommandLine.Output(), "Flags:")
		flag.PrintDefaults()
	}
	flags := flags{
		verbose: flag.Bool("v", false, "verbose debug output"),
		version: flag.Bool("version", false, "print version and exit"),
		diff:    flag.Bool("d", false, "print diffs"),
	}
	flag.Parse()
	args := flag.Args()

	untidy, err := run(ctx, flags, args)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	if untidy {
		os.Exit(1)
	}
}

type flags struct {
	diff    *bool
	verbose *bool
	version *bool
}

func run(ctx context.Context, flags flags, paths []string) (bool, error) {
	if *flags.version {
		fmt.Printf("go-tidy-check %s\n", Version)
		return false, nil
	}

	var logger logger
	if *flags.verbose {
		logger = debugLogger{}
	} else {
		logger = nopLogger{}
	}

	if runningAsGitHubAction() {
		var err error
		paths, err = pathsInsideContainer(paths)
		if err != nil {
			return false, fmt.Errorf("getting paths inside container: %w", err)
		}
	}

	if len(paths) == 0 {
		paths = append(paths, "")
	}

	var result bool
	for _, path := range paths {
		untidy, err := check(ctx, path, *flags.diff, logger)
		if err != nil {
			return false, fmt.Errorf("checking module %q: %w", path, err)
		}
		result = result || untidy
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
	mod, sum, err := readFiles(modPath, sumPath, logger)
	if err != nil {
		return false, err
	}

	defer repoReset(repo, logger)

	logger.Log("running go mod tidy")
	tidyCmd := exec.CommandContext(ctx, "go", "mod", "tidy")
	tidyCmd.Dir = path
	out, err := tidyCmd.CombinedOutput()
	var tidyErr *exec.ExitError
	if errors.As(err, &tidyErr) {
		return false, fmt.Errorf("running 'go mod tidy': %w; %s", tidyErr, out)
	} else if err != nil {
		return false, fmt.Errorf("running 'go mod tidy': %w", err)
	}

	logger.Log("checking if go.mod and go.sum have been modified")
	modified, err = repoModified(repo)
	if err != nil {
		return false, fmt.Errorf("checking for modification: %w", err)
	}

	if !modified {
		return false, nil
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
		return true, nil
	}

	logger.Log("generating diffs")
	if err := printDiffs(modPath, sumPath, mod, sum, logger); err != nil {
		return false, fmt.Errorf("printing diffs: %w", err)
	}

	return true, nil
}

func readFiles(modPath, sumPath string, logger logger) (mod, sum []byte, err error) {
	mod, err = os.ReadFile(modPath)
	if err != nil {
		return nil, nil, fmt.Errorf("reading %q: %w", modPath, err)
	}

	sum, err = os.ReadFile(sumPath)
	if errors.Is(err, os.ErrNotExist) {
		logger.Log("%q does not exist, using empty string", sumPath)
		return mod, []byte{}, nil
	}
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

func printDiffs(modPath, sumPath string, mod, sum []byte, logger logger) error {
	mod2, sum2, err := readFiles(modPath, sumPath, logger)
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
