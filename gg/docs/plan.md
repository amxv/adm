# ADM Implementation Plan (V1)

## Purpose

This plan turns `gg/docs/spec.md` into an implementation sequence optimized for:

- Very low latency per CLI call
- Safe concurrent access with many agents
- Minimal runtime complexity (single binary, no daemon)
- Fast iteration for a Go newcomer

The plan is intentionally staged so we can prove performance early, before adding non-essential features.

## Current Status

- Last updated: 2026-02-08
- Current phase: Phase 5 (next)
- Completed phases:
  - Phase 0 completed in commit `0550acd` (CLI scaffold, DB bootstrap, schema v1, `register`/`status`)
  - Phase 1 completed in commit `eca61f0` (send/broadcast/claim/unclaim/check-claim commands)
  - Phase 2 completed in commit `1fae7f0` (`sync`, `inbox`, ack-token flow, delivery-state tests)
  - Phase 3 completed (claims uniqueness, sender validation, worktree repo root, 22 CLI integration tests)
  - Phase 4 completed in commit `9c5a767` (benchmarks, stress test, migration fast path, BEGIN IMMEDIATE)

## Scope

### In scope (V1)

- Single `adm` binary in Go
- SQLite-backed storage at `.agents/adm/adm.db`
- Hook-managed passive messaging via `adm sync`
- Invisible acknowledgement using `ack_token` / `batch_token`
- Direct messages and broadcast
- Soft file claims and claim warnings
- Agent register/status/liveness
- JSON output for hook-facing flows

### Out of scope (defer)

- Daemon/service mode
- Network transport
- Hard file locking
- Cursor-based replay APIs
- Rich observability stack

## Performance Targets

### Functional targets

- Handles dozens of concurrent agents in same repo
- Handles hundreds of sync calls/minute without lock storms

### Latency targets (local machine, warm cache)

- `adm sync` empty queue: p50 < 4 ms, p95 < 12 ms
- `adm sync` with 1-10 messages: p50 < 8 ms, p95 < 20 ms
- `adm send` direct: p50 < 5 ms
- `adm check-claim`: p50 < 3 ms

These are engineering targets, not hard product promises. We will benchmark and tune.

## Architecture Decisions

### Runtime shape

- One process per command invocation
- No background workers
- No long-lived locks

### Database location

- Database: `.agents/adm/adm.db`
- Hook local state directory: `.agents/adm/state/`
- Optional SQL migrations metadata: `.agents/adm/migrations/` (or internal table only)

### SQLite defaults (set on open)

- `PRAGMA journal_mode=WAL;`
- `PRAGMA synchronous=NORMAL;`
- `PRAGMA busy_timeout=5000;`
- `PRAGMA foreign_keys=ON;`
- `PRAGMA temp_store=MEMORY;` (optional)

### Go driver

Preferred: `modernc.org/sqlite` for simpler DX and distribution (pure Go, no CGO toolchain requirement).

Fallback option: `github.com/mattn/go-sqlite3` if benchmarks show `modernc` cannot meet latency targets on real workloads.

## Suggested Project Structure

```text
.
├── cmd/
│   └── adm/
│       └── main.go
├── internal/
│   ├── cli/              # arg parsing + command dispatch
│   ├── db/               # open/pragmas/migrations/transactions
│   ├── model/            # lightweight data structs
│   ├── service/          # register/send/broadcast/sync/claims/status
│   ├── format/           # JSON/text formatters
│   └── pathnorm/         # repo-root + path normalization helpers
├── gg/
│   └── docs/
│       ├── spec.md
│       └── plan.md
├── go.mod
└── go.sum
```

Keep layers thin. Avoid over-abstraction.

## Data Model (V1)

### `agents`

Tracks presence and liveness.

- `id` INTEGER PK
- `name` TEXT NOT NULL UNIQUE
- `task` TEXT NOT NULL DEFAULT ''
- `created_at` DATETIME NOT NULL
- `updated_at` DATETIME NOT NULL
- `last_seen_at` DATETIME NOT NULL

Indexes:

- `UNIQUE(name)`
- `INDEX(last_seen_at)`

### `messages`

Stores canonical message content.

