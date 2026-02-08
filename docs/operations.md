# ADM Operations Guide

## File Locations

All ADM state is stored under `.agents/adm/` at the project root (the nearest ancestor directory containing `.git`).

```
.agents/
  adm/
    adm.db                          # SQLite database (WAL mode)
    adm.db-wal                      # WAL file (auto-managed by SQLite)
    adm.db-shm                      # Shared memory file (auto-managed)
    state/
      <agent-name>.ack_token        # Hook delivery state per agent
```

### .gitignore

Add to your `.gitignore`:

```
.agents/
```

The `.agents/` directory is local state and should not be committed.

## Database

ADM uses SQLite in WAL mode. The database is created automatically on first use.

### Pragmas (set on every open)

| Pragma | Value | Purpose |
|--------|-------|---------|
| `journal_mode` | WAL | Concurrent reads during writes |
| `synchronous` | NORMAL | Durability with WAL performance |
| `busy_timeout` | 5000 | Wait up to 5s for locks instead of failing |
| `foreign_keys` | ON | Referential integrity |
| `temp_store` | MEMORY | Temp tables in memory |

### Tables

- **agents** - Registered agents with name, task, and liveness timestamps
- **messages** - Message content (direct and broadcast)
- **message_receipts** - Per-recipient delivery state (pending/offered/delivered)
- **sync_batches** - Batch tokens for acknowledgement tracking
- **claims** - File ownership claims (soft signals)

### Schema version

The current schema version is tracked via `PRAGMA user_version`. Current version: **2**.

## Agent Liveness

Agents are considered **online** if `last_seen_at` is within 5 minutes. Otherwise they show as **stale** in `adm status`.

The `last_seen_at` timestamp is updated by:
- `adm register` (on registration/re-registration)
- `adm sync` (heartbeat on every sync call)

## Message Delivery States

```
pending  -->  offered  -->  delivered
   |              |
   |              +--- (hook crash: stays offered, re-offered on next sync)
   |
   +--- (awaiting next sync call)
```

- **pending**: Message queued, not yet returned by sync
- **offered**: Returned in a sync batch, awaiting acknowledgement
- **delivered**: Acknowledged via ack_token in a subsequent sync call

### Duplicate delivery

If a hook crashes after receiving messages but before saving the `batch_token`, the same messages will be returned again on the next sync. This is by design (at-least-once delivery). Message IDs are stable, so recipients can detect duplicates.

## Troubleshooting

### Messages not being delivered

1. **Check agent is registered:**
   ```bash
   adm status
   ```
   The recipient agent must appear in the status list.

2. **Check pending messages:**
   ```bash
   adm inbox --agent <recipient>
   ```
   If messages show as `pending`, the agent's hook hasn't called `adm sync` yet.

3. **Check hook is running:**
   Verify that `ADM_AGENT` is set and the hook script is executable:
   ```bash
   echo $ADM_AGENT
   ls -la .claude/hooks/adm/
   ```

4. **Check for stuck offered messages:**
   If messages show as `offered` in inbox, the hook received them but crashed before saving the ack token. Delete the agent's token file to reset:
   ```bash
   rm .agents/adm/state/<agent>.ack_token
   ```
   Messages will be re-offered on the next sync.

### Database locked errors

SQLite has a 5-second busy timeout. Under normal load (dozens of agents), this is sufficient. If you see lock errors:

1. Check for runaway processes holding the database open
2. Verify no other tools are accessing `adm.db` directly
3. Check disk I/O (WAL mode needs adequate write throughput)

### Reset all state

To start fresh:

```bash
rm -rf .agents/adm/
```

The database and state files will be recreated on the next `adm` invocation. All message history, agent registrations, and claims will be lost.

### Reset delivery state only

To re-deliver all offered/pending messages for an agent:

```bash
rm .agents/adm/state/<agent>.ack_token
```

### Inspect the database

```bash
sqlite3 .agents/adm/adm.db

-- Check registered agents
SELECT name, task, last_seen_at FROM agents;

-- Check pending messages for an agent
SELECT m.sender_name, m.body, r.state
FROM message_receipts r
JOIN messages m ON m.id = r.message_id
WHERE r.recipient_name = 'my-agent'
ORDER BY r.created_at;

-- Check active claims
SELECT agent_name, path_pattern, path_norm FROM claims;
```

## Performance

ADM is designed for sub-10ms invocations. Measured latencies on Apple M1:

| Operation | p50 | p95 |
|-----------|-----|-----|
| sync (empty) | 1.0ms | 1.9ms |
| sync (1 message) | 1.2ms | 1.5ms |
| send (direct) | 1.1ms | 1.4ms |
| check-claim | 0.8ms | 1.9ms |

The database uses WAL mode which allows concurrent readers during writes. The sync transaction uses `BEGIN IMMEDIATE` to acquire a write lock upfront and avoid lock-promotion failures.

## Concurrency

ADM handles dozens of concurrent agents in the same repository. Each CLI invocation opens its own database connection, executes a short transaction, and exits. SQLite's WAL mode and busy timeout handle write serialization transparently.

Tested with 24 concurrent agents syncing simultaneously with 120 total messages. Zero message loss, zero lock timeout errors.
