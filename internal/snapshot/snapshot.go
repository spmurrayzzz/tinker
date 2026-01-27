package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const Version = 1

const MaxSnapshotNameLength = 127

type TaskSnapshot struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Status       string    `json:"status"`
	Tags         []string  `json:"tags,omitempty"`
	Archived     bool      `json:"archived"`
	Dependencies []int64   `json:"dependencies"`
	CommitHash   *string   `json:"commit_hash,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Snapshot struct {
	Version   int            `json:"version"`
	CreatedAt time.Time      `json:"created_at"`
	Tasks     []TaskSnapshot `json:"tasks"`
}

func ValidateName(name string) error {
	if len(name) == 0 {
		return fmt.Errorf("snapshot name cannot be empty")
	}
	if len(name) > MaxSnapshotNameLength {
		return fmt.Errorf("snapshot name too long (max %d characters)", MaxSnapshotNameLength)
	}
	for _, c := range name {
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-') {
			return fmt.Errorf("invalid snapshot name %q: must contain only alphanumeric, dots, underscores, and hyphens", name)
		}
	}
	return nil
}

func Write(path string, tasks []TaskSnapshot) error {
	if err := ValidateName(filepath.Base(path)); err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	tmp, err := os.CreateTemp(dir, "*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()

	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	snap := Snapshot{
		Version:   Version,
		CreatedAt: time.Now().UTC(),
		Tasks:     tasks,
	}
	if err := enc.Encode(snap); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("encode: %w", err)
	}
	tmp.Close()

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename: %w", err)
	}

	return nil
}

func Read(path string) (*Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	if snap.Version != Version {
		return nil, fmt.Errorf("unsupported snapshot version: %d (expected %d)", snap.Version, Version)
	}

	return &snap, nil
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func Delete(path string) error {
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove: %w", err)
	}
	return nil
}

func List(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("read dir: %w", err)
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			name := strings.TrimSuffix(e.Name(), ".json")
			names = append(names, name)
		}
	}
	return names, nil
}
