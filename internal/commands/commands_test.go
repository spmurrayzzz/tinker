package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"tinker/internal/xdg"
)

func TestParseTagExpression(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		include []string
		exclude []string
		wantErr bool
	}{
		{"empty", "", nil, nil, false},
		{"single include", "+feature", []string{"feature"}, nil, false},
		{"single exclude", "-wip", nil, []string{"wip"}, false},
		{"no prefix treated as include", "feature", []string{"feature"}, nil, false},
		{"mixed", "+a,-b,c", []string{"a", "c"}, []string{"b"}, false},
		{"empty include tag", "+", nil, nil, true},
		{"empty exclude tag", "-", nil, nil, true},
		{"multiple includes", "+a,+b,+c", []string{"a", "b", "c"}, nil, false},
		{"multiple excludes", "-a,-b,-c", nil, []string{"a", "b", "c"}, false},
		{"whitespace handling", " +a , -b , c ", []string{"a", "c"}, []string{"b"}, false},
		{"empty parts skipped", "+a,,+b", []string{"a", "b"}, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inc, exc, err := ParseTagExpression(tt.expr)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.include, inc)
			assert.Equal(t, tt.exclude, exc)
		})
	}
}

func TestParseTaskID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{"simple", "123", 123, false},
		{"zero padded", "00001", 1, false},
		{"large", "99999", 99999, false},
		{"negative", "-1", -1, false},
		{"invalid", "abc", 0, true},
		{"empty", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTaskID(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseDependsOn(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []int64
		wantErr bool
	}{
		{"empty", "", nil, false},
		{"single", "1", []int64{1}, false},
		{"multiple", "1,2,3", []int64{1, 2, 3}, false},
		{"zero padded", "001,002", []int64{1, 2}, false},
		{"invalid", "1,abc,3", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDependsOn(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatTime(t *testing.T) {
	ts := int64(1704067200)
	formatted := FormatTime(ts)
	assert.Equal(t, "2024-01-01T00:00:00Z", formatted)
}

func TestFormatTaskID(t *testing.T) {
	tests := []struct {
		id    int64
		width int
		want  string
	}{
		{1, 5, "1"},
		{123, 5, "123"},
		{99999, 5, "99999"},
	}

	for _, tt := range tests {
		got := FormatTaskID(tt.id, tt.width)
		assert.Equal(t, tt.want, got)
	}
}

func TestSplitComma(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"a", []string{"a"}},
		{"a,b,c", []string{"a", "b", "c"}},
		{"a,,b", []string{"a", "", "b"}},
	}

	for _, tt := range tests {
		got := splitComma(tt.input)
		assert.Equal(t, tt.want, got)
	}
}

func TestListSnapshots_SortsAndFilters(t *testing.T) {
	originalDataHome := xdg.DataHome
	xdg.DataHome = t.TempDir()
	t.Cleanup(func() {
		xdg.DataHome = originalDataHome
	})

	projectKey := "test-project"
	snapshotDir := xdg.ProjectSnapshotsDir(projectKey)

	err := os.MkdirAll(snapshotDir, 0700)
	assert.NoError(t, err)

	files := []string{
		filepath.Join(snapshotDir, "b.json"),
		filepath.Join(snapshotDir, "a.json"),
		filepath.Join(snapshotDir, "ignore.txt"),
	}

	for _, path := range files {
		assert.NoError(t, os.WriteFile(path, []byte("{}"), 0600))
	}

	subdir := filepath.Join(snapshotDir, "subdir")
	assert.NoError(t, os.MkdirAll(subdir, 0700))
	assert.NoError(t, os.WriteFile(
		filepath.Join(subdir, "c.json"),
		[]byte("{}"),
		0600,
	))

	ctx := &ProjectContext{ProjectKey: projectKey}
	got, err := ListSnapshots(ctx)
	assert.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, got)
}

func TestListSnapshots_EmptyWhenDirMissing(t *testing.T) {
	originalDataHome := xdg.DataHome
	xdg.DataHome = t.TempDir()
	t.Cleanup(func() {
		xdg.DataHome = originalDataHome
	})

	ctx := &ProjectContext{ProjectKey: "test-project"}
	got, err := ListSnapshots(ctx)
	assert.NoError(t, err)
	assert.Empty(t, got)
}

func TestParseIDExpression(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		expected []int64
		wantErr  bool
	}{
		{"single unpadded", "5", []int64{5}, false},
		{"single padded", "00005", []int64{5}, false},
		{"comma list", "1,3,5", []int64{1, 3, 5}, false},
		{"simple range", "1-3", []int64{1, 2, 3}, false},
		{"padded range", "00001-00003", []int64{1, 2, 3}, false},
		{"mixed", "1,3-5,10", []int64{1, 3, 4, 5, 10}, false},
		{"complex mixed", "1,3,5-10,15,20-22", []int64{1, 3, 5, 6, 7, 8, 9, 10, 15, 20, 21, 22}, false},
		{"overlapping ranges", "1-5,3-7", []int64{1, 2, 3, 4, 5, 6, 7}, false},
		{"empty", "", nil, true},
		{"invalid range", "5-1", nil, true},
		{"invalid id", "abc", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseIDExpression(tt.expr)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}
