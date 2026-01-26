package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func GitRoot(dir string) (string, error) {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("%s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func IsAncestor(gitRoot, commit string) (bool, error) {
	cmd := exec.Command("git", "-C", gitRoot, "merge-base", "--is-ancestor", commit, "HEAD")
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("%s", strings.TrimSpace(string(exitErr.Stderr)))
	}
	return false, err
}

func CanonicalPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}
