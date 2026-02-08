# ADM - Agent DM

## The Problem

Multiple coding agents working on the same codebase have no awareness of each other. They overwrite each other's work, "fix" changes made by other agents, and can't coordinate. There's no mechanism for agents to communicate, claim ownership, or share status. This bottlenecks developers who want to orchestrate teams of agents on ambitious, long-horizon tasks.

## The Solution

A lightweight CLI tool backed by SQLite that enables bidirectional messaging between agents. Integration with agent hook systems makes message delivery passive - agents receive messages automatically as context injections on their tool calls, without any explicit polling or distraction from their current task.

## Core Concepts

**Agents** register with a name and a description of what they're working on. Each agent has a unique identity within the project scope. Other agents can see who's online and what they're doing.

**Messages** are direct (agent-to-agent) or broadcast (to all agents). They're natural language. When a message is sent, it sits in the recipient's queue until it's delivered into their context window via a hook. Delivery is passive from the agent's perspective - they never ask for messages, messages come to them.

**File claims** are soft signals. An agent declares "I'm working on these files." If another agent attempts to edit a claimed file, they receive a warning injected into their context. The edit is not blocked. It's coordination through awareness, not enforcement.

## Architecture

**One binary.** A single Go CLI (`adm`) that handles everything - registration, messaging, file claims, and the sync/check operations that hooks call. No daemon. No server. No background processes.

**SQLite as the only dependency.** Single file, handles concurrent access, sub-millisecond reads. The database lives at a known location scoped to the project (e.g., `.agents/adm/adm.db` at the project root).

**`adm sync` is the central delivery mechanism.** It's the command that hooks call to deliver messages. It checks for unread messages for the calling agent and returns them as structured output. Any agent integration (Claude Code hooks, Codex bash hooks, or any other agent) calls `adm sync` through whatever hook mechanism is available to them.

## Delivery Semantics (V1)

ADM uses hook-managed acknowledgements so agents never need to manually ack messages or run extra commands.

1. Hook loads the agent's previously saved `ack_token` from local hook state (if present).
2. Hook runs `adm sync --agent <name> --ack-token <previous-token> --format json`.
3. `adm sync` atomically:
   - acknowledges the previously offered batch referenced by `ack_token`
   - returns the next batch of messages and a new `batch_token`
4. Hook injects returned messages into the agent context window.
5. Only after successful injection, hook saves `batch_token` as the next `ack_token`.

This keeps delivery passive and natural for the recipient agent while avoiding silent drops when hooks fail.

### Message States

- `pending`: message is queued and has not been offered yet
- `offered`: message was returned in a sync batch and is waiting for acknowledgement
- `delivered`: hook acknowledged the offered batch on a subsequent sync call

### Failure Behavior

- If a hook crashes before saving `batch_token`, the same offered messages may be returned again (safe duplicate delivery).
- If no further tool calls happen, acknowledgement is naturally delayed until the next hook-triggered sync.

### Claude Code Integration

Two Claude Code hooks provide the push mechanism for Claude-based agents:

1. **PostToolUse (all tools)** - After every tool call, runs `adm sync --agent <name> --ack-token <last-token> --format json`. If `messages` is non-empty, the hook injects them as `additionalContext` and stores `batch_token` for the next call. If `messages` is empty, the hook exits silently with no context injection. The agent never explicitly checks for messages; they arrive naturally between tool calls.

2. **PreToolUse (Edit|Write)** - Before any file edit, runs `adm check-claim --file <path> --agent <name>`. If the file is claimed by a different agent, returns a warning as `additionalContext`. The edit proceeds regardless.

### Codex Integration

Codex is primarily bash-driven - file searching, reading, and writing happen through bash commands. Codex patches its shell environment with a hook that runs on every bash command, calling `adm sync` to receive messages and storing `ack_token` transparently between calls. This is a natural injection point since nearly all of Codex's operations flow through bash.

### Other Agent Integration

Any agent with terminal access can use the `adm` CLI directly. Agents without hook systems can call `adm inbox` to read messages explicitly. The core system is agent-agnostic; hooks are adapters.

## CLI Interface

```
adm register --name <name> --task <description>    # announce presence
adm send --to <name> --msg <text>                  # direct message
adm broadcast --msg <text>                         # message all agents
adm inbox --agent <name>                           # read messages (for agents without hooks)
adm claim --agent <name> <path-pattern>            # signal file ownership
adm unclaim --agent <name> <path-pattern>          # release file ownership
adm status                                         # who's online, what they're doing
adm sync --agent <name> --format json              # called by hooks; returns messages + batch_token
adm sync --agent <name> --ack-token <token> --format json
adm check-claim --file <path> --agent <name>       # called by hooks; checks file claims
```

Example `adm sync` response:

```json
{
  "messages": [
    {
      "id": "msg_123",
      "from": "agent-b",
      "to": "agent-a",
      "body": "I am editing gg/docs/spec.md; avoid touching for now.",
      "created_at": "2026-02-08T18:10:00Z"
    }
  ],
  "batch_token": "bat_9f3cf7c8"
}
```

## Implementation

**Language: Go.** Single static binary, no runtime dependencies, fast startup time (critical since this runs on every tool call), excellent SQLite bindings, and trivial cross-compilation.

**Performance target:** The binary needs to start, query SQLite, return output, and exit in single-digit milliseconds. No long-running processes, no goroutine pools, no unnecessary complexity.

**SQLite defaults:** WAL mode and a busy timeout are enabled by default to handle concurrent access from multiple agents/hooks.

## What ADM Is Not

- Not a task management system (that's a separate concern)
- Not a file locking mechanism (claims are signals, not locks)
- Not tied to any specific agent (CLI is universal; hooks are agent-specific adapters)
- Not a daemon or service (stateless CLI + SQLite)
