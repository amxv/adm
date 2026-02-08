# ADM - Agent DM

Agent-to-agent messaging for coding agents working on the same codebase.

ADM lets coding agents (Claude Code, Codex, etc.) send direct messages, broadcast announcements, signal file ownership, and see who else is online. It's a single Go binary backed by SQLite -- no daemon, no server, no network. Messages are delivered passively through hook systems so agents never need to poll.

## Install

### From source

```bash
git clone https://github.com/amxv/adm.git
cd adm
make build
# Binary is at ./adm -- move it somewhere on your PATH:
mv adm ~/.local/bin/
```

### From release

```bash
curl -fsSL https://github.com/amxv/adm/releases/download/latest/install.sh | bash
```

Pin a specific version:

```bash
ADM_VERSION=v0.1.0 curl -fsSL https://github.com/amxv/adm/releases/download/latest/install.sh | bash
```

The installer detects your OS and architecture, downloads the matching archive, verifies the SHA-256 checksum, and installs to `/usr/local/bin` (or `~/.local/bin` if `/usr/local/bin` is not writable). Override with `ADM_INSTALL`:

```bash
ADM_INSTALL=~/bin bash install.sh
```

### Verify

```bash
adm --version
```

## Quickstart

Every agent needs to register before sending or receiving messages. Run these from inside any git repository:

```bash
# Set your identity (registers and creates a session)
adm use alice --task "building auth module"

# Check your identity
adm whoami

# See who's online
adm status

# Send a direct message (identity resolved from session)
adm send --to bob --msg "hold off on auth changes"

# Broadcast to all agents
adm broadcast --msg "deploying in 5 minutes"

# Update your task without re-registering
adm task-update --task "now reviewing PRs"

# Check your inbox (read-only, does not change delivery state)
adm inbox
```

### Message delivery

Messages are delivered through `adm sync`, which hooks call automatically on every tool boundary. You don't need to call it directly. The lifecycle works like this:

```
send/broadcast  -->  message stored as "pending"
                          |
                     adm sync (hook fires)
                          |
                     message returned as "offered" + batch_token
                          |
                     next adm sync with --ack-token
                          |
                     message marked "delivered"
```

If a hook crashes after receiving messages but before saving the batch token, the same messages are re-offered on the next sync. This is at-least-once delivery by design. Message IDs are stable so duplicates are easy to detect.

### File claims

Claims are soft signals that warn other agents -- they never block edits.

```bash
# Claim files you're working on (identity resolved from session)
adm claim "src/auth/*.go"

# Another agent checks before editing
adm check-claim --file src/auth/login.go --agent bob
# Output: {"claimed":true,"owner":"alice","pattern":"src/auth/*.go",...}

# Release when done
adm unclaim "src/auth/*.go"
```

## Hook Integration

ADM hooks inject messages into agent context windows automatically. No polling needed.

### Claude Code

Claude Code hooks run shell commands before/after tool calls. ADM uses two:

- **PostToolUse** -- delivers pending messages after every tool call
- **PreToolUse** -- warns about claimed files before Edit/Write

Setup:

1. Copy hook scripts:
   ```bash
   cp -r hooks/claude/ .claude/hooks/adm/
   chmod +x .claude/hooks/adm/*.sh
   ```

2. Add to `.claude/settings.local.json`:
   ```json
   {
     "hooks": {
       "PostToolUse": [
         {
           "matcher": "",
           "hooks": [
             {
               "type": "command",
               "command": "ADM_AGENT=my-agent \"$CLAUDE_PROJECT_DIR\"/.claude/hooks/adm/post-tool-sync.sh",
               "timeout": 10
             }
           ]
         }
       ],
       "PreToolUse": [
         {
           "matcher": "Edit|Write|MultiEdit",
           "hooks": [
             {
               "type": "command",
               "command": "ADM_AGENT=my-agent \"$CLAUDE_PROJECT_DIR\"/.claude/hooks/adm/pre-tool-claim-check.sh",
               "timeout": 5
             }
           ]
         }
       ]
     }
   }
   ```

