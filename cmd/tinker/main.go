package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
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

const builtInQuickstartPrompt = `## Task Management with tinker

tinker is a dependency-aware task manager designed for git-integrated workflows.
Each git repository gets isolated storage, and completed tasks can be linked to
commits for automatic reconciliation when history changes.

### Why tinker?

- Dependency-aware: Track blockers between tasks with cycle detection
- Git-integrated: Link completed tasks to commits, sync resets orphaned work
- Per-project: Isolated SQLite storage per git repository
- Snapshots: Save and restore task state atomically
- Tags: Flexible filtering with include/exclude expressions

### Quick Start

Initialize in any git repository:
  tinker init

Create tasks with dependencies:
  tinker add-task "Build login form" -d "Create React component"
  tinker add-task "Add validation" -d "Email and password rules" --depends-on 1

List and view tasks:
  tinker list-tasks
  tinker list-tasks --status pending
  tinker view-task 1

Update status and link to commits:
  tinker update-task 1 --status in_progress
  tinker update-task 1 --status completed --commit abc123f

### Task Status Values

- pending      - Work not yet started
- in_progress  - Currently being worked on
- completed    - Finished (optionally linked to a commit)

### Working with Dependencies

Tasks can depend on other tasks. A task with unfinished dependencies cannot
be meaningfully completed until its blockers are resolved.

  tinker add-task "Deploy" -d "Ship to prod" --depends-on 1,2,3

Dependencies prevent deletion of tasks that other tasks depend on.
Cycle detection prevents circular dependency chains.

### Tags and Filtering

Add tags to organize tasks:
  tinker add-tag 1 feature
  tinker add-tag 1 priority-high
  tinker set-tags 1 backend api v2

Filter with tag expressions (+ to include, - to exclude):
  tinker list-tasks --tags +feature           # must have 'feature'
  tinker list-tasks --tags +backend,-wip      # backend but not wip
  tinker list-tasks --tags +api,+v2           # must have both

### Archiving Completed Work

Archive tasks to hide them from default listings:
  tinker archive 1
  tinker archive --all                        # archive all completed
  tinker archive --all --tags +feature        # only completed features
  tinker list-tasks --include-archived        # show archived tasks
  tinker unarchive 1                          # restore if needed

### Git Sync

When you rebase, reset, or otherwise change git history, completed tasks
linked to orphaned commits can be automatically reset to pending:
  tinker sync

This checks each completed task's commit hash against current HEAD ancestry.

### Snapshots

Save and restore complete task state:
  tinker snapshot before-refactor
  tinker restore before-refactor

Snapshots are atomic JSON files stored per-project.

### Workflow for AI Agents

1. Check pending work:       tinker list-tasks --status pending
2. Claim a task:             tinker update-task <id> --status in_progress
3. Work on it:               Implement, test, document
4. Discover blockers?        Create dependent task with --depends-on
5. Commit your changes:      git add && git commit
6. Complete with commit:     tinker update-task <id> --status completed -c <hash>
7. Archive when done:        tinker archive <id>

### Storage Locations

- Data:     ~/.local/share/tinker/projects/<key>/tasks.db
- Snapshots: ~/.local/share/tinker/projects/<key>/snapshots/
- Config:   ~/.config/tinker/

Each git repository is identified by a unique project key derived from its path.

### Shell Completion

  tinker completion bash > /etc/bash_completion.d/tinker
  tinker completion zsh > "${fpath[1]}/_tinker"
  tinker completion fish > ~/.config/fish/completions/tinker.fish
`

