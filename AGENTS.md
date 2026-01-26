# AGENTS.md - Coding Agent Guide for Tinker

## Overview

Tinker is a Go-based CLI tool for managing task-based workflows with git integration. It provides local SQLite persistence, per-project isolation using XDG base directories, task snapshots, and git-history reconciliation (`tinker sync`). The tool is designed for AI agent workflows but works for any task management use case.

## Project Information

**Type**: Single Go binary CLI application
**Language**: Go 1.24.12
**Size**: Small (~15 Go source files)
**Module**: `github.com/spmurray/tinker`
**License**: Not specified

**Runtime Dependencies**:
- `github.com/spf13/cobra v1.10.2` - CLI framework
- `github.com/mattn/go-sqlite3 v1.14.33` - SQLite driver
- `golang.org/x/crypto/blake3` - Hashing for project keys (included in stdlib)

**Testing Dependencies**:
- `github.com/stretchr/testify v1.11.1` - Assertion library

## Build and Validate

Always run these commands in the repository root (`/Users/spmurray/src/tries/2026-01-23-tinker`).

### Build
```bash
make build
# Or: go build -o tinker ./cmd/tinker
```
Produces the `tinker` binary in the repo root. No preconditions required. Clean build always succeeds.

### Run Tests
```bash
make test
# Or: go test ./internal/... -v
```
Runs all unit tests in `internal/` packages. Tests use standard library `testing` package. No database or external dependencies required for unit tests.

### Linting and Formatting
```bash
make format   # go fmt ./...
make vet      # go vet ./...
make validate # Runs format, vet, and test in sequence
```

### Dependency Management
```bash
make deps
# Runs: go mod download && go mod verify
```

### Clean
```bash
make clean
# Removes: rm -f tinker
```

## Project Layout

```
/Users/spmurray/src/tries/2026-01-23-tinker/
├── cmd/tinker/
│   └── main.go              # CLI entry point, Cobra commands
├── internal/
│   ├── xdg/
│   │   └── xdg.go           # XDG base directory paths, file locking
│   ├── project/
│   │   └── project.go       # Project key derivation from git root
│   ├── config/
│   │   └── config.go        # Global and project config read/write
│   ├── model/
│   │   └── task.go          # Task struct, TaskStatus enum
│   ├── db/
│   │   └── db.go            # SQLite operations, schema management
│   ├── deps/
│   │   └── cycle.go         # Cycle detection for task dependencies
│   ├── snapshot/
│   │   └── snapshot.go      # Snapshot save/restore to JSON
│   ├── git/
│   │   └── git.go           # Git toplevel, ancestor check
│   └── commands/
│       └── commands.go      # Command handlers, project resolution
├── Makefile                 # Build targets
├── go.mod                   # Module definition
├── go.sum                   # Dependency checksums
├── BUGS.md                  # Bug report and fix documentation
├── PLAN.md                  # Implementation plan (detailed)
├── PROPOSAL.md              # Feature proposal
├── README.md                # User-facing documentation
└── .gitignore               # Ignores: /tinker, tinker.exe
```

## Architecture

### Entry Point
`cmd/tinker/main.go:18-433` - Contains all Cobra command definitions and the main CLI dispatcher.

### Storage Paths (XDG)
- **Data**: `~/.local/share/tinker/projects/<project_key>/tasks.db`
- **Snapshots**: `~/.local/share/tinker/projects/<project_key>/snapshots/<name>.json`
- **Global Config**: `~/.config/tinker/config.json`
- **Project Config**: `~/.config/tinker/projects/<project_key>/config.json`

Project key is derived from git toplevel path: the absolute canonical path with `/` replaced by `-`, trimmed of leading/trailing dashes (e.g., `/Users/spmurray/src/tries/2026-01-23-tinker` becomes `--Users-spmurray-src-tries-2026-01-23-tinker`).

### SQLite Schema
`internal/db/db.go:104-138` - Defines tables:
- `global_config` - Stores ID width for zero-padding
- `tasks` - Task records with auto-increment IDs
- `task_deps` - Join table for task dependencies with FK constraints

