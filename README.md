# Tinker

A Go-based CLI tool for managing task-based workflows with git integration. Designed for AI agent workflows but works for any task management use case.

## Quick Start

```bash
# Build
make build

# Initialize in a git repository
tinker init

# Add a task
tinker add-task "Implement feature X" --description "Description here"

# List tasks
tinker list-tasks
```

## Commands

| Command | Description |
|---------|-------------|
| `tinker init [--path <dir>]` | Initialize project storage |
| `tinker quickstart` | Print usage guide |
| `tinker add-task <name> [--description] [--depends-on <ids>]` | Add a new task |
| `tinker list-tasks` | List all tasks |
| `tinker view-task <id>` | Show task details |
| `tinker update-task <id> --status <status> [--commit <hash>]` | Update task |
| `tinker delete-task <id>` | Delete a task |
| `tinker snapshot <name>` | Save task state to JSON |
| `tinker restore <name>` | Restore from snapshot |
| `tinker sync` | Reconcile with git history |

Status values: `pending`, `in_progress`, `completed`.

## Features

- **SQLite persistence** - Local storage with WAL mode
- **Per-project isolation** - XDG base directories with derived project keys
- **Task dependencies** - Cycle detection prevents circular references
- **Git reconciliation** - `sync` resets completed tasks whose commits are no longer in history
- **Snapshots** - Atomic JSON backups for state preservation
- **Foreign key constraints** - Prevents deletion of dependency targets

## Architecture

```
cmd/tinker/      # CLI entry point
internal/
  xdg/           # XDG paths, file locking
  project/       # Project key derivation
  config/        # Config read/write
  model/         # Task struct
  db/            # SQLite operations
  deps/          # Cycle detection
  snapshot/      # JSON snapshots
  git/           # Git operations
  commands/      # Command handlers
```

## Storage

- **Data**: `~/.local/share/tinker/projects/<key>/tasks.db`
- **Snapshots**: `~/.local/share/tinker/projects/<key>/snapshots/<name>.json`
- **Global Config**: `~/.config/tinker/config.json`
- **Project Config**: `~/.config/tinker/projects/<key>/config.json`

Project key derived from git toplevel path using BLAKE3 hash.

## Build

```bash
make build      # Build binary
make test       # Run tests
make validate   # Format, vet, test
make clean      # Remove artifacts
```

## Requirements

- Go 1.24.12+
- C compiler (for CGO - go-sqlite3)

On macOS: `xcode-select --install`
