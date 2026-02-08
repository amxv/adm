# ADM - Agent DM

Agent-to-agent communication for coding agents working on the same codebase.

## What is this?

ADM is a CLI tool that lets coding agents (Claude, Codex, etc.) send messages to each other, see who's online, and signal file ownership. It's designed to be called from agent hook systems (Claude Code hooks, Codex shell hooks) so messages are delivered passively into agent context windows without explicit polling.

## Project Map

```
cmd/adm/main.go            Entry point
internal/cli/               Command definitions (one file per command)
  root.go                   Root command, subcommand wiring
  register.go               adm register - announce agent presence
  status.go                 adm status - list online agents
internal/db/                SQLite database layer
  db.go                     Open, close, pragmas, project root detection
  schema.go                 Table definitions and migration SQL
gg/docs/
  spec.md                   Product specification
  plan.md                   Implementation plan with phases
```

## Architecture

- Single Go binary, no daemon, no background processes
- SQLite (WAL mode) at `.agents/adm/adm.db` relative to project root
- Project root = nearest ancestor directory containing `.git` (fallback: CWD)
- Every CLI invocation: open DB, run query, print output, exit
- Target: sub-10ms per invocation

## Key Concepts

- **Agents** register with a name and task description. Liveness tracked via `last_seen_at`.
- **Messages** are direct (agent-to-agent) or broadcast. Delivered passively via `adm sync` called from hooks.
- **File claims** are soft signals (warn, don't block) for coordination.
- **Delivery protocol** uses batch tokens for at-least-once delivery without agent-side polling.

## Build & Run

```
go build -o adm ./cmd/adm
./adm register --name myagent --task "working on auth"
./adm status
```

## Database

SQLite with these pragmas set on every open:
- `journal_mode=WAL` (concurrent reads)
- `synchronous=NORMAL` (durability with WAL)
- `busy_timeout=5000` (wait on locks instead of failing)
- `foreign_keys=ON`

Tables: `agents`, `messages`, `message_receipts`, `sync_batches`, `claims`

## Go Dependencies

- `github.com/spf13/cobra` - CLI framework
- `modernc.org/sqlite` - Pure Go SQLite driver (no CGO)