### Command Handlers
`internal/commands/commands.go` - Contains `ResolveProject()`, `ParseTaskID()`, `ValidateDependsOn()`, `CheckDeleteProtection()`, `SyncTasks()`, `SnapshotTasks()`, `RestoreSnapshot()`, and `FormatTime()`.

## Validation Pipeline

The `make validate` target runs the complete check sequence: `format` → `vet` → `test`. All three must pass for code to be considered valid.

**No GitHub Actions or CI/CD**: This repository does not have automated CI. Validation must be run manually or by the coding agent.

## Key Facts for Quick Navigation

- **Database operations**: `internal/db/db.go`
- **CLI commands**: `cmd/tinker/main.go`
- **Task dependencies/cycle detection**: `internal/deps/cycle.go`
- **XDG paths and file locking**: `internal/xdg/xdg.go`
- **Git operations**: `internal/git/git.go`
- **Project resolution**: `internal/commands/commands.go:ResolveProject()`
- **Configuration**: `internal/config/config.go`
- **Bug history**: `BUGS.md` contains detailed bug reports and fixes
- **Implementation details**: `PLAN.md` contains comprehensive technical specifications

## Important Implementation Details

1. **File locking**: Global config writes use advisory locking via `internal/xdg/xdg.go:69-84`. Always acquire lock before writing global config.

2. **DB close behavior**: `DB.Close()` in `internal/db/db.go:417-423` runs `PRAGMA wal_checkpoint(TRUNCATE)` before closing to ensure clean shutdown.

3. **Task ID parsing**: `commands.ParseTaskID()` in `internal/commands/commands.go` accepts zero-padded IDs (e.g., "00001") and converts to int64.

4. **Dependency validation**: `ValidateDependsOn()` checks for self-dependency, duplicate dependencies, and cycle detection before allowing insert.

5. **Snapshot atomicity**: Snapshots use temp files + rename for atomic writes (see `internal/db/db.go:441-455`).

6. **Test isolation**: Tests use `t.TempDir()` and must restore environment variables (XDG paths) after completion (see `BUGS.md:173-231`).

## Environment Setup

No special environment setup required beyond standard Go toolchain. The project uses only:
- Go 1.24.12 (from `go.mod`)
- SQLite3 (via `go-sqlite3` CGO bindings, requires C compiler)

CGO is enabled by default for `go-sqlite3`. On macOS, this requires Xcode command line tools.

## Common Workflows

### Adding a New Command
1. Add command struct in `cmd/tinker/main.go`
2. Register in `init()` function
3. Implement handler in `internal/commands/commands.go` if shared logic needed
4. Add tests in appropriate `*_test.go` file
5. Run `make validate`

### Modifying Database Schema
1. Update `internal/db/db.go:createSchema()` with new DDL
2. Increment `SchemaVersion` constant (currently 1)
3. Add migration logic in `initSchema()` if upgrading from previous versions
4. Update `internal/db/db_test.go` if it exists
5. Run `make test`

### Adding Dependencies
1. `go get <package>@<version>`
2. Run `make deps` to verify
3. Commit changes to `go.mod` and `go.sum`

## Troubleshooting

**Build fails with CGO errors**: Install Xcode command line tools: `xcode-select --install`

**Test failures with database locked**: Ensure `DB.Close()` is called with `defer` in all code paths.

**Global config race conditions**: Use `xdg.LockFile()` before writing global config (already implemented in `initCmd`).

## Documentation

- **User-facing docs**: `README.md`
- **Technical spec**: `PLAN.md`
- **Bug history**: `BUGS.md`
- **Feature proposal**: `PROPOSAL.md`

## Trust This Guide

The information in this file is comprehensive and accurate. Code agents should trust these instructions and only perform additional search if:
- A specific command or pattern documented here fails
- The documentation is clearly incomplete for the task at hand
- Actual behavior differs from documented behavior

When in doubt, run `make validate` to verify changes before submitting.
