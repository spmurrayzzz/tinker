package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"tinker/internal/commands"
	"tinker/internal/config"
	"tinker/internal/db"
	"tinker/internal/git"
	"tinker/internal/model"
	"tinker/internal/project"
	"tinker/internal/xdg"
)

var rootCmd = &cobra.Command{
	Use:   "tinker",
	Short: "Manage task-based workflows per git project",
}

var initCmd = &cobra.Command{
	Use:   "init [--path <directory>]",
	Short: "Initialize project storage for a git repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, _ := cmd.Flags().GetString("path")
		if path == "" {
			path = "."
		}

		gitRoot, err := git.GitRoot(path)
		if err != nil {
			return fmt.Errorf("not a git repository: %w", err)
		}

		canonical, err := git.CanonicalPath(gitRoot)
		if err != nil {
			return fmt.Errorf("canonicalize path: %w", err)
		}

		projectKey, err := project.DeriveKey(canonical)
		if err != nil {
			return fmt.Errorf("derive project key: %w", err)
		}

		dirs := []string{
			xdg.ProjectDataDir(projectKey),
			xdg.ProjectSnapshotsDir(projectKey),
			xdg.ProjectConfigDir(projectKey),
		}
		for _, dir := range dirs {
			if err := xdg.EnsureDir(dir); err != nil {
				return fmt.Errorf("ensure dir %s: %w", dir, err)
			}
		}

		globalPath := xdg.GlobalConfigPath()
		lockFile, err := xdg.LockFile(globalPath)
		if err != nil {
			return fmt.Errorf("acquire lock: %w", err)
		}
		defer lockFile.Close()

		if _, err := os.Stat(globalPath); os.IsNotExist(err) {
			if err := config.WriteGlobalConfig(&config.GlobalConfig{Version: 1, IDWidth: 5}); err != nil {
				return fmt.Errorf("write global config: %w", err)
			}
		}

		if err := config.WriteProjectConfig(&config.ProjectConfig{
			Version:    1,
			ProjectKey: projectKey,
			GitRoot:    canonical,
		}); err != nil {
			return fmt.Errorf("write project config: %w", err)
		}

		db, err := db.Open(projectKey)
		if err != nil {
			return fmt.Errorf("open db: %w", err)
		}
		defer db.Close()

		return nil
	},
}

var quickstartCmd = &cobra.Command{
	Use:   "quickstart",
	Short: "Print usage guide",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Print(`tinker - Task workflow management

Commands:
  tinker init [--path <dir>]     Initialize project storage
  tinker quickstart              Show this guide
  tinker snapshot <name>         Save snapshot of all tasks
  tinker restore <name>          Restore tasks from snapshot
  tinker add-task <name> --description "<desc>" [--depends-on <ids>]
  tinker list-tasks              List all tasks
  tinker view-task <id>          Show task details
  tinker update-task <id> --status <status> [--commit <hash>]
  tinker delete-task <id>        Delete a task
  tinker sync                    Reset completed tasks whose commit is not in HEAD

Status values: pending, in_progress, completed
Dependencies: comma-separated task IDs (e.g., --depends-on 1,2,3)
`)
		return nil
	},
}

var addTaskCmd = &cobra.Command{
	Use:   "add-task <name> --description \"<desc>\" [--depends-on <ids>]",
	Short: "Add a new task",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("missing task name")
		}
		name := args[0]
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("task name cannot be empty")
		}

		description, _ := cmd.Flags().GetString("description")
		if description == "" {
			return fmt.Errorf("description is required")
		}

		depsStr, _ := cmd.Flags().GetStringSlice("depends-on")
		var deps []int64
		for _, s := range depsStr {
			id, err := commands.ParseTaskID(s)
			if err != nil {
				return fmt.Errorf("invalid dependency %q: %w", s, err)
			}
			deps = append(deps, id)
		}

		ctx, err := commands.ResolveProject()
		if err != nil {
			return err
		}
		defer ctx.DB.Close()

		if len(deps) > 0 {
			if err := commands.ValidateDependsOn(ctx.DB, 0, deps); err != nil {
				return err
			}
		}

		id, err := ctx.DB.InsertTask(name, description, model.StatusPending, deps)
		if err != nil {
			return fmt.Errorf("insert task: %w", err)
		}

		fmt.Printf("%0*d\n", ctx.IDWidth, id)
		return nil
	},
}

var listTasksCmd = &cobra.Command{
	Use:   "list-tasks",
	Short: "List all tasks",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := commands.ResolveProject()
		if err != nil {
			return err
		}
		defer ctx.DB.Close()

		tasks, err := ctx.DB.ListTasks()
		if err != nil {
			return fmt.Errorf("list tasks: %w", err)
		}

		for _, t := range tasks {
			fmt.Printf("%0*d %s %s\n", ctx.IDWidth, t.ID, t.Status, t.Name)
		}
		return nil
	},
}

