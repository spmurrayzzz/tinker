package xdg

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

var (
	DataHome   string
	ConfigHome string
)

func init() {
	DataHome = getDataHome()
	ConfigHome = getConfigHome()
}

func getDataHome() string {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.Getenv("HOME"), ".local", "share")
	}
	return filepath.Join(home, ".local", "share")
}

func getConfigHome() string {
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(home, ".config")
}

func ProjectDataDir(projectKey string) string {
	return filepath.Join(DataHome, "tinker", "projects", projectKey)
}

func ProjectSnapshotsDir(projectKey string) string {
	return filepath.Join(ProjectDataDir(projectKey), "snapshots")
}

func ProjectConfigDir(projectKey string) string {
	return filepath.Join(ConfigHome, "tinker", "projects", projectKey)
}

func GlobalConfigPath() string {
	return filepath.Join(ConfigHome, "tinker", "config.json")
}

func EnsureDir(path string) error {
	return os.MkdirAll(path, 0700)
}

func WriteFile(path string, data []byte) error {
	if err := EnsureDir(filepath.Dir(path)); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func LockFile(associatedPath string) (*os.File, error) {
	lockDir := filepath.Dir(associatedPath)
	if err := EnsureDir(lockDir); err != nil {
		return nil, fmt.Errorf("ensure lock dir: %w", err)
	}
	lockPath := filepath.Join(lockDir, "tinker.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, fmt.Errorf("acquire lock: %w", err)
	}
	return f, nil
}