- `id` TEXT PK (ULID/UUID)
- `sender_name` TEXT NOT NULL
- `body` TEXT NOT NULL
- `kind` TEXT NOT NULL CHECK (`direct`, `broadcast`)
- `created_at` DATETIME NOT NULL

Indexes:

- `INDEX(created_at)`
- `INDEX(sender_name, created_at)`

### `message_receipts`

One row per recipient per message. This keeps sync fast and simple.

- `id` INTEGER PK
- `message_id` TEXT NOT NULL REFERENCES `messages(id)`
- `recipient_name` TEXT NOT NULL
- `state` TEXT NOT NULL CHECK (`pending`, `offered`, `delivered`)
- `batch_token` TEXT NULL
- `offered_at` DATETIME NULL
- `delivered_at` DATETIME NULL
- `created_at` DATETIME NOT NULL

Indexes:

- `INDEX(recipient_name, state, created_at)`
- `INDEX(recipient_name, batch_token)`
- `INDEX(message_id)`

### `sync_batches`

Tracks offered batches for acknowledgement.

- `token` TEXT PK
- `agent_name` TEXT NOT NULL
- `created_at` DATETIME NOT NULL

Indexes:

- `INDEX(agent_name, created_at)`

### `claims`

Soft ownership signals.

- `id` INTEGER PK
- `agent_name` TEXT NOT NULL
- `path_pattern` TEXT NOT NULL
- `path_norm` TEXT NOT NULL
- `created_at` DATETIME NOT NULL
- `updated_at` DATETIME NOT NULL

Indexes:

- `INDEX(path_norm)`
- `INDEX(agent_name)`

## Command Contracts

### `adm register --name <name> --task <description>`

- Upsert by `name`
- Update `task`, `updated_at`, `last_seen_at`
- Idempotent

### `adm send --to <name> --msg <text>`

- Recipient must already exist in `agents`; if not, command fails fast with non-zero exit.
- Insert into `messages` (`kind=direct`)
- Insert one `message_receipts` row (`state=pending`)

### `adm broadcast --msg <text>`

- Insert one `messages` row (`kind=broadcast`)
- Materialize one `message_receipts` row for each other active/known agent
- Keep sender excluded unless `--include-self` exists (defer flag in v1)

### `adm sync --agent <name> [--ack-token <token>] --format json`

Hot path. Must be transactionally safe and fast.

Response shape:

```json
{
  "messages": [
    {
      "id": "msg_123",
      "from": "agent-b",
      "to": "agent-a",
      "body": "...",
      "created_at": "2026-02-08T18:10:00Z"
    }
  ],
  "batch_token": "bat_abc123"
}
```

Rules:

- If `ack_token` is provided, mark all rows in that batch for this agent as `delivered`.
- Select next `pending` receipts for this agent (ordered by `created_at ASC`, capped by internal constant, e.g. `10`).
- If selected rows > 0:
  - create new `batch_token`
  - mark selected rows as `offered` and attach token/time
  - return selected messages + token
- If selected rows == 0:
  - return empty `messages`
  - return `batch_token` as an empty string (`""`) for a stable response shape

### `adm check-claim --file <path> --agent <name>`

- Normalize input path relative to repo root
- Return warning data if file matches claims by other agents
- Non-blocking always

### `adm claim --agent <name> <path-pattern>`

- Normalize and upsert claim pattern/path
- Update timestamps on re-claim

### `adm unclaim --agent <name> <path-pattern>`

- Remove matching claim rows for agent

### `adm status`

- Show registered agents
- Show `online/stale` derived from `last_seen_at` against TTL
- Include `task`

### `adm inbox --agent <name>`

- Convenience read path for non-hook clients
- Read-only view of `pending` + `offered` messages for the agent
- Never mutates delivery state (only `sync` changes message state)

## Sync Transaction Algorithm (Pseudo)

