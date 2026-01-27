# Tinker CLI Usage Guide

Tinker is a task management tool designed for software development workflows. It stores tasks in SQLite with per-project isolation, supports task dependencies with cycle detection, and provides git-history reconciliation.

## Project Setup

Before using tinker, initialize it in your git repository:

```bash
tinker init                    # Initialize in current directory
tinker init --path /path/to/repo  # Initialize in specific directory
```

This creates project-specific storage under your XDG config and data directories.

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
| `tinker snapshot <name>` | Save task snapshot |
| `tinker restore <name>` | Restore from snapshot |
| `tinker sync` | Reconcile with git history |
| `tinker quickstart` | Print usage guide |
| `tinker completion` | Generate shell completions |