var quickstartCmd = &cobra.Command{
	Use:   "quickstart",
	Short: "Print usage guide",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Print(builtInQuickstartPrompt)
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

		statusStr, _ := cmd.Flags().GetString("status")
		tagsExpr, _ := cmd.Flags().GetString("tags")
		includeArchived, _ := cmd.Flags().GetBool("include-archived")

		filter := db.TaskFilter{
			IncludeArchived: includeArchived,
		}

		if statusStr != "" {
			status, err := model.ParseTaskStatus(statusStr)
			if err != nil {
				return fmt.Errorf("invalid status: %w", err)
			}
			filter.Status = &status
		}

		if tagsExpr != "" {
			include, exclude, err := commands.ParseTagExpression(tagsExpr)
			if err != nil {
				return fmt.Errorf("invalid tag expression: %w", err)
			}
			filter.IncludeTags = include
			filter.ExcludeTags = exclude
		}

		tasks, err := ctx.DB.ListTasksFiltered(filter)
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
		fmt.Printf("archived: %t\n", t.Archived)
		fmt.Printf("tags: ")
		for i, tag := range t.Tags {
			if i > 0 {
				fmt.Print(",")
			}
			fmt.Print(tag)
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

var addTagCmd = &cobra.Command{
	Use:   "add-tag <task-id> <tag>",
	Short: "Add tag to task",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 2 {
			return fmt.Errorf("usage: add-tag <task-id> <tag>")
		}
		id, err := commands.ParseTaskID(args[0])
		if err != nil {
			return fmt.Errorf("invalid task id: %w", err)
		}
		tag := args[1]
		if tag == "" {
			return fmt.Errorf("tag cannot be empty")
		}

		ctx, err := commands.ResolveProject()
		if err != nil {
			return err
		}
		defer ctx.DB.Close()

		if err := ctx.DB.AddTag(id, tag); err != nil {
			return fmt.Errorf("add tag: %w", err)
		}
		return nil
	},
}

var removeTagCmd = &cobra.Command{
	Use:   "remove-tag <task-id> <tag>",
	Short: "Remove tag from task",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 2 {
			return fmt.Errorf("usage: remove-tag <task-id> <tag>")
		}
		id, err := commands.ParseTaskID(args[0])
		if err != nil {
			return fmt.Errorf("invalid task id: %w", err)
		}
		tag := args[1]

		ctx, err := commands.ResolveProject()
		if err != nil {
			return err
		}
		defer ctx.DB.Close()

		if err := ctx.DB.RemoveTag(id, tag); err != nil {
			return fmt.Errorf("remove tag: %w", err)
		}
		return nil
	},
}

var listTagsCmd = &cobra.Command{
	Use:   "list-tags <task-id>",
	Short: "List all tags for task",
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

		tags, err := ctx.DB.GetTaskTags(id)
		if err != nil {
			return fmt.Errorf("get tags: %w", err)
		}
		if tags == nil {
			return fmt.Errorf("task not found: %d", id)
		}
		for _, tag := range tags {
			fmt.Println(tag)
		}
		return nil
	},
}

var setTagsCmd = &cobra.Command{
	Use:   "set-tags <task-id> <tag1> [tag2]...",
	Short: "Replace all tags on task",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("missing task id")
		}
		id, err := commands.ParseTaskID(args[0])
		if err != nil {
			return fmt.Errorf("invalid task id: %w", err)
		}
		tags := args[1:]

		ctx, err := commands.ResolveProject()
		if err != nil {
			return err
		}
		defer ctx.DB.Close()

		if err := ctx.DB.SetTaskTags(id, tags); err != nil {
			return fmt.Errorf("set tags: %w", err)
		}
		return nil
	},
}

var archiveCmd = &cobra.Command{
	Use:   "archive [task-id]",
	Short: "Archive a task",
	RunE: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")
		tagsExpr, _ := cmd.Flags().GetString("tags")

		ctx, err := commands.ResolveProject()
		if err != nil {
			return err
		}
		defer ctx.DB.Close()

		if all {
			if len(args) > 0 {
				return fmt.Errorf("--all cannot be used with a task-id argument")
			}
			status := model.StatusCompleted
			filter := db.TaskFilter{Status: &status}
			if tagsExpr != "" {
				include, exclude, err := commands.ParseTagExpression(tagsExpr)
				if err != nil {
					return fmt.Errorf("invalid tag expression: %w", err)
				}
				filter.IncludeTags = include
				filter.ExcludeTags = exclude
			}
			count, err := ctx.DB.BulkArchive(filter)
			if err != nil {
				return fmt.Errorf("archive: %w", err)
			}
			fmt.Printf("Archived %d completed task(s)\n", count)
			return nil
		}

		if tagsExpr != "" {
			return fmt.Errorf("--tags requires --all")
		}

		if len(args) < 1 {
			return fmt.Errorf("missing task id (use --all to archive all completed tasks)")
		}
		id, err := commands.ParseTaskID(args[0])
		if err != nil {
			return fmt.Errorf("invalid task id: %w", err)
		}

		if err := ctx.DB.ArchiveTask(id); err != nil {
			return fmt.Errorf("archive: %w", err)
		}
		return nil
	},
}

