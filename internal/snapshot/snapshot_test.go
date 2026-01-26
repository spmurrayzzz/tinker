package snapshot

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestValidateName_Valid(t *testing.T) {
	names := []string{"abc", "abc123", "a-b_c", "A.B.C", "snap-001", "v1.0"}
	for _, name := range names {
		err := ValidateName(name)
		assert.NoError(t, err, "expected %q to be valid", name)
	}
}

func TestValidateName_Invalid(t *testing.T) {
	names := []string{"abc@def", "abc def", "abc/def", "abc\ndef", "abc\tdef"}
	for _, name := range names {
		err := ValidateName(name)
		assert.Error(t, err, "expected %q to be invalid", name)
	}
}

func TestValidateName_Length(t *testing.T) {
	shortName := makeString(127, 'a')
	err := ValidateName(shortName)
	assert.NoError(t, err, "127-char name should be valid")

	longName := makeString(128, 'a')
	err = ValidateName(longName)
	assert.Error(t, err, "128-char name should be invalid")
	assert.Contains(t, err.Error(), "too long")

	veryLongName := makeString(256, 'a')
	err = ValidateName(veryLongName)
	assert.Error(t, err, "256-char name should be invalid")

	emptyName := ""
	err = ValidateName(emptyName)
	assert.Error(t, err, "empty name should be invalid")
	assert.Contains(t, err.Error(), "cannot be empty")
}

func makeString(n int, c byte) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = c
	}
	return string(b)
}

func TestWriteAndRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	commit := "abc123"
	tasks := []TaskSnapshot{
		{
			ID:           1,
			Name:         "task1",
			Description:  "First task",
			Status:       "pending",
			Dependencies: []int64{2},
			CommitHash:   &commit,
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		},
		{
			ID:           2,
			Name:         "task2",
			Description:  "Second task",
			Status:       "completed",
			Dependencies: []int64{},
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		},
	}

	err := Write(path, tasks)
	assert.NoError(t, err)

	snap, err := Read(path)
	assert.NoError(t, err)
	assert.Equal(t, Version, snap.Version)
	assert.Len(t, snap.Tasks, 2)
	assert.Equal(t, int64(1), snap.Tasks[0].ID)
	assert.Equal(t, "task1", snap.Tasks[0].Name)
	assert.Equal(t, "pending", snap.Tasks[0].Status)
	assert.Equal(t, []int64{2}, snap.Tasks[0].Dependencies)
	assert.NotNil(t, snap.Tasks[0].CommitHash)
	assert.Equal(t, "abc123", *snap.Tasks[0].CommitHash)
}

func TestWrite_InvalidName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid@name.json")

	err := Write(path, []TaskSnapshot{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid snapshot name")
}

func TestRead_VersionMismatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "version.json")

	data := `{"version": 99, "created_at": "2024-01-01T00:00:00Z", "tasks": []}`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := Read(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported snapshot version")
}

func TestExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exists.json")

	assert.False(t, Exists(path))

	if err := os.WriteFile(path, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}

	assert.True(t, Exists(path))
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "delete.json")

	if err := os.WriteFile(path, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}

	assert.True(t, Exists(path))

	err := Delete(path)
	assert.NoError(t, err)
	assert.False(t, Exists(path))
}

func TestList(t *testing.T) {
	dir := t.TempDir()

	paths := []string{
		"snap1.json",
		"snap2.json",
		"notjson.txt",
		"subdir/snap3.json",
	}
	for _, p := range paths {
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("{}"), 0600); err != nil {
			t.Fatal(err)
		}
	}

	names, err := List(dir)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"snap1", "snap2"}, names)
}

func TestList_Empty(t *testing.T) {
	dir := t.TempDir()

	names, err := List(dir)
	assert.NoError(t, err)
	assert.Empty(t, names)
}

func TestList_DirNotExist(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "notexist")

	names, err := List(dir)
	assert.NoError(t, err)
	assert.Empty(t, names)
}
