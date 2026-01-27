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

Tags:
  tinker add-tag <id> <tag>      Add tag to task
  tinker remove-tag <id> <tag>   Remove tag from task
  tinker set-tags <id> <t1> ...  Set all tags on task
  tinker list-tags <id>          List tags for task

Archiving:
  tinker archive <id>            Archive a task
  tinker unarchive <id>          Restore archived task

Filtering:
  tinker list-tasks --status pending           # by status
  tinker list-tasks --tags +feature,-wip       # by tag expression
  tinker list-tasks --include-archived         # include archived

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