```text
BEGIN IMMEDIATE;

-- 1) heartbeat
UPDATE agents SET last_seen_at = now, updated_at = now WHERE name = :agent;

-- 2) ack previous batch (if provided)
IF ack_token != '' THEN
  UPDATE message_receipts
  SET state='delivered', delivered_at=now
  WHERE recipient_name=:agent
    AND batch_token=:ack_token
    AND state='offered';
END IF;

-- 3) select next pending messages (limit N)
SELECT receipt ids + message payload
FROM message_receipts r
JOIN messages m ON m.id=r.message_id
WHERE r.recipient_name=:agent
  AND r.state='pending'
ORDER BY r.created_at ASC
LIMIT :N;

-- 4) if any, mark offered with new token
IF rows_found > 0 THEN
  INSERT INTO sync_batches(token, agent_name, created_at) VALUES (:token, :agent, now);
  UPDATE message_receipts
  SET state='offered', batch_token=:token, offered_at=now
  WHERE id IN (...selected ids...);
END IF;

COMMIT;
```

Notes:

- Keep this transaction short.
- Do not run expensive path operations inside this path.
- Use prepared statements for frequently used queries.

## Path Normalization Rules

To keep claims deterministic across agents:

- Resolve repo root once per invocation (look upward for `.git`; fallback CWD)
- Convert input file path to absolute
- Resolve symlinks when possible
- Convert to repo-relative path
- Normalize separators to `/`
- Remove redundant `./` and `../` segments where legal

Store normalized value in DB and compare on normalized form only.

Implementation note:

- Treat `.git` as either a directory or file (to support worktree/submodule layouts).

## Hook Integration Contract

### Shared behavior (all providers)

- Hook state file: `.agents/adm/state/<agent>.ack_token`
- On each tool boundary:
  - read token file (or empty)
  - run `adm sync ...`
  - if `messages` empty: no injection, no token update
  - if `messages` non-empty and injection succeeds: persist `batch_token`

### Claude PostToolUse

- Convert messages to concise, readable `additionalContext`
- Exit silently when no messages

### Codex shell hook

- Call `adm sync` opportunistically with cooldown guard (already scaffolded)
- Emit only when messages exist
- Write token only after printing/injecting message block succeeds

## Testing Strategy

### Unit tests

- Path normalization edge cases
- Message state transitions
- Batch ack behavior
- Claim matching
- JSON formatter stability

### Integration tests

- Register/send/sync end-to-end
- Crash simulation between offered and token-save (duplicate-safe behavior)
- Broadcast materialization correctness
- Concurrent sync from many agents

### Concurrency tests

- Spawn N goroutines/processes calling `sync` repeatedly
- Verify no lost messages and acceptable lock wait times

### Regression tests

- Empty sync is silent for hook-facing usage
- No context noise when no messages

## Benchmark Plan

Implement Go benchmarks and a simple CLI stress script.

### Benchmarks

- `BenchmarkSyncEmpty`
- `BenchmarkSyncWithMessages1`
- `BenchmarkSyncWithMessages10`
- `BenchmarkSendDirect`
- `BenchmarkCheckClaim`

### Stress test script

- Simulate 24-48 agents
- Loop sync at realistic cadence
- Random sends/broadcasts
- Capture p50/p95/p99 and lock timeout count

### Success criteria

- Meets target latencies in local stress runs
- No growing lock timeout trend under steady load

## Implementation Phases

### Phase 0: Scaffold and DB foundation

Status: completed

Deliverables:

- Go module initialized
- CLI entrypoint + subcommand routing
- DB open/init with PRAGMAs
- Schema migration v1

Exit criteria:

- `adm` boots and creates `.agents/adm/adm.db`

### Phase 1: Core lifecycle commands

Status: completed

Deliverables:

- `register`, `status`
- `send`, `broadcast` (materialized receipts)
- `claim`, `unclaim`, `check-claim`

Exit criteria:

- Commands work on happy path with tests
- Core negative-path tests exist (`send` to unknown agent fails, invalid claim/check inputs fail cleanly)

Notes: Added `--from` flag to send/broadcast for sender identity. Extracted pathnorm package for shared path normalization. check-claim outputs JSON for hook consumption. Commit: `eca61f0`.

### Phase 2: Sync and delivery semantics

Status: completed

Deliverables:

- `sync` transaction with ack-token and batch-token
- JSON response contract
- Heartbeat update in sync
- `inbox` fallback command