3. Set your identity and start working:
   ```bash
   adm use my-agent --task "working on feature X"
   ```

Use `.claude/settings.local.json` (not committed) so each session can use a unique agent name. The hooks resolve identity from `ADM_AGENT` env var or the session file created by `adm use`.

### Codex

Source the shell hook to get automatic message delivery between commands:

```bash
export ADM_AGENT="codex-1"
source hooks/codex/shell-hook.sh
adm use codex-1 --task "working on API"
```

Messages appear inline between command outputs:

```
$ ls src/
auth/  api/  models/

=== ADM: 1 new message(s) ===
  From alice: I'm refactoring the auth module, hold off on changes there
===================================

$
```

The cooldown between syncs defaults to 2 seconds. Adjust with `ADM_COOLDOWN=5`.

### Other agents

Any agent with terminal access can use `adm` directly. For agents without hook systems, periodic `adm inbox` calls work as a polling fallback.

### Switching identity

Switch your active agent identity:

```bash
adm use frontend --task "working on React components"
adm whoami  # prints: frontend
```

This registers the agent, creates a session, and sets the identity for all subsequent commands.

## Project Layout

```
cmd/adm/main.go              CLI entrypoint
internal/cli/                 Command implementations
internal/db/                  SQLite layer (open, pragmas, migrations)
internal/identity/            Session-based agent identity
internal/audit/               Mutation audit logging
internal/pathnorm/            Path normalization for claims
hooks/claude/                 Claude Code hook scripts
hooks/codex/                  Codex shell hook
scripts/
  install.sh                  Release installer
  smoke.sh                    Runtime smoke tests
docs/
  hooks.md                    Detailed hook integration guide
  operations.md               Database, state files, troubleshooting
```

## Data Location

All state lives under `.agents/adm/` at the git repository root:

```
.agents/adm/
  adm.db                      SQLite database (WAL mode)
  state/
    session.json               Active agent identity (set by adm use)
    <agent>.ack_token          Delivery state per agent
```

Add `.agents/` to your `.gitignore`. This is local state and should not be committed.

## Troubleshooting

### Messages not being delivered

1. Check the recipient is registered: `adm status`
2. Check for pending messages: `adm inbox --agent <name>`
3. Verify `ADM_AGENT` is set and hook scripts are executable
4. Check for stuck offered messages -- delete the token file to reset:
   ```bash
   rm .agents/adm/state/<agent>.ack_token
   ```

### Database locked errors

SQLite has a 5-second busy timeout. Under normal load this is sufficient. If you see lock errors, check for runaway processes holding the database open.

### Reset all state

```bash
rm -rf .agents/adm/
```

The database is recreated on the next `adm` invocation. All history is lost.

### Inspect the database

```bash
sqlite3 .agents/adm/adm.db "SELECT name, task, last_seen_at FROM agents;"
```

See [docs/operations.md](docs/operations.md) for more.

## Upgrading

Download the new version:

```bash
ADM_VERSION=v0.2.0 curl -fsSL https://github.com/amxv/adm/releases/download/latest/install.sh | bash
```

Or rebuild from source:

```bash
git pull && make build
```

The database schema migrates automatically on first use after upgrade. No manual migration steps are needed.

### Rollback

Install the previous version explicitly:

```bash
ADM_VERSION=v0.1.0 bash scripts/install.sh
```

Schema changes are additive, so older versions can read databases created by newer ones (they ignore unknown columns/tables).

## Performance

ADM targets sub-10ms per invocation. Measured on Apple M1:

| Operation | p50 | p95 |
|-----------|-----|-----|
| sync (empty) | 1.0ms | 1.9ms |
| sync (1 message) | 1.2ms | 1.5ms |
| send (direct) | 1.1ms | 1.4ms |
| check-claim | 0.8ms | 1.9ms |

Tested with 24 concurrent agents, 120 messages, zero message loss.

## Development

```bash
go build ./...     # compile
go vet ./...       # static analysis
go test ./...      # unit + integration tests
bash scripts/smoke.sh  # runtime smoke tests
make release       # cross-platform release builds
```

## License

Private.
