package xdg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDataHome(t *testing.T) {
	orig := os.Getenv("XDG_DATA_HOME")
	defer os.Setenv("XDG_DATA_HOME", orig)

	os.Unsetenv("XDG_DATA_HOME")
	DataHome = getDataHome()
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".local", "share")
	if DataHome != expected {
		t.Errorf("expected %s, got %s", expected, DataHome)
	}

	os.Setenv("XDG_DATA_HOME", "/custom/data")
	DataHome = getDataHome()
	if DataHome != "/custom/data" {
		t.Errorf("expected /custom/data, got %s", DataHome)
	}
}

func TestConfigHome(t *testing.T) {
	orig := os.Getenv("XDG_CONFIG_HOME")
	defer os.Setenv("XDG_CONFIG_HOME", orig)

	os.Unsetenv("XDG_CONFIG_HOME")
	ConfigHome = getConfigHome()
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".config")
	if ConfigHome != expected {
		t.Errorf("expected %s, got %s", expected, ConfigHome)
	}

	os.Setenv("XDG_CONFIG_HOME", "/custom/config")
	ConfigHome = getConfigHome()
	if ConfigHome != "/custom/config" {
		t.Errorf("expected /custom/config, got %s", ConfigHome)
	}
}

func TestProjectPaths(t *testing.T) {
	key := "testproject-abc123"
	if got := ProjectDataDir(key); got != filepath.Join(DataHome, "tinker", "projects", key) {
		t.Errorf("ProjectDataDir = %s, want %s", got, filepath.Join(DataHome, "tinker", "projects", key))
	}
	if got := ProjectSnapshotsDir(key); got != filepath.Join(DataHome, "tinker", "projects", key, "snapshots") {
		t.Errorf("ProjectSnapshotsDir = %s, want %s", got, filepath.Join(DataHome, "tinker", "projects", key, "snapshots"))
	}
	if got := ProjectConfigDir(key); got != filepath.Join(ConfigHome, "tinker", "projects", key) {
		t.Errorf("ProjectConfigDir = %s, want %s", got, filepath.Join(ConfigHome, "tinker", "projects", key))
	}
}

func TestEnsureDir(t *testing.T) {
	tmp := t.TempDir()
	testPath := filepath.Join(tmp, "a", "b", "c")

	if err := EnsureDir(testPath); err != nil {
		t.Fatalf("EnsureDir failed: %v", err)
	}

	info, err := os.Stat(testPath)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory")
	}
	if info.Mode().Perm() != 0700 {
		t.Errorf("expected permissions 0700, got %o", info.Mode().Perm())
	}
}

func TestWriteFile(t *testing.T) {
	tmp := t.TempDir()
	testPath := filepath.Join(tmp, "subdir", "file.txt")

	if err := WriteFile(testPath, []byte("hello")); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	data, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("expected 'hello', got %s", string(data))
	}

	info, err := os.Stat(testPath)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected permissions 0600, got %o", info.Mode().Perm())
	}
}

func TestGlobalConfigPath(t *testing.T) {
	expected := filepath.Join(ConfigHome, "tinker", "config.json")
	if got := GlobalConfigPath(); got != expected {
		t.Errorf("GlobalConfigPath = %s, want %s", got, expected)
	}
}