var viewTaskCmd = &cobra.Command{
	Use:   "view-task <id>",
	Short: "Show task details",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("missing task id")
		}

		id, err := commands.ParseTaskID(args[0])
		if err != nil {
			return fmt.Errorf("invalid task id: %w", err)
		}

		ctx, err := commands.ResolveProject()
		if err != nil {
			return err
		}
		defer ctx.DB.Close()

		t, err := ctx.DB.GetTask(id)
		if err != nil {
			return fmt.Errorf("get task: %w", err)
		}
		if t == nil {
			return fmt.Errorf("task not found: %d", id)
		}

		fmt.Printf("id: %0*d\n", ctx.IDWidth, t.ID)
		fmt.Printf("name: %s\n", t.Name)
		fmt.Printf("description: %s\n", t.Description)
		fmt.Printf("status: %s\n", t.Status)
		fmt.Printf("dependencies: ")
		for i, d := range t.Dependencies {
			if i > 0 {
				fmt.Print(",")
			}
			fmt.Printf("%0*d", ctx.IDWidth, d)
		}
		fmt.Println()
		fmt.Printf("commit_hash: ")
		if t.CommitHash != nil {
			fmt.Print(*t.CommitHash)
		}
		fmt.Println()
		fmt.Printf("created_at: %s\n", formatTime(t.CreatedAt))
		fmt.Printf("updated_at: %s\n", formatTime(t.UpdatedAt))
		return nil
	},
}

var updateTaskCmd = &cobra.Command{
	Use:   "update-task <id> --status <status> [--commit <hash>]",
	Short: "Update task status and/or commit",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("missing task id")
		}

		id, err := commands.ParseTaskID(args[0])
		if err != nil {
			return fmt.Errorf("invalid task id: %w", err)
		}

		statusStr, _ := cmd.Flags().GetString("status")
		if statusStr == "" {
			return fmt.Errorf("status is required")
		}

		status, err := model.ParseTaskStatus(statusStr)
		if err != nil {
			return fmt.Errorf("invalid status: %w", err)
		}

		var commitHash *string
		if cmd.Flags().Changed("commit") {
			hash, _ := cmd.Flags().GetString("commit")
			if err := validateCommitHash(hash); err != nil {
				return fmt.Errorf("invalid commit: %w", err)
			}
			commitHash = &hash
		}

		ctx, err := commands.ResolveProject()
		if err != nil {
			return err
		}
		defer ctx.DB.Close()

		if err := ctx.DB.UpdateTaskStatusAndCommit(id, status, commitHash); err != nil {
			return fmt.Errorf("update task: %w", err)
		}

		return nil
	},
}

var deleteTaskCmd = &cobra.Command{
	Use:   "delete-task <id>",
	Short: "Delete a task",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("missing task id")
		}

		id, err := commands.ParseTaskID(args[0])
		if err != nil {
			return fmt.Errorf("invalid task id: %w", err)
		}

		ctx, err := commands.ResolveProject()
		if err != nil {
			return err
		}
		defer ctx.DB.Close()

		if err := commands.CheckDeleteProtection(ctx.DB, id); err != nil {
			return err
		}

		if err := ctx.DB.DeleteTask(id); err != nil {
			return fmt.Errorf("delete task: %w", err)
		}

		return nil
	},
}

var snapshotCmd = &cobra.Command{
	Use:   "snapshot <name>",
	Short: "Save snapshot of all tasks",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("missing snapshot name")
		}
		name := args[0]

		ctx, err := commands.ResolveProject()
		if err != nil {
			return err
		}
		defer ctx.DB.Close()

		if err := commands.SnapshotTasks(ctx, name); err != nil {
			return fmt.Errorf("create snapshot: %w", err)
		}

		return nil
	},
}

var restoreCmd = &cobra.Command{
	Use:   "restore <name>",
	Short: "Restore tasks from snapshot",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("missing snapshot name")
		}
		name := args[0]

		ctx, err := commands.ResolveProject()
		if err != nil {
			return err
		}
		defer ctx.DB.Close()

		if err := commands.RestoreSnapshot(ctx, name); err != nil {
			return fmt.Errorf("restore snapshot: %w", err)
		}

		return nil
	},
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Reset completed tasks whose commit is not in HEAD",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := commands.ResolveProject()
		if err != nil {
			return err
		}
		defer ctx.DB.Close()

		checked, resetIDs, err := commands.SyncTasks(ctx)
		if err != nil {
			return fmt.Errorf("sync: %w", err)
		}

		fmt.Printf("checked: %d\n", checked)
		if len(resetIDs) == 0 {
			fmt.Println("reset: none")
		} else {
			fmt.Print("reset: ")
			for i, id := range resetIDs {
				if i > 0 {
					fmt.Print(",")
				}
				fmt.Printf("%0*d", ctx.IDWidth, id)
			}
			fmt.Println()
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(quickstartCmd)
	rootCmd.AddCommand(addTaskCmd)
	rootCmd.AddCommand(listTasksCmd)
	rootCmd.AddCommand(viewTaskCmd)
	rootCmd.AddCommand(updateTaskCmd)
	rootCmd.AddCommand(deleteTaskCmd)
	rootCmd.AddCommand(snapshotCmd)
	rootCmd.AddCommand(restoreCmd)
	rootCmd.AddCommand(syncCmd)

	initCmd.Flags().StringP("path", "p", "", "Directory to initialize (defaults to current directory)")

	addTaskCmd.Flags().StringP("description", "d", "", "Task description")
	addTaskCmd.Flags().StringSliceP("depends-on", "D", []string{}, "Task IDs this task depends on")

	updateTaskCmd.Flags().StringP("status", "s", "", "Task status (pending, in_progress, completed)")
	updateTaskCmd.Flags().StringP("commit", "c", "", "Commit hash")
}

func formatTime(ts int64) string {
	return commands.FormatTime(ts)
}

func validateCommitHash(hash string) error {
	if len(hash) < 7 || len(hash) > 40 {
		return fmt.Errorf("hash must be 7-40 hex characters")
	}
	for _, c := range hash {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return fmt.Errorf("hash contains invalid characters")
		}
	}
	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
