package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitRoot(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := GitRoot(tmpDir)
	if err == nil {
		t.Error("expected error for non-git directory")
	}

	if err := exec.Command("git", "init", tmpDir).Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	root, err := GitRoot(tmpDir)
	if err != nil {
		t.Fatalf("git root failed: %v", err)
	}
	if !samePath(root, tmpDir) {
		t.Errorf("expected %s, got %s", tmpDir, root)
	}
}

func TestIsAncestor(t *testing.T) {
	tmpDir := t.TempDir()

	if err := exec.Command("git", "init", tmpDir).Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	if err := exec.Command("git", "-C", tmpDir, "add", ".").Run(); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if err := exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run(); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	_, err := IsAncestor(tmpDir, "abc123")
	if err == nil {
		t.Error("expected error for invalid commit")
	}

	isAnc, err := IsAncestor(tmpDir, "HEAD")
	if err != nil {
		t.Fatalf("is ancestor failed: %v", err)
	}
	if !isAnc {
		t.Error("HEAD should be ancestor of HEAD")
	}
}

func TestCanonicalPath(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("create subdir: %v", err)
	}

	canonical, err := CanonicalPath(subDir)
	if err != nil {
		t.Fatalf("canonical path failed: %v", err)
	}
	if !samePath(canonical, subDir) {
		t.Errorf("expected %s, got %s", subDir, canonical)
	}
}

func samePath(a, b string) bool {
	if a == b {
		return true
	}
	a = strings.TrimPrefix(a, "/private")
	b = strings.TrimPrefix(b, "/private")
	return a == b
}