Exit criteria:

- End-to-end hook simulation passes
- Duplicate-safe failure scenario verified

Notes: Sync uses BEGIN IMMEDIATE for write safety. Full lifecycle tested: empty sync returns `{"messages":[],"batch_token":""}`, messages transition pending->offered->delivered correctly, ack-token advancement works. Inbox is read-only. Commit: `1fae7f0`.

### Phase 3: Correctness and Stabilization

Status: completed

Deliverables:

- Eliminate duplicate claim rows with explicit uniqueness and deterministic upsert behavior
- Add sender validation for `send`/`broadcast` (sender must be a registered agent)
- Harden repo-root detection for worktree/submodule layouts (`.git` may be a file)
- Expand CLI tests to cover Phase 1 and Phase 2 happy + negative paths

Exit criteria:

- Duplicate claim bug is fixed and covered by tests
- `send`/`broadcast` fail cleanly for unknown sender/recipient combinations
- Phase 1+2 commands have stable regression coverage

Notes: Added schema v2 migration (PRAGMA user_version tracking) with UNIQUE index on claims(agent_name, path_norm) and dedup of existing rows. Claim uses proper ON CONFLICT upsert. Send/broadcast validate sender exists. FindRepoRoot accepts .git as file or directory. 22 CLI integration tests added covering register, status, send, broadcast, claim, unclaim, check-claim, sync lifecycle, inbox, and worktree detection. Removed pre-existing bench tests (will reimplement in Phase 4).

### Phase 4: Performance hardening

Status: completed

Deliverables:

- Benchmarks + stress harness
- Query/index tuning based on measured bottlenecks
- Hot-path allocation reduction

Exit criteria:

- Meets latency targets and concurrency stability

Notes: Migration fast path skips DDL when schema is current. Sync uses proper BEGIN IMMEDIATE via *sql.Conn. Go benchmarks (5 benchmarks) and concurrent stress test (24 agents, 120 messages) added. Latency report test validates p50/p95. All targets met: sync empty p50=1.0ms, sync w/msg p50=1.2ms, send p50=1.1ms, check-claim p50=0.8ms. Commit: `9c5a767`.

### Phase 5: Hook adapters and docs

Deliverables:

- Claude hook example script
- Codex shell hook integration notes
- Operational docs (state location, troubleshooting)

Exit criteria:

- Both provider paths tested in real workflow

### Phase 6: Private Release Packaging

Deliverables:

- Add cross-platform build targets for `darwin/linux` and `amd64/arm64`
- Produce versioned release artifacts (`adm_<version>_<os>_<arch>.tar.gz`) with checksums
- Add installer script (`scripts/install.sh`) that:
  - detects OS/arch
  - downloads the appropriate artifact from a private release URL
  - verifies checksum
  - installs `adm` into a target bin directory (`/usr/local/bin` or `$HOME/.local/bin`)
- Define install UX:
  - one-liner: `curl -fsSL <private-release-url>/install.sh | bash`
  - optional version pinning via env var (for example `ADM_VERSION=v0.1.0`)

Exit criteria:

- Fresh machine install works via one-liner
- Installed binary version matches requested/default version
- Checksums are validated successfully

### Phase 7: Runtime Validation Gate (Self-Verified)

Deliverables:

- Add a repeatable runtime smoke script at `scripts/smoke.sh` to validate real behavior, not only unit tests.
- Smoke script must run in an isolated temp workspace with a `.git` marker and include:
  - fresh install path using release artifact (or installer flow when available)
  - `register` / `send` / `sync` end-to-end delivery
  - `claim` / `check-claim` / `unclaim` flow
  - `inbox` read-only verification
  - `status` liveness output check
- Add a release-validation command sequence in docs (copy/paste runnable).
- Require phase runner (Claude) to execute smoke script and report key outputs in phase notes.

Exit criteria:

- Runtime smoke passes end-to-end in a clean temp directory.
- Validation is performed by the implementing agent itself (not assumed from tests).
- Phase notes include commands run and observed outputs.

### Phase 8: README and Operator Docs

Deliverables:

