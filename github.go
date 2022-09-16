package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	mountInfoPath             = "/proc/self/mountinfo"
	githubContainerMountPoint = "/github/workspace"
)

var argsReg = regexp.MustCompile(`("[^"]*"|[^"\s]+)(\s+|$)`)

func runningAsGitHubAction() bool {
	return os.Getenv("GITHUB_ACTION_REPOSITORY") == "katexochen/go-tidy-check"
}

func pathsInsideContainer(paths []string) ([]string, error) {
	// When only a single path is passed, try to parse the path argument as
	// multi-argument string. This might be the case when running as a GitHub action.
	if len(paths) == 1 {
		paths = parseArgsFromString(paths[0])
	}

	// If the passed paths are absolute paths from outside the container, we need to
	// convert them to paths inside the container.
	mountSource, err := mountSourceDir()
	if err != nil {
		return nil, fmt.Errorf("getting mount source dir: %w", err)
	}
	for i, path := range paths {
		if strings.HasPrefix(path, mountSource) {
			newPath := filepath.Join(githubContainerMountPoint, path[len(mountSource):])
			paths[i] = newPath
			fmt.Printf("replacing path %q with %q\n", path, newPath)
		}
	}

	return paths, nil
}

func mountSourceDir() (string, error) {
	mountInfo, err := os.Open(mountInfoPath)
	if err != nil {
		return "", fmt.Errorf("reading %q: %w", mountInfoPath, err)
	}

	lines := bufio.NewScanner(mountInfo)
	var mountSource string
	for lines.Scan() {
		line := lines.Text()
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		mountPoint := fields[4]
		if mountPoint != githubContainerMountPoint {
			continue
		}

		mountSource = fields[3]
		break
	}

	return mountSource, nil
}

// https://stackoverflow.com/q/43536673
func parseArgsFromString(multiArgStr string) []string {
	argStrs := argsReg.FindAllString(multiArgStr, -1)
	args := make([]string, len(argStrs))
	for i, v := range argStrs {
		args[i] = strings.TrimSpace(v)
	}
	return args
}
