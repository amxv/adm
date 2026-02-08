# ADM Hook Integration Guide

ADM delivers messages passively through hook systems. Agents never poll for messages; hooks call `adm sync` on tool boundaries and inject any waiting messages into the agent's context.

## Prerequisites

- `adm` binary installed and on PATH
- `jq` installed (for JSON parsing in hook scripts)
- Agent identity set with `adm use <name> --task <description>`

## Identity

Set your identity before starting a session. The recommended approach is `adm use`:

```bash
adm use my-agent --task "working on feature X"
```

This creates a session file that hooks read automatically. Alternatively, set the `ADM_AGENT` environment variable:

```bash
export ADM_AGENT="my-agent"
```

Hooks resolve identity in order: `ADM_AGENT` env var > session file > legacy fallback.

## Claude Code

Claude Code has a hook system that runs shell commands before/after tool calls. ADM uses two hooks:

1. **PostToolUse** - Runs after every tool call, delivers pending messages
2. **PreToolUse** - Runs before Edit/Write, warns about claimed files

### Setup

1. Copy the hook scripts to your project:
   ```bash
   cp -r hooks/claude/ .claude/hooks/adm/
   chmod +x .claude/hooks/adm/*.sh
   ```

2. Add to `.claude/settings.json` (or `.claude/settings.local.json` for local-only):
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

3. Set your identity:
   ```bash
   adm use my-agent --task "working on feature X"
   ```

### How it works

**Message delivery (PostToolUse):**

```
Agent makes tool call  ->  Tool executes  ->  PostToolUse hook fires
                                                |
                                                v
                                     adm sync --agent my-agent
                                                |
                                        +--------------+
                                        | messages > 0 |  ->  Inject as additionalContext
                                        +--------------+      Save batch_token for next ack
                                        | messages = 0 |  ->  Exit silently (no noise)
                                        +--------------+
```

**Claim warnings (PreToolUse):**

```
Agent requests Edit  ->  PreToolUse hook fires
                          |
                          v
               adm check-claim --file <path> --agent my-agent
                          |
                   +-------------+
                   | claimed     |  ->  Inject warning as additionalContext
                   +-------------+      (edit still proceeds)
                   | not claimed |  ->  Exit silently
                   +-------------+
```

### Multiple agents

When running multiple Claude Code sessions on the same project, each session needs a unique agent name:

```json
{
  "hooks": {
    "PostToolUse": [{
      "matcher": "",
      "hooks": [{
        "type": "command",
        "command": "ADM_AGENT=claude-1 .claude/hooks/adm/post-tool-sync.sh",
        "timeout": 10
      }]
    }]
  }
}
```

Use `.claude/settings.local.json` (not committed) so each developer can set their own agent name.

## Codex

Codex operates primarily through bash. The shell hook sources into your shell environment and runs `adm sync` between commands via `PROMPT_COMMAND`.

### Setup

1. Set agent name and source the hook:
   ```bash
   export ADM_AGENT="codex-1"
   source /path/to/hooks/codex/shell-hook.sh
   ```

   Or add to your shell profile for automatic activation.

2. Set your identity:
   ```bash
   adm use codex-1 --task "working on API"
   ```

### How it works

The hook inserts `_adm_sync` into `PROMPT_COMMAND`. Before each prompt:

1. Check cooldown (default 2 seconds, configurable via `ADM_COOLDOWN`)
2. Run `adm sync` with the stored ack token
3. If messages exist, display them inline and save the batch token
4. If no messages, do nothing

Messages appear between command outputs:

```
$ ls src/
auth/  api/  models/

=== ADM: 1 new message(s) ===
  From alice: I'm refactoring the auth module, hold off on changes there
===================================

$
```

### Cooldown

The cooldown prevents excessive sync calls during rapid command sequences. Default is 2 seconds. Adjust with:

```bash
export ADM_COOLDOWN=5  # seconds between syncs
```

## Other Agents

Any agent with terminal access can use `adm` directly:

```bash
# Check for messages (read-only, does not change state)
adm inbox --agent my-agent

# Or use sync for full delivery semantics
adm sync --agent my-agent --format json
```

For agents without hook systems, periodic `adm inbox` calls work as a simple polling alternative.

## State Files

Hook state is stored in `.agents/adm/state/`:

```
.agents/adm/state/
  session.json              # Active agent identity (set by adm use)
  my-agent.ack_token        # Last batch token for acknowledgement
  codex-1.ack_token
```

The `session.json` file is created by `adm use` and read by hooks to resolve identity. The `.ack_token` files are small (one token per agent) and safe to delete if delivery state needs to be reset.
