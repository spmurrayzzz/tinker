package db

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tinker/internal/model"
	"tinker/internal/xdg"
)

func setupTestDB(t *testing.T) *DB {
	tmpDir := t.TempDir()
	origDataHome := os.Getenv("XDG_DATA_HOME")
	origConfigHome := os.Getenv("XDG_CONFIG_HOME")
	origXdgDataHome := xdg.DataHome
	origXdgConfigHome := xdg.ConfigHome
	t.Cleanup(func() {
		os.Setenv("XDG_DATA_HOME", origDataHome)
		os.Setenv("XDG_CONFIG_HOME", origConfigHome)
		xdg.DataHome = origXdgDataHome
		xdg.ConfigHome = origXdgConfigHome
	})
	os.Setenv("XDG_DATA_HOME", tmpDir)
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	xdg.DataHome = tmpDir
	xdg.ConfigHome = tmpDir

	db, err := Open("test-project")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestTaskTagOperations(t *testing.T) {
	db := setupTestDB(t)

	id, err := db.InsertTask("test", "desc", model.StatusPending, nil)
	require.NoError(t, err)

	err = db.AddTag(id, "feature")
	require.NoError(t, err)

	tags, err := db.GetTaskTags(id)
	require.NoError(t, err)
	assert.Equal(t, []string{"feature"}, tags)

	err = db.AddTag(id, "feature")
	require.NoError(t, err)
	tags, err = db.GetTaskTags(id)
	require.NoError(t, err)
	assert.Equal(t, []string{"feature"}, tags)

	err = db.AddTag(id, "urgent")
	require.NoError(t, err)
	tags, err = db.GetTaskTags(id)
	require.NoError(t, err)
	assert.Equal(t, []string{"feature", "urgent"}, tags)

	err = db.RemoveTag(id, "feature")
	require.NoError(t, err)
	tags, err = db.GetTaskTags(id)
	require.NoError(t, err)
	assert.Equal(t, []string{"urgent"}, tags)

	err = db.SetTaskTags(id, []string{"alpha", "beta", "gamma"})
	require.NoError(t, err)
	tags, err = db.GetTaskTags(id)
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha", "beta", "gamma"}, tags)

	tags, err = db.GetTaskTags(99999)
	require.NoError(t, err)
	assert.Nil(t, tags)
}

func TestTaskArchiveUnarchive(t *testing.T) {
	db := setupTestDB(t)

	id, err := db.InsertTask("test", "desc", model.StatusPending, nil)
	require.NoError(t, err)

	archived, err := db.IsArchived(id)
	require.NoError(t, err)
	assert.False(t, archived)

	err = db.ArchiveTask(id)
	require.NoError(t, err)

	archived, err = db.IsArchived(id)
	require.NoError(t, err)
	assert.True(t, archived)

	err = db.UnarchiveTask(id)
	require.NoError(t, err)

	archived, err = db.IsArchived(id)
	require.NoError(t, err)
	assert.False(t, archived)

	err = db.ArchiveTask(99999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "task not found")
}

func TestListTasksFiltered(t *testing.T) {
	db := setupTestDB(t)

	id1, err := db.InsertTask("pending1", "desc", model.StatusPending, nil)
	require.NoError(t, err)
	err = db.AddTag(id1, "feature")
	require.NoError(t, err)

	id2, err := db.InsertTask("pending2", "desc", model.StatusPending, nil)
	require.NoError(t, err)
	err = db.AddTag(id2, "bug")
	require.NoError(t, err)

	id3, err := db.InsertTask("completed1", "desc", model.StatusCompleted, nil)
	require.NoError(t, err)
	err = db.AddTag(id3, "feature")
	require.NoError(t, err)

	id4, err := db.InsertTask("completed2", "desc", model.StatusCompleted, nil)
	require.NoError(t, err)
	err = db.ArchiveTask(id4)
	require.NoError(t, err)

	pendingStatus := model.StatusPending
	tasks, err := db.ListTasksFiltered(TaskFilter{Status: &pendingStatus})
	require.NoError(t, err)
	assert.Len(t, tasks, 2)
	for _, task := range tasks {
		assert.Equal(t, model.StatusPending, task.Status)
	}

	completedStatus := model.StatusCompleted
	tasks, err = db.ListTasksFiltered(TaskFilter{Status: &completedStatus})
	require.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, id3, tasks[0].ID)

	tasks, err = db.ListTasksFiltered(TaskFilter{IncludeArchived: false})
	require.NoError(t, err)
	assert.Len(t, tasks, 3)
	for _, task := range tasks {
		assert.False(t, task.Archived)
	}

	tasks, err = db.ListTasksFiltered(TaskFilter{IncludeArchived: true})
	require.NoError(t, err)
	assert.Len(t, tasks, 4)

	tasks, err = db.ListTasksFiltered(TaskFilter{IncludeTags: []string{"feature"}})
	require.NoError(t, err)
	assert.Len(t, tasks, 2)
	for _, task := range tasks {
		assert.Contains(t, task.Tags, "feature")
	}

	tasks, err = db.ListTasksFiltered(TaskFilter{ExcludeTags: []string{"feature"}})
	require.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, id2, tasks[0].ID)

	tasks, err = db.ListTasksFiltered(TaskFilter{
		Status:      &pendingStatus,
		IncludeTags: []string{"feature"},
	})
	require.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, id1, tasks[0].ID)
}

