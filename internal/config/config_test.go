package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"tinker/internal/xdg"
)

func TestGlobalConfig(t *testing.T) {
	tmpDir := t.TempDir()
	xdgDataHome := filepath.Join(tmpDir, "data")
	xdgConfigHome := filepath.Join(tmpDir, "config")

	origDataHome := os.Getenv("XDG_DATA_HOME")
	origConfigHome := os.Getenv("XDG_CONFIG_HOME")
	origXdgDataHome := xdg.DataHome
	origXdgConfigHome := xdg.ConfigHome
	defer func() {
		os.Setenv("XDG_DATA_HOME", origDataHome)
		os.Setenv("XDG_CONFIG_HOME", origConfigHome)
		xdg.DataHome = origXdgDataHome
		xdg.ConfigHome = origXdgConfigHome
	}()

	os.Setenv("XDG_DATA_HOME", xdgDataHome)
	os.Setenv("XDG_CONFIG_HOME", xdgConfigHome)
	xdg.DataHome = xdgDataHome
	xdg.ConfigHome = xdgConfigHome

	cfg := &GlobalConfig{
		Version:        1,
		IDWidth:        5,
		QuickstartMode: "append",
	}
	err := WriteGlobalConfig(cfg)
	if err != nil {
		t.Fatalf("WriteGlobalConfig: %v", err)
	}

	readCfg, err := ReadGlobalConfig()
	if err != nil {
		t.Fatalf("ReadGlobalConfig: %v", err)
	}
	if readCfg.Version != 1 || readCfg.IDWidth != 5 ||
		readCfg.QuickstartMode != "append" {
		t.Errorf("got %+v, want quickstart_mode append", readCfg)
	}
}

func TestProjectConfig(t *testing.T) {
	tmpDir := t.TempDir()
	xdgDataHome := filepath.Join(tmpDir, "data")
	xdgConfigHome := filepath.Join(tmpDir, "config")

	origDataHome := os.Getenv("XDG_DATA_HOME")
	origConfigHome := os.Getenv("XDG_CONFIG_HOME")
	origXdgDataHome := xdg.DataHome
	origXdgConfigHome := xdg.ConfigHome
	defer func() {
		os.Setenv("XDG_DATA_HOME", origDataHome)
		os.Setenv("XDG_CONFIG_HOME", origConfigHome)
		xdg.DataHome = origXdgDataHome
		xdg.ConfigHome = origXdgConfigHome
	}()

	os.Setenv("XDG_DATA_HOME", xdgDataHome)
	os.Setenv("XDG_CONFIG_HOME", xdgConfigHome)
	xdg.DataHome = xdgDataHome
	xdg.ConfigHome = xdgConfigHome

	projectKey := "testproj-abc12345"
	cfg := &ProjectConfig{
		Version:        1,
		ProjectKey:     projectKey,
		GitRoot:        "/foo/bar",
		QuickstartMode: "replace",
	}
	err := WriteProjectConfig(cfg)
	if err != nil {
		t.Fatalf("WriteProjectConfig: %v", err)
	}

	readCfg, err := ReadProjectConfig(projectKey)
	if err != nil {
		t.Fatalf("ReadProjectConfig: %v", err)
	}
	if readCfg.Version != 1 || readCfg.ProjectKey != projectKey ||
		readCfg.GitRoot != "/foo/bar" ||
		readCfg.QuickstartMode != "replace" {
		t.Errorf("got %+v", readCfg)
	}
}

func TestReadGlobalConfigMissing(t *testing.T) {
	tmpDir := t.TempDir()
	xdgDataHome := filepath.Join(tmpDir, "data")
	xdgConfigHome := filepath.Join(tmpDir, "config")

	origDataHome := os.Getenv("XDG_DATA_HOME")
	origConfigHome := os.Getenv("XDG_CONFIG_HOME")
	origXdgDataHome := xdg.DataHome
	origXdgConfigHome := xdg.ConfigHome
	defer func() {
		os.Setenv("XDG_DATA_HOME", origDataHome)
		os.Setenv("XDG_CONFIG_HOME", origConfigHome)
		xdg.DataHome = origXdgDataHome
		xdg.ConfigHome = origXdgConfigHome
	}()

	os.Setenv("XDG_DATA_HOME", xdgDataHome)
	os.Setenv("XDG_CONFIG_HOME", xdgConfigHome)
	xdg.DataHome = xdgDataHome
	xdg.ConfigHome = xdgConfigHome

	_, err := ReadGlobalConfig()
	if err == nil {
		t.Fatal("expected missing config error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist, got %v", err)
	}
}

func TestReadProjectConfigMissing(t *testing.T) {
	tmpDir := t.TempDir()
	xdgDataHome := filepath.Join(tmpDir, "data")
	xdgConfigHome := filepath.Join(tmpDir, "config")

	origDataHome := os.Getenv("XDG_DATA_HOME")
	origConfigHome := os.Getenv("XDG_CONFIG_HOME")
	origXdgDataHome := xdg.DataHome
	origXdgConfigHome := xdg.ConfigHome
	defer func() {
		os.Setenv("XDG_DATA_HOME", origDataHome)
		os.Setenv("XDG_CONFIG_HOME", origConfigHome)
		xdg.DataHome = origXdgDataHome
		xdg.ConfigHome = origXdgConfigHome
	}()

	os.Setenv("XDG_DATA_HOME", xdgDataHome)
	os.Setenv("XDG_CONFIG_HOME", xdgConfigHome)
	xdg.DataHome = xdgDataHome
	xdg.ConfigHome = xdgConfigHome

	_, err := ReadProjectConfig("missing-project")
	if err == nil {
		t.Fatal("expected missing config error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist, got %v", err)
	}
}
