package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"tinker/internal/xdg"
)

type GlobalConfig struct {
	Version int `json:"version"`
	IDWidth int `json:"id_width"`
}

type ProjectConfig struct {
	Version    int    `json:"version"`
	ProjectKey string `json:"project_key"`
	GitRoot    string `json:"git_root"`
}

func ReadGlobalConfig() (*GlobalConfig, error) {
	path := xdg.GlobalConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf(
				"global config not found at %s: %w",
				path,
				err,
			)
		}
		return nil, fmt.Errorf("read global config: %w", err)
	}
	var cfg GlobalConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse global config: %w", err)
	}
	return &cfg, nil
}

func WriteGlobalConfig(cfg *GlobalConfig) error {
	path := xdg.GlobalConfigPath()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal global config: %w", err)
	}
	return atomicWrite(path, data, 0600)
}

func ReadProjectConfig(projectKey string) (*ProjectConfig, error) {
	path := filepath.Join(xdg.ProjectConfigDir(projectKey), "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf(
				"project config not found for %s: %w",
				projectKey,
				err,
			)
		}
		return nil, fmt.Errorf("read project config: %w", err)
	}
	var cfg ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse project config: %w", err)
	}
	return &cfg, nil
}

func WriteProjectConfig(cfg *ProjectConfig) error {
	dir := xdg.ProjectConfigDir(cfg.ProjectKey)
	if err := xdg.EnsureDir(dir); err != nil {
		return fmt.Errorf("ensure config dir: %w", err)
	}
	path := filepath.Join(dir, "config.json")
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal project config: %w", err)
	}
	return atomicWrite(path, data, 0600)
}

func atomicWrite(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := xdg.EnsureDir(dir); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod temp: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	return os.Rename(tmpPath, path)
}

func normalizeQuickstartMode(mode string) (string, error) {
	switch mode {
	case "":
		return "", nil
	case "append", "replace":
		return mode, nil
	default:
		return "", fmt.Errorf(
			"invalid quickstart_mode %q (allowed: append, replace)",
			mode,
		)
	}
}