func TestBulkArchive(t *testing.T) {
	db := setupTestDB(t)

	id1, err := db.InsertTask("pending1", "desc", model.StatusPending, nil)
	require.NoError(t, err)
	id2, err := db.InsertTask("pending2", "desc", model.StatusPending, nil)
	require.NoError(t, err)
	id3, err := db.InsertTask("completed1", "desc", model.StatusCompleted, nil)
	require.NoError(t, err)

	completedStatus := model.StatusCompleted
	count, err := db.BulkArchive(TaskFilter{Status: &completedStatus})
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	archived, err := db.IsArchived(id1)
	require.NoError(t, err)
	assert.False(t, archived)

	archived, err = db.IsArchived(id2)
	require.NoError(t, err)
	assert.False(t, archived)

	archived, err = db.IsArchived(id3)
	require.NoError(t, err)
	assert.True(t, archived)
}

func TestBulkUnarchive(t *testing.T) {
	db := setupTestDB(t)

	id1, err := db.InsertTask("task1", "desc", model.StatusCompleted, nil)
	require.NoError(t, err)
	err = db.ArchiveTask(id1)
	require.NoError(t, err)
	id2, err := db.InsertTask("task2", "desc", model.StatusCompleted, nil)
	require.NoError(t, err)
	err = db.ArchiveTask(id2)
	require.NoError(t, err)
	id3, err := db.InsertTask("task3", "desc", model.StatusPending, nil)
	require.NoError(t, err)
	err = db.ArchiveTask(id3)
	require.NoError(t, err)

	completedStatus := model.StatusCompleted
	count, err := db.BulkUnarchive(TaskFilter{
		Status:          &completedStatus,
		IncludeArchived: true,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	archived, err := db.IsArchived(id1)
	require.NoError(t, err)
	assert.False(t, archived)

	archived, err = db.IsArchived(id2)
	require.NoError(t, err)
	assert.False(t, archived)

	archived, err = db.IsArchived(id3)
	require.NoError(t, err)
	assert.True(t, archived)
}

func TestBulkAddRemoveTag(t *testing.T) {
	db := setupTestDB(t)

	id1, err := db.InsertTask("task1", "desc", model.StatusPending, nil)
	require.NoError(t, err)
	id2, err := db.InsertTask("task2", "desc", model.StatusPending, nil)
	require.NoError(t, err)
	_, err = db.InsertTask("task3", "desc", model.StatusCompleted, nil)
	require.NoError(t, err)

	pendingStatus := model.StatusPending
	count, err := db.BulkAddTag(TaskFilter{Status: &pendingStatus}, "urgent")
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	tags, err := db.GetTaskTags(id1)
	require.NoError(t, err)
	assert.Contains(t, tags, "urgent")

	tags, err = db.GetTaskTags(id2)
	require.NoError(t, err)
	assert.Contains(t, tags, "urgent")

	count, err = db.BulkRemoveTag(TaskFilter{Status: &pendingStatus}, "urgent")
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	tags, err = db.GetTaskTags(id1)
	require.NoError(t, err)
	assert.NotContains(t, tags, "urgent")
}

func TestTagPersistence(t *testing.T) {
	tmpDir := t.TempDir()
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
	os.Setenv("XDG_DATA_HOME", tmpDir)
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	xdg.DataHome = tmpDir
	xdg.ConfigHome = tmpDir

	db1, err := Open("persist-test")
	require.NoError(t, err)

	id, err := db1.InsertTask("test", "desc", model.StatusPending, nil)
	require.NoError(t, err)
	err = db1.SetTaskTags(id, []string{"alpha", "beta"})
	require.NoError(t, err)
	err = db1.ArchiveTask(id)
	require.NoError(t, err)

	err = db1.Close()
	require.NoError(t, err)

	db2, err := Open("persist-test")
	require.NoError(t, err)
	defer db2.Close()

	tags, err := db2.GetTaskTags(id)
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha", "beta"}, tags)

	archived, err := db2.IsArchived(id)
	require.NoError(t, err)
	assert.True(t, archived)
}

func TestSnapshotWithTags(t *testing.T) {
	db := setupTestDB(t)

	tasks := []model.TaskWithDeps{
		{
			Task: model.Task{
				ID:          1,
				Name:        "task1",
				Description: "desc1",
				Status:      model.StatusPending,
				Tags:        []string{"feature", "urgent"},
				Archived:    false,
				CreatedAt:   1000,
				UpdatedAt:   1000,
			},
			Dependencies: nil,
		},
		{
			Task: model.Task{
				ID:          2,
				Name:        "task2",
				Description: "desc2",
				Status:      model.StatusCompleted,
				Tags:        []string{"bug"},
				Archived:    true,
				CreatedAt:   2000,
				UpdatedAt:   2000,
			},
			Dependencies: []int64{1},
		},
	}

	err := db.RestoreSnapshot(tasks)
	require.NoError(t, err)

	task1, err := db.GetTask(1)
	require.NoError(t, err)
	require.NotNil(t, task1)
	assert.Equal(t, "task1", task1.Name)
	assert.Equal(t, []string{"feature", "urgent"}, task1.Tags)
	assert.False(t, task1.Archived)

	task2, err := db.GetTask(2)
	require.NoError(t, err)
	require.NotNil(t, task2)
	assert.Equal(t, "task2", task2.Name)
	assert.Equal(t, []string{"bug"}, task2.Tags)
	assert.True(t, task2.Archived)
	assert.Equal(t, []int64{1}, task2.Dependencies)
}

func TestFreshSchemaHasTagsAndArchivedColumns(t *testing.T) {
	db := setupTestDB(t)

	id, err := db.InsertTask("test", "desc", model.StatusPending, nil)
	require.NoError(t, err)

	task, err := db.GetTask(id)
	require.NoError(t, err)
	require.NotNil(t, task)

	assert.NotNil(t, task.Tags)
	assert.False(t, task.Archived)
}

func TestTagOperationsOnNonExistentTask(t *testing.T) {
	db := setupTestDB(t)

	err := db.AddTag(99999, "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "task not found")

	err = db.RemoveTag(99999, "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "task not found")

	err = db.SetTaskTags(99999, []string{"test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "task not found")

	err = db.UnarchiveTask(99999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "task not found")
}

func TestListTasksFilteredWithDependencies(t *testing.T) {
	db := setupTestDB(t)

	id1, err := db.InsertTask("task1", "desc", model.StatusPending, nil)
	require.NoError(t, err)

	id2, err := db.InsertTask("task2", "desc", model.StatusPending, []int64{id1})
	require.NoError(t, err)

	tasks, err := db.ListTasksFiltered(TaskFilter{})
	require.NoError(t, err)
	assert.Len(t, tasks, 2)

	for _, task := range tasks {
		if task.ID == id2 {
			assert.Equal(t, []int64{id1}, task.Dependencies)
		}
	}
}

func TestTagsJSONHelpers(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected string
	}{
		{"nil", nil, "[]"},
		{"empty", []string{}, "[]"},
		{"single", []string{"alpha"}, `["alpha"]`},
		{"multiple", []string{"a", "b", "c"}, `["a","b","c"]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			json, err := TagsToJSON(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, json)

			tags, err := TagsFromJSON(json)
			require.NoError(t, err)
			if tt.input == nil {
				assert.Equal(t, []string{}, tags)
			} else {
				assert.Equal(t, tt.input, tags)
			}
		})
	}

	tags, err := TagsFromJSON("")
	require.NoError(t, err)
	assert.Equal(t, []string{}, tags)

	tags, err = TagsFromJSON("null")
	require.NoError(t, err)
	assert.Equal(t, []string{}, tags)
}

func TestDBPath(t *testing.T) {
	db := setupTestDB(t)

	path := db.Path()
	assert.Contains(t, path, "test-project")
	assert.Contains(t, path, "tasks.db")
}

func TestSnapshotsDir(t *testing.T) {
	db := setupTestDB(t)

	dir := db.SnapshotsDir()
	assert.Contains(t, dir, "test-project")
	assert.Contains(t, dir, "snapshots")

	err := db.CreateSnapshotsDir()
	require.NoError(t, err)

	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestCreateAndRenameSnapshotFile(t *testing.T) {
	db := setupTestDB(t)

	tmpFile, tmpPath, err := db.CreateSnapshotFile()
	require.NoError(t, err)
	require.NotNil(t, tmpFile)

	_, err = tmpFile.WriteString("test content")
	require.NoError(t, err)
	tmpFile.Close()

	finalPath := filepath.Join(db.SnapshotsDir(), "test.json")
	err = db.RenameSnapshot(tmpPath, finalPath)
	require.NoError(t, err)

	content, err := os.ReadFile(finalPath)
	require.NoError(t, err)
	assert.Equal(t, "test content", string(content))
}
