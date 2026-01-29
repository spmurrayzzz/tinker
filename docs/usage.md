# Tinker CLI Usage Guide

Tinker is a task management tool designed for software development workflows. It stores tasks in SQLite with per-project isolation, supports task dependencies with cycle detection, and provides git-history reconciliation.

## Project Setup

Before using tinker, initialize it in your git repository:

```bash
tinker init                    # Initialize in current directory
tinker init --path /path/to/repo  # Initialize in specific directory
```

This creates project-specific storage under your XDG config and data directories.

## Quickstart Guidance

`tinker quickstart` prints the built-in workflow prompt. If your repo contains
`.tinker/quickstart.md`, that file is appended under a
"Local workflow instructions" header.

To replace the built-in prompt, set `quickstart_mode` to `replace` in either
`~/.config/tinker/config.json` or
`~/.config/tinker/projects/<key>/config.json`.

Valid values:

- `append` (default)
- `replace`

Project config overrides global config. If `replace` is set but
`.tinker/quickstart.md` is missing, `tinker quickstart` falls back to the
built-in prompt.

## Task Management

### Adding Tasks

Create new tasks with a name and description:

```bash
tinker add-task "Implement feature X" --description "Add user authentication"
tinker add-task "API endpoint" -d "Create REST API" -D 1,2
```

Flags:
- `-d, --description`: Task description (required)
- `-D, --depends-on`: Task IDs this task depends on (optional)

### Listing Tasks

View all tasks in the project:

```bash
tinker list-tasks
```

Output shows task ID, status, and name:

```
00001 pending Task Name 1
00002 in_progress Task Name 2
00003 completed Task Name 3
```

### Viewing Task Details

Get detailed information about a specific task:

```bash
tinker view-task 1
tinker view-task 00001  # Zero-padded format works too
```

### Updating Task Status

Change task status and optionally associate a commit:

```bash
tinker update-task 1 --status in_progress
tinker update-task 1 --status completed --commit abc1234
```

Status values: `pending`, `in_progress`, `completed`

### Deleting Tasks

Remove a task from the project:

```bash
tinker delete-task 1
```

Cannot delete tasks that other tasks depend on.

## Task Dependencies

Tasks can depend on other tasks, creating a dependency graph:

```bash
tinker add-task "Implement feature X" -d "Add user authentication" -D 1,2
```

Tinker validates dependencies:
- All referenced tasks must exist
- No self-dependencies
- No duplicate dependencies
- No circular dependencies

## Tags

Organize tasks with tags for filtering and categorization.

### Adding Tags

Add a single tag to a task:

```bash
tinker add-tag 1 feature
tinker add-tag 1 priority-high
```

### Removing Tags

Remove a tag from a task:

```bash
tinker remove-tag 1 feature
```

### Setting All Tags

Replace all tags on a task:

```bash
tinker set-tags 1 backend api v2
```

### Listing Tags

View all tags for a task:

```bash
tinker list-tags 1
```

### Filtering by Tags

Use tag expressions when listing tasks:

```bash
# Must have specific tag
tinker list-tasks --tags +feature

# Must not have tag
tinker list-tasks --tags -wip

# Must have one tag but not another
tinker list-tasks --tags +backend,-wip

# Must have multiple tags
tinker list-tasks --tags +api,+v2
```

## Archiving

Archive completed tasks to hide them from default listings.

### Archiving Tasks

Archive a single task:

```bash
tinker archive 1
```

Archive all completed tasks:

```bash
tinker archive --all
```

Archive completed tasks with specific tags:

```bash
tinker archive --all --tags +feature
```

### Unarchiving Tasks

Restore an archived task:

```bash
tinker unarchive 1
```

### Viewing Archived Tasks

Include archived tasks in listings:

```bash
tinker list-tasks --include-archived
# or
tinker list-tasks -a
```

## Snapshots

Save and restore complete task state for backup or branching workflows.

### Creating Snapshots

```bash
tinker snapshot before-refactor
tinker snapshot backup-v1
```

### Restoring Snapshots

```bash
tinker restore before-refactor
```

Restoring deletes all existing tasks and restores from the snapshot. Task IDs are preserved.

## Git Integration

Tinker integrates with git for commit tracking and history reconciliation.

### Commit Association

Link tasks to commits when completing them:

```bash
tinker update-task 1 --status completed --commit $(git rev-parse HEAD)
```

### Sync Command

Reconcile task state with git history:

```bash
tinker sync
```

This checks completed tasks with associated commits and resets any whose commits are no longer in the current branch back to `pending`. Useful after rebasing or amending commits.

## Common Workflows

### Starting a New Feature

```bash
tinker init
tinker add-task "Design API" -d "Create OpenAPI spec"
tinker add-task "Implement API" -d "Write handler code" -D 1
tinker list-tasks
```

### Completing Work

```bash
tinker update-task 1 --status in_progress
# ... do work ...
tinker update-task 1 --status completed --commit $(git rev-parse HEAD)
```

### Organizing with Tags

```bash
tinker add-task "Build login form" -d "Create React component"
tinker add-tag 1 frontend
tinker add-tag 1 auth

# Later, filter by tags
tinker list-tasks --tags +frontend
```

### Archiving Completed Work

```bash
# Archive all completed tasks
tinker archive --all

# Archive only completed tasks tagged as 'feature'
tinker archive --all --tags +feature

# View archived tasks
tinker list-tasks --include-archived
```

### Creating a Backup Before Risky Changes

```bash
tinker snapshot before-refactor
```

### Restoring from Backup

```bash
tinker restore before-refactor
```

### Syncing After Rebase

```bash
tinker sync
```

## Resetting Project

Delete all tasks and reinitialize with a fresh database:

```bash
# Reset with confirmation prompt
tinker reset

# Reset without confirmation
tinker reset --force

# Keep snapshots after reset
tinker reset --force --keep-snapshots
```

**Warning**: This permanently deletes all tasks and cannot be undone. Snapshots are also deleted unless `--keep-snapshots` is specified. Global configuration (like `id_width`) is preserved.

## Shell Completions

Generate shell completion scripts:

```bash
# Bash
source <(tinker completion bash)

# Zsh
tinker completion zsh > "${fpath[1]}/_tinker"

# Fish
tinker completion fish | source
```

## Quick Reference

| Command | Description |
|---------|-------------|
| `tinker init` | Initialize project storage |
| `tinker add-task <name>` | Add a new task |
| `tinker list-tasks` | List all tasks |
| `tinker view-task <id>` | Show task details |
| `tinker update-task <id>` | Update task status |
| `tinker delete-task <id>` | Delete a task |
| `tinker add-tag <task-id> <tag>` | Add tag to task |
| `tinker remove-tag <task-id> <tag>` | Remove tag from task |
| `tinker list-tags <task-id>` | List tags for task |
| `tinker set-tags <task-id> <tag1>...` | Replace all tags |
| `tinker archive [task-id]` | Archive task(s) |
| `tinker unarchive <task-id>` | Restore archived task |
| `tinker snapshot <name>` | Save task snapshot |
| `tinker restore <name>` | Restore from snapshot |
| `tinker sync` | Reconcile with git history |
| `tinker reset` | Reset project (delete all tasks) |
| `tinker quickstart` | Print usage guide |
| `tinker completion` | Generate shell completions |