var unarchiveCmd = &cobra.Command{
	Use:   "unarchive <task-id>",
	Short: "Restore archived task",
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

		if err := ctx.DB.UnarchiveTask(id); err != nil {
			return fmt.Errorf("unarchive: %w", err)
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

const bashCompletionScript = `# tinker bash completion
__tinker_init_completion()
{
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    words=("${COMP_WORDS[@]}")
    cword="$COMP_CWORD"
}

__tinker_get_completions()
{
    local requestComp out lines line args cur_word
    cur_word="${words[${#words[@]}-1]}"
    args=("${words[@]:1}")
    requestComp="${words[0]} __complete ${args[*]}"
    if [[ -z "$cur_word" ]]; then
        requestComp="${requestComp} ''"
    fi
    out=$(eval "$requestComp" 2>/dev/null)
    lines=()
    while IFS= read line; do
        lines+=("$line")
    done <<< "$out"
    for ((i=0; i<${#lines[@]}-1; i++)); do
        echo "${lines[$i]%%	*}"
    done
}

__tinker_handle_completion()
{
    local out
    out=$(__tinker_get_completions)
    while IFS= read -r comp; do
        COMPREPLY+=("$comp")
    done <<< "$out"
}

__start_tinker()
{
    __tinker_init_completion
    __tinker_handle_completion
}

complete -F __start_tinker tinker
`

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate completion script",
	Long: `To load completions:

Bash:

  $ source <(tinker completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ tinker completion bash > /etc/bash_completion.d/tinker
  # macOS:
  $ tinker completion bash > /usr/local/etc/bash_completion.d/tinker

Zsh:

  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:

  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ tinker completion zsh > "${fpath[1]}/_tinker"

  # You will need to start a new shell for this to take effect.

Fish:

  $ tinker completion fish | source

  # To load completions for each session, execute once:
  $ tinker completion fish > ~/.config/fish/completions/tinker.fish
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.ExactValidArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			fmt.Print(bashCompletionScript)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return nil
	},
}

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset project - delete all tasks and reinitialize",
	RunE: func(cmd *cobra.Command, args []string) error {
		force, err := cmd.Flags().GetBool("force")
		if err != nil {
			return fmt.Errorf("reset: get force flag: %w", err)
		}
		keepSnapshots, err := cmd.Flags().GetBool("keep-snapshots")
		if err != nil {
			return fmt.Errorf("reset: get keep-snapshots flag: %w", err)
		}

		ctx, err := commands.ResolveProject()
		if err != nil {
			return fmt.Errorf("reset: resolve project: %w", err)
		}
		ctx.DB.Close()

		if !force {
			fmt.Print("This will delete all tasks and cannot be undone. Continue? [y/N]: ")
			reader := bufio.NewReader(os.Stdin)
			response, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("reset: read confirmation: %w", err)
			}
			response = strings.TrimSpace(strings.ToLower(response))
			if response != "y" && response != "yes" {
				fmt.Println("Aborted.")
				return nil
			}
		}

		dbPath := filepath.Join(xdg.ProjectDataDir(ctx.ProjectKey), "tasks.db")
		for _, ext := range []string{"", "-wal", "-shm"} {
			path := dbPath + ext
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("reset: remove database%s: %w", ext, err)
			}
		}

		if !keepSnapshots {
			snapshotsDir := xdg.ProjectSnapshotsDir(ctx.ProjectKey)
			entries, err := os.ReadDir(snapshotsDir)
			if err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("reset: read snapshots directory: %w", err)
			}
			for _, entry := range entries {
				path := filepath.Join(snapshotsDir, entry.Name())
				if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("reset: remove snapshot %s: %w", entry.Name(), err)
				}
			}
			os.Remove(snapshotsDir)
		}

		newDB, err := db.Open(ctx.ProjectKey)
		if err != nil {
			return fmt.Errorf("reset: reinitialize database: %w", err)
		}
		if err := newDB.Close(); err != nil {
			return fmt.Errorf("reset: close reinitialized database: %w", err)
		}

		fmt.Println("Project reset successfully.")
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
	rootCmd.AddCommand(addTagCmd)
	rootCmd.AddCommand(removeTagCmd)
	rootCmd.AddCommand(listTagsCmd)
	rootCmd.AddCommand(setTagsCmd)
	rootCmd.AddCommand(archiveCmd)
	rootCmd.AddCommand(unarchiveCmd)
	rootCmd.AddCommand(snapshotCmd)
	rootCmd.AddCommand(restoreCmd)
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(resetCmd)
	rootCmd.AddCommand(completionCmd)

	initCmd.Flags().StringP("path", "p", "", "Directory to initialize (defaults to current directory)")

	addTaskCmd.Flags().StringP("description", "d", "", "Task description")
	addTaskCmd.Flags().StringSliceP("depends-on", "D", []string{}, "Task IDs this task depends on")

	listTasksCmd.Flags().String("status", "", "Filter by status (pending, in_progress, completed)")
	listTasksCmd.Flags().String("tags", "", "Tag expression: +tag (must have), -tag (must not have)")
	listTasksCmd.Flags().BoolP("include-archived", "a", false, "Include archived tasks")

	updateTaskCmd.Flags().StringP("status", "s", "", "Task status (pending, in_progress, completed)")
	updateTaskCmd.Flags().StringP("commit", "c", "", "Commit hash")

	archiveCmd.Flags().Bool("all", false, "Archive all completed tasks")
	archiveCmd.Flags().String("tags", "", "Tag expression for --all: +tag (must have), -tag (must not have)")

	resetCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	resetCmd.Flags().Bool("keep-snapshots", false, "Preserve snapshot files")
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
