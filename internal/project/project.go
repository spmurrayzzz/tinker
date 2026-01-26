package project

import (
	"fmt"
	"path/filepath"
	"strings"
)

func DeriveKey(gitRoot string) (string, error) {
	canonical, err := filepath.EvalSymlinks(gitRoot)
	if err != nil {
		return "", fmt.Errorf("canonicalize path: %w", err)
	}
	abs, err := filepath.Abs(canonical)
	if err != nil {
		return "", fmt.Errorf("absolute path: %w", err)
	}
	key := strings.ReplaceAll(abs, "/", "-")
	return strings.Trim(key, "-"), nil
}
