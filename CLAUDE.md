# ADM - Agent DM

Agent-to-agent communication for coding agents working on the same codebase.

## IMPORTANT - READ ON EVERY SESSION START OR CONTEXT REFRESH

Read these files immediately before doing any work:
- `gg/docs/spec.md` - Product specification (the source of truth)
- `gg/docs/plan.md` - Implementation plan with phases, progress tracking, and current status

These documents contain the full project context, architecture decisions, data model, and phased implementation plan. They are kept up to date as work progresses.

## Development Loop

After completing each phase:
1. Run `go build ./...` (compile check)
2. Run `go vet ./...` (static analysis)
3. Run `go test ./...` (tests)
4. If all pass: commit the implementation
5. Update `gg/docs/plan.md` - mark the completed phase with a checkmark and note what was done
6. Commit the plan update separately
7. Start the next phase immediately

Keep `gg/docs/plan.md` as the live progress tracker. Update the Project Map section below as new files are added.

## What is this?

ADM is a CLI tool that lets coding agents (Claude, Codex, etc.) send messages to each other, see who's online, and signal file ownership. It's designed to be called from agent hook systems (Claude Code hooks, Codex shell hooks) so messages are delivered passively into agent context windows without explicit polling.

## Project Map

```
cmd/adm/main.go              Entry point
internal/cli/                 Command definitions (one file per command)
  root.go                     Root command, subcommand wiring
  register.go                 adm register - announce agent presence
  status.go                   adm status - list online agents
  send.go                     adm send - direct message
  broadcast.go                adm broadcast - message all agents
  claim.go                    adm claim - signal file ownership
  unclaim.go                  adm unclaim - release file ownership
  checkclaim.go               adm check-claim - check file claims
  sync.go                     adm sync - hook delivery endpoint
  inbox.go                    adm inbox - read-only message view
  ui.go                       adm ui - start local web dashboard
  taskupdate.go               adm task-update - update task without re-register
  use.go                      adm use - set active agent identity
  whoami.go                   adm whoami - show current identity
  admin.go                    adm admin - maintenance commands (gated)
  cli_test.go                 CLI integration tests (40 tests)
  bench_test.go               Benchmarks and concurrent stress test
internal/identity/            Session-based agent identity
  identity.go                 Session management, identity resolution chain
internal/audit/               Mutation audit logging
  audit.go                    Best-effort append-only audit log
internal/server/              HTTP API + embedded frontend
  server.go                   Server struct, routing, JSON helpers
  handlers.go                 API handlers (health, messages, agents, claims, audit)
  embed.go                    Embedded frontend static file serving
  dist/                       Built React frontend assets (embedded)
internal/db/                  SQLite database layer
  db.go                       Open, close, pragmas, migrations
  schema.go                   Table definitions and migration SQL (v1-v3)
internal/pathnorm/            Path normalization utilities
  pathnorm.go                 Normalize, FindRepoRoot, Match
  pathnorm_test.go            Path normalization tests
hooks/claude/                 Claude Code hook scripts
  post-tool-sync.sh           PostToolUse: message delivery
  pre-tool-claim-check.sh     PreToolUse: file claim warnings
  settings.example.json       Example Claude Code hook settings
hooks/codex/                  Codex hook scripts
  shell-hook.sh               Shell hook for PROMPT_COMMAND integration
docs/                         User-facing documentation
  hooks.md                    Hook integration guide
  operations.md               State location, troubleshooting, performance
ui/                           Vite + React frontend (operator dashboard)
  src/App.tsx                 Main dashboard layout
  src/api.ts                  API client and TypeScript types
  src/hooks.ts                Polling hook, utility functions
  src/components/             React components
gg/docs/
  spec.md                     Product specification
  plan.md                     Implementation plan with phases
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

## Using ADM (for agents)

ADM hooks are wired up in this repo. Messages are delivered automatically via PostToolUse hooks - you don't need to poll. Just use the CLI to send/receive.

### Identity

Do not set identity by editing files directly. Identity should be managed through ADM commands.

Set your identity once when you start a session:

```bash
./adm use <your-name> --task "working on React components"
./adm whoami
```

Update your task as work changes:

```bash
./adm task-update --task "new focus area"
```

### Sending messages

```bash
# Direct message to another agent
./adm send --to <recipient> --msg "your message"

# Broadcast to all agents
./adm broadcast --msg "your message"
```

Both sender and recipient must be registered agents.

### Receiving messages

Messages are delivered **automatically** into your context after every tool call via the PostToolUse hook. You don't need to do anything - messages arrive passively.

If you need to check manually:

```bash
# Read-only view of your inbox (does not change delivery state)
./adm inbox
```

### Coordination

```bash
# See who's online and what they're working on
./adm status

# Claim files you're working on (warns other agents, doesn't block)
./adm claim "src/auth/*.go"

# Release a claim
./adm unclaim "src/auth/*.go"
```

### Registration

Register yourself when you start a session:

```bash
./adm register --name <your-name> --task "description of what you're doing"
```

Re-registering updates your task description and refreshes your liveness timestamp.

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
