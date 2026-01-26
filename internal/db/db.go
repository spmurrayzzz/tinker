package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"tinker/internal/model"
	"tinker/internal/xdg"
)

const SchemaVersion = 1

type DB struct {
	*sql.DB
	projectKey string
	idWidth    int
}

func Open(projectKey string) (*DB, error) {
	dbPath := filepath.Join(xdg.ProjectDataDir(projectKey), "tasks.db")
	if err := xdg.EnsureDir(filepath.Dir(dbPath)); err != nil {
		return nil, fmt.Errorf("ensure db dir: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	d := &DB{DB: db, projectKey: projectKey}
	if err := d.initSchema(); err != nil {
		db.Close()
		return nil, err
	}

	var idWidth int
	err = db.QueryRow("SELECT id_width FROM global_config").Scan(&idWidth)
	if err != nil {
		if err == sql.ErrNoRows {
			idWidth = 5
		} else {
			db.Close()
			return nil, fmt.Errorf("get id width: %w", err)
		}
	}
	d.idWidth = idWidth

	return d, nil
}

func (d *DB) IDWidth() int {
	return d.idWidth
}

func (d *DB) ProjectKey() string {
	return d.projectKey
}

func (d *DB) initSchema() error {
	var version int
	err := d.QueryRow("PRAGMA user_version").Scan(&version)
	if err != nil {
		return fmt.Errorf("get user_version: %w", err)
	}

	if version == 0 {
		tx, err := d.Begin()
		if err != nil {
			return fmt.Errorf("begin tx: %w", err)
		}
		defer tx.Rollback()

		if err := d.createSchema(); err != nil {
			return err
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", SchemaVersion)); err != nil {
			return fmt.Errorf("set user_version: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit tx: %w", err)
		}
	} else if version != SchemaVersion {
		return fmt.Errorf("unsupported schema version: %d (expected %d)", version, SchemaVersion)
	}

	_, err = d.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		return fmt.Errorf("enable foreign keys: %w", err)
	}

	return nil
}

func (d *DB) createSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS global_config (
			id_width INTEGER NOT NULL DEFAULT 5
		)`,
		`INSERT INTO global_config (id_width) VALUES (5)`,
		`CREATE TABLE IF NOT EXISTS tasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT NOT NULL,
			status TEXT NOT NULL,
			commit_hash TEXT NULL,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS task_deps (
			task_id INTEGER NOT NULL,
			depends_on INTEGER NOT NULL,
			PRIMARY KEY (task_id, depends_on),
			FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
			FOREIGN KEY (depends_on) REFERENCES tasks(id) ON DELETE RESTRICT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_commit ON tasks(commit_hash)`,
		`CREATE INDEX IF NOT EXISTS idx_deps_depends_on ON task_deps(depends_on)`,
	}

	for _, q := range queries {
		if _, err := d.Exec(q); err != nil {
			return fmt.Errorf("exec schema query: %w", err)
		}
	}

	return nil
}

func (d *DB) InsertTask(name, description string, status model.TaskStatus, dependencies []int64) (int64, error) {
	tx, err := d.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().Unix()
	var id int64
	err = tx.QueryRow(`
		INSERT INTO tasks (name, description, status, commit_hash, created_at, updated_at)
		VALUES (?, ?, ?, NULL, ?, ?)
		RETURNING id
	`, name, description, status, now, now).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert task: %w", err)
	}

	for _, dep := range dependencies {
		if _, err := tx.Exec(`
			INSERT INTO task_deps (task_id, depends_on) VALUES (?, ?)
		`, id, dep); err != nil {
			return 0, fmt.Errorf("insert dep: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}

	return id, nil
}

func (d *DB) ListTasks() ([]model.Task, error) {
	rows, err := d.Query(`
		SELECT id, name, description, status, commit_hash, created_at, updated_at
		FROM tasks
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []model.Task
	for rows.Next() {
		var t model.Task
		var commitHash sql.NullString
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.Status, &commitHash, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		if commitHash.Valid {
			t.CommitHash = &commitHash.String
		}
		tasks = append(tasks, t)
	}

	for i := range tasks {
		deps, err := d.getDependencies(tasks[i].ID)
		if err != nil {
			return nil, err
		}
		tasks[i].Dependencies = deps
	}

	return tasks, nil
}

func (d *DB) GetTask(id int64) (*model.Task, error) {
	var t model.Task
	var commitHash sql.NullString
	err := d.QueryRow(`
		SELECT id, name, description, status, commit_hash, created_at, updated_at
		FROM tasks
		WHERE id = ?
	`, id).Scan(&t.ID, &t.Name, &t.Description, &t.Status, &commitHash, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}
	if commitHash.Valid {
		t.CommitHash = &commitHash.String
	}

	deps, err := d.getDependencies(id)
	if err != nil {
		return nil, err
	}
	t.Dependencies = deps

	return &t, nil
}

func (d *DB) getDependencies(taskID int64) ([]int64, error) {
	rows, err := d.Query(`
		SELECT depends_on FROM task_deps WHERE task_id = ? ORDER BY depends_on
	`, taskID)
	if err != nil {
		return nil, fmt.Errorf("get deps: %w", err)
	}
	defer rows.Close()

	var deps []int64
	for rows.Next() {
		var dep int64
		if err := rows.Scan(&dep); err != nil {
			return nil, fmt.Errorf("scan dep: %w", err)
		}
		deps = append(deps, dep)
	}
	return deps, nil
}

func (d *DB) UpdateTaskStatus(id int64, status model.TaskStatus) error {
	_, err := d.Exec(`
		UPDATE tasks SET status = ?, updated_at = ?
		WHERE id = ?
	`, status, time.Now().Unix(), id)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	return nil
}

func (d *DB) UpdateTaskCommit(id int64, commitHash string) error {
	_, err := d.Exec(`
		UPDATE tasks SET commit_hash = ?, updated_at = ?
		WHERE id = ?
	`, commitHash, time.Now().Unix(), id)
	if err != nil {
		return fmt.Errorf("update commit: %w", err)
	}
	return nil
}

func (d *DB) UpdateTaskStatusAndCommit(id int64, status model.TaskStatus, commitHash *string) error {
	now := time.Now().Unix()
	var err error
	if commitHash == nil {
		_, err = d.Exec(`
			UPDATE tasks SET status = ?, commit_hash = NULL, updated_at = ?
			WHERE id = ?
		`, status, now, id)
	} else {
		_, err = d.Exec(`
			UPDATE tasks SET status = ?, commit_hash = ?, updated_at = ?
			WHERE id = ?
		`, status, *commitHash, now, id)
	}
	if err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	return nil
}

func (d *DB) DeleteTask(id int64) error {
	res, err := d.Exec("DELETE FROM tasks WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("task not found: %d", id)
	}
	return nil
}

func (d *DB) TaskExists(id int64) (bool, error) {
	var count int
	err := d.QueryRow("SELECT COUNT(*) FROM tasks WHERE id = ?", id).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (d *DB) GetCompletedTasksWithCommit() ([]model.Task, error) {
	rows, err := d.Query(`
		SELECT id, name, description, status, commit_hash, created_at, updated_at
		FROM tasks
		WHERE status = 'completed' AND commit_hash IS NOT NULL
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("get completed tasks: %w", err)
	}
	defer rows.Close()

	var tasks []model.Task
	for rows.Next() {
		var t model.Task
		var commitHash sql.NullString
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.Status, &commitHash, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		if commitHash.Valid {
			t.CommitHash = &commitHash.String
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (d *DB) GetAllEdges() (map[int64][]int64, error) {
	rows, err := d.Query("SELECT DISTINCT task_id, depends_on FROM task_deps")
	if err != nil {
		return nil, fmt.Errorf("get edges: %w", err)
	}
	defer rows.Close()

	edges := make(map[int64][]int64)
	for rows.Next() {
		var taskID, dependsOn int64
		if err := rows.Scan(&taskID, &dependsOn); err != nil {
			return nil, fmt.Errorf("scan edge: %w", err)
		}
		edges[taskID] = append(edges[taskID], dependsOn)
	}
	return edges, nil
}

func (d *DB) RestoreSnapshot(tasks []model.TaskWithDeps) error {
	tx, err := d.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM task_deps"); err != nil {
		return fmt.Errorf("clear deps: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM tasks"); err != nil {
		return fmt.Errorf("clear tasks: %w", err)
	}

	maxID := int64(0)
	validIDs := make(map[int64]bool)
	for _, t := range tasks {
		if t.ID > maxID {
			maxID = t.ID
		}
		validIDs[t.ID] = true
		if _, err := tx.Exec(`
			INSERT INTO tasks (id, name, description, status, commit_hash, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, t.ID, t.Name, t.Description, t.Status, t.CommitHash, t.CreatedAt, t.UpdatedAt); err != nil {
			return fmt.Errorf("insert task %d: %w", t.ID, err)
		}
	}

	for _, t := range tasks {
		for _, dep := range t.Dependencies {
			if !validIDs[dep] {
				return fmt.Errorf("snapshot contains invalid dependency: task %d depends on non-existent task %d", t.ID, dep)
			}
			if _, err := tx.Exec(`
				INSERT INTO task_deps (task_id, depends_on) VALUES (?, ?)
			`, t.ID, dep); err != nil {
				return fmt.Errorf("insert dep for %d: %w", t.ID, err)
			}
		}
	}

	if maxID > 0 {
		if _, err := tx.Exec("INSERT OR REPLACE INTO sqlite_sequence (name, seq) VALUES ('tasks', ?)", maxID); err != nil {
			return fmt.Errorf("update sequence: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func (d *DB) Close() error {
	_, err := d.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	if err != nil {
		return fmt.Errorf("checkpoint: %w", err)
	}
	return d.DB.Close()
}

func (d *DB) Path() string {
	return filepath.Join(xdg.ProjectDataDir(d.projectKey), "tasks.db")
}

func (d *DB) SnapshotsDir() string {
	return xdg.ProjectSnapshotsDir(d.projectKey)
}

func (d *DB) CreateSnapshotsDir() error {
	return xdg.EnsureDir(d.SnapshotsDir())
}

func (d *DB) SnapshotPath(name string) string {
	return filepath.Join(d.SnapshotsDir(), name+".json")
}

func (d *DB) CreateSnapshotFile() (*os.File, string, error) {
	snapshotsDir := d.SnapshotsDir()
	if err := xdg.EnsureDir(snapshotsDir); err != nil {
		return nil, "", fmt.Errorf("ensure snapshots dir: %w", err)
	}
	tmp, err := os.CreateTemp(snapshotsDir, "*.tmp")
	if err != nil {
		return nil, "", fmt.Errorf("create temp: %w", err)
	}
	return tmp, tmp.Name(), nil
}

func (d *DB) RenameSnapshot(tmpPath, finalPath string) error {
	return os.Rename(tmpPath, finalPath)
}