- Add/update `README.md` with:
  - what ADM is and who it is for
  - install instructions (private channel + `curl | bash` flow)
  - quickstart (`register`, `send`, `broadcast`, `sync`, `status`)
  - hook integration overview for Claude and Codex
  - project layout and data location (`.agents/adm/`)
  - troubleshooting and common errors
- Include a short release/update section (upgrade and rollback guidance)

Exit criteria:

- New user can install and run first command in under 5 minutes following only `README.md`
- README commands are validated against current CLI behavior

### Phase 9: Web UI MVP (Vite + React)

Deliverables:

- Create lightweight web UI using `Vite + React`
- Add a minimal HTTP API layer in `adm` for UI reads (local-only by default)
- Build an operator dashboard with:
  - global message feed (direct + broadcast)
  - agent status panel (online/stale + task + last seen)
  - claims panel
  - message detail view
- Add search and filtering:
  - free-text search in message body
  - filters by `from`, `to`, `kind`, `state`, and time range
- Add pagination/virtualized list behavior for large message volumes

Exit criteria:

- Operator can open UI and see all messages with usable latency
- Search/filter operations work against real project data
- UI runs locally without additional infrastructure

### Phase 10: Web UI Enhancements and Usability

Deliverables:

- Saved filters/presets for common workflows
- Conflict radar view (claim overlap and coordination hotspots)
- Delivery debug panel (`pending`/`offered`/`delivered` counts and recent batch tokens)
- Basic UI polish for desktop and mobile layouts

Exit criteria:

- UI is usable for day-to-day multi-agent monitoring
- Key debugging tasks can be completed without SQL/manual log inspection

## Phase Completion Loop

After each phase is completed:

1. Run `go build ./...`
2. Run `go vet ./...`
3. Run `go test ./...`
4. Commit implementation changes
5. Update this file (`gg/docs/plan.md`) with:
   - phase status
   - what shipped
   - any scope adjustments
6. Commit the plan update separately
7. Immediately begin the next phase

If context is compacted/refreshed, the next agent session must start by reading:

1. `gg/docs/spec.md`
2. `gg/docs/plan.md`
3. `CLAUDE.md`

## Operational Notes

- Use UTC timestamps everywhere.
- Keep output deterministic for parser stability.
- Keep hook payloads short (default max messages per batch = 10).
- Avoid logging to stdout in hook-facing commands; reserve stdout for machine-readable output.

## Risks and Mitigations

### SQLite write contention

Mitigation:

- WAL + busy timeout
- Short transactions
- Avoid extra writes in hot path

### Message duplication due to hook crashes

Mitigation:

- Intended behavior under token protocol
- Include message IDs so duplicates are easy to recognize

### Path mismatch across tools/OS contexts

Mitigation:

- Strict normalization strategy
- Test with relative/absolute/symlinked paths

### Scope creep

Mitigation:

- Keep v1 focused on commands in spec
- Defer cursors/daemons/advanced replay APIs

## Implementation Checklist

- [x] Initialize Go module and CLI skeleton
- [x] Implement DB bootstrap + migration v1
- [x] Implement register/status
- [x] Implement send/broadcast
- [x] Implement claim/unclaim/check-claim
- [x] Implement sync with ack-token flow
- [x] Implement inbox fallback
- [x] Stabilize Phase 1/2 correctness fixes (claims uniqueness, sender validation, worktree-safe repo root)
- [x] Add unit/integration/concurrency tests (22 CLI integration tests)
- [x] Add benchmarks and stress script
- [x] Validate against performance targets
- [ ] Document hook usage for Claude and Codex
- [ ] Implement private release packaging and installer (`curl | bash`)
- [ ] Add and run runtime smoke validation gate (`scripts/smoke.sh`) with captured outputs
- [ ] Add/update `README.md` for install + quickstart + integrations
- [ ] Build Web UI MVP using Vite + React (messages, search, filters)
- [ ] Add Web UI enhancements (saved filters, conflict radar, delivery debug)

## Immediate Next Step

Start Phase 5: Hook adapters and docs.

1. Create Claude Code hook example script (PostToolUse + PreToolUse)
2. Create Codex shell hook integration notes
3. Add operational docs (state location, troubleshooting)
