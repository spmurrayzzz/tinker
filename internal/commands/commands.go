package commands

import (
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"tinker/internal/config"
	"tinker/internal/db"
	"tinker/internal/deps"
	"tinker/internal/git"
	"tinker/internal/model"
	"tinker/internal/project"
	"tinker/internal/snapshot"
	"tinker/internal/xdg"
)

func toUnix(t time.Time) int64 {
	return t.Unix()
}

func fromUnix(s int64) time.Time {
	return time.Unix(s, 0).UTC()
}

func FormatTime(ts int64) string {
	return time.Unix(ts, 0).UTC().Format(time.RFC3339)
}

type ProjectContext struct {
	DB         *db.DB
	ProjectKey string
	GitRoot    string
	IDWidth    int
}

func ResolveProject() (*ProjectContext, error) {
	gitRoot, err := git.GitRoot("")
	if err != nil {
		return nil, fmt.Errorf("not inside a git repository; run tinker init")
	}

	projectKey, err := project.DeriveKey(gitRoot)
	if err != nil {
		return nil, fmt.Errorf("derive project key: %w", err)
	}

	projConfig, err := config.ReadProjectConfig(projectKey)
	if err != nil {
		return nil, fmt.Errorf("project not initialized; run tinker init")
	}

	if projConfig.GitRoot != gitRoot {
		return nil, fmt.Errorf("project key collision; git root mismatch")
	}

	db, err := db.Open(projectKey)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	return &ProjectContext{
		DB:         db,
		ProjectKey: projectKey,
		GitRoot:    gitRoot,
		IDWidth:    db.IDWidth(),
	}, nil
}

func FormatTaskID(id int64, width int) string {
	return strconv.FormatInt(id, 10)
}

func ParseTaskID(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

func ParseDependsOn(s string) ([]int64, error) {
	if s == "" {
		return nil, nil
	}
	var ids []int64
	for _, part := range splitComma(s) {
		id, err := ParseTaskID(part)
		if err != nil {
			return nil, fmt.Errorf("invalid dependency id %q: %w", part, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func splitComma(s string) []string {
	var parts []string
	var current string
	for _, c := range s {
		if c == ',' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func ValidateDependsOn(db *db.DB, taskID int64, depsList []int64) error {
	for _, dep := range depsList {
		exists, err := db.TaskExists(dep)
		if err != nil {
			return fmt.Errorf("check task exists %d: %w", dep, err)
		}
		if !exists {
			return fmt.Errorf("dependency task not found: %d", dep)
		}
	}

	edges, err := db.GetAllEdges()
	if err != nil {
		return fmt.Errorf("get edges: %w", err)
	}

	if err := deps.ValidateNewDependencies(edges, depsList, taskID); err != nil {
		return err
	}

	return nil
}

func CheckDeleteProtection(db *db.DB, taskID int64) error {
	tasks, err := db.ListTasks()
	if err != nil {
		return fmt.Errorf("list tasks: %w", err)
	}
	for _, t := range tasks {
		for _, dep := range t.Dependencies {
			if dep == taskID {
				return fmt.Errorf("cannot delete task %d: other tasks depend on it", taskID)
			}
		}
	}
	return nil
}

func SyncTasks(ctx *ProjectContext) (int, []int64, error) {
	tasks, err := ctx.DB.GetCompletedTasksWithCommit()
	if err != nil {
		return 0, nil, fmt.Errorf("get completed tasks: %w", err)
	}

	checked := 0
	var resetIDs []int64

	for _, t := range tasks {
		if t.CommitHash == nil {
			continue
		}
		checked++
		isAncestor, err := git.IsAncestor(ctx.GitRoot, *t.CommitHash)
		if err != nil {
			return 0, nil, fmt.Errorf("check ancestor %s: %w", *t.CommitHash, err)
		}
		if !isAncestor {
			nilHash := ""
			if err := ctx.DB.UpdateTaskStatusAndCommit(t.ID, model.StatusPending, &nilHash); err != nil {
				return 0, nil, fmt.Errorf("reset task %d: %w", t.ID, err)
			}
			resetIDs = append(resetIDs, t.ID)
		}
	}

	return checked, resetIDs, nil
}

func SnapshotTasks(ctx *ProjectContext, name string) error {
	if err := snapshot.ValidateName(name); err != nil {
		return err
	}

	tasks, err := ctx.DB.ListTasks()
	if err != nil {
		return fmt.Errorf("list tasks: %w", err)
	}

	snapTasks := make([]snapshot.TaskSnapshot, len(tasks))
	for i, t := range tasks {
		snapTasks[i] = snapshot.TaskSnapshot{
			ID:           t.ID,
			Name:         t.Name,
			Description:  t.Description,
			Status:       string(t.Status),
			Dependencies: t.Dependencies,
			CommitHash:   t.CommitHash,
			CreatedAt:    fromUnix(t.CreatedAt),
			UpdatedAt:    fromUnix(t.UpdatedAt),
		}
	}

	path := filepath.Join(xdg.ProjectSnapshotsDir(ctx.ProjectKey), name+".json")
	if err := snapshot.Write(path, snapTasks); err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}

	return nil
}

func RestoreSnapshot(ctx *ProjectContext, name string) error {
	if err := snapshot.ValidateName(name); err != nil {
		return err
	}

	path := filepath.Join(xdg.ProjectSnapshotsDir(ctx.ProjectKey), name+".json")
	snap, err := snapshot.Read(path)
	if err != nil {
		return fmt.Errorf("read snapshot: %w", err)
	}

	tasks := make([]model.TaskWithDeps, len(snap.Tasks))
	for i, s := range snap.Tasks {
		tasks[i] = model.TaskWithDeps{
			Task: model.Task{
				ID:          s.ID,
				Name:        s.Name,
				Description: s.Description,
				Status:      model.TaskStatus(s.Status),
				CommitHash:  s.CommitHash,
				CreatedAt:   toUnix(s.CreatedAt),
				UpdatedAt:   toUnix(s.UpdatedAt),
			},
			Dependencies: s.Dependencies,
		}
	}

	if err := ctx.DB.RestoreSnapshot(tasks); err != nil {
		return fmt.Errorf("restore snapshot: %w", err)
	}

	return nil
}
