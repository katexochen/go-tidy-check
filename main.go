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

	logger.Log("reading %q & %q", modPath, sumPath)
	mod, sum, err := readFiles(modPath, sumPath, logger)
	if err != nil {
		return false, err
	}

	defer restoreFiles(modPath, sumPath, mod, sum, logger)

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

	logger.Log("reading %q & %q again", modPath, sumPath)
	mod2, sum2, err := readFiles(modPath, sumPath, logger)
	if err != nil {
		return false, err
	}

	if !modified(mod, mod2, sum, sum2) {
		logger.Log("%q and %q are tidy", modPath, sumPath)
		return false, nil
	}
	logger.Log("%q and %q are not tidy", modPath, sumPath)

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
	if err := printDiffs(modPath, sumPath, mod, mod2, sum, sum2, logger); err != nil {
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

func restoreFiles(modPath, sumPath string, mod, sum []byte, logger logger) error {
	if err := os.WriteFile(modPath, mod, 0); err != nil {
		return fmt.Errorf("writing %q: %w", modPath, err)
	}
	logger.Log("restored %q", modPath)

	if len(sum) == 0 {
		logger.Log("removing %q, as it didn't exist before", sumPath)

		if err := os.Remove(sumPath); err != nil {
			return fmt.Errorf("removing %q: %w", sumPath, err)
		}

		return nil
	}

	if err := os.WriteFile(sumPath, sum, 0); err != nil {
		return fmt.Errorf("writing %q: %w", sumPath, err)
	}
	logger.Log("restored %q", sumPath)

	return nil
}

func printDiffs(modPath, sumPath string, mod1, mod2, sum1, sum2 []byte, logger logger) error {
	if !bytes.Equal(mod1, mod2) {
		edits := myers.ComputeEdits(span.URIFromPath("go.mod"), string(mod1), string(mod2))
		fmt.Println(gotextdiff.ToUnified(
			fmt.Sprintf("a/%s", modPath),
			fmt.Sprintf("b/%s", modPath),
			string(mod1),
			edits,
		))

	}

	if !bytes.Equal(sum1, sum2) {
		edits := myers.ComputeEdits(span.URIFromPath("go.sum"), string(sum1), string(sum2))
		fmt.Print(gotextdiff.ToUnified(
			fmt.Sprintf("a/%s", sumPath),
			fmt.Sprintf("b/%s", sumPath),
			string(sum1),
			edits,
		))
	}

	return nil
}

func modified(mod1, mod2, sum1, sum2 []byte) bool {
	return !bytes.Equal(mod1, mod2) || !bytes.Equal(sum1, sum2)
}
