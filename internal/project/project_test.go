package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeriveKey(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "myproject-123")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	canonicalDir, err := filepath.EvalSymlinks(absDir)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}

	key, err := DeriveKey(dir)
	if err != nil {
		t.Fatalf("DeriveKey: %v", err)
	}

	expectedKey := strings.Trim(strings.ReplaceAll(canonicalDir, "/", "-"), "-")
	if key != expectedKey {
		t.Errorf("expected '%s', got '%s'", expectedKey, key)
	}
}

func TestDeriveKeySpaces(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "my project")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	canonicalDir, err := filepath.EvalSymlinks(absDir)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}

	key, err := DeriveKey(dir)
	if err != nil {
		t.Fatalf("DeriveKey: %v", err)
	}

	expectedKey := strings.Trim(strings.ReplaceAll(canonicalDir, "/", "-"), "-")
	if key != expectedKey {
		t.Errorf("expected '%s', got '%s'", expectedKey, key)
	}
}

func TestDeriveKeyConsistency(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "consistent")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	key1, err := DeriveKey(dir)
	if err != nil {
		t.Fatalf("DeriveKey first call: %v", err)
	}
	key2, err := DeriveKey(dir)
	if err != nil {
		t.Fatalf("DeriveKey second call: %v", err)
	}

	if key1 != key2 {
		t.Errorf("inconsistent keys: %s != %s", key1, key2)
	}
}

func TestDeriveKeySymlink(t *testing.T) {
	tmp := t.TempDir()
	realDir := filepath.Join(tmp, "realproject")
	linkDir := filepath.Join(tmp, "linkproject")

	if err := os.MkdirAll(realDir, 0755); err != nil {
		t.Fatalf("mkdir real: %v", err)
	}
	if err := os.Symlink(realDir, linkDir); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	realKey, err := DeriveKey(realDir)
	if err != nil {
		t.Fatalf("DeriveKey real: %v", err)
	}
	linkKey, err := DeriveKey(linkDir)
	if err != nil {
		t.Fatalf("DeriveKey link: %v", err)
	}

	if realKey != linkKey {
		t.Errorf("symlink should produce same key: %s != %s", realKey, linkKey)
	}
}
