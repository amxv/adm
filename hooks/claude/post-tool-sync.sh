#!/usr/bin/env bash
# post-tool-sync.sh - Claude Code PostToolUse hook for ADM message delivery
#
# Runs after every tool call. Syncs pending messages for this agent and
# injects them as additionalContext if any are waiting.
#
# Requirements:
#   - adm binary on PATH
#   - jq installed
#   - ADM_AGENT environment variable set to the agent name
#
# Configuration in .claude/settings.json:
# {
#   "hooks": {
#     "PostToolUse": [
#       {
#         "matcher": "",
#         "hooks": [
#           {
#             "type": "command",
#             "command": "/path/to/hooks/claude/post-tool-sync.sh",
#             "timeout": 10
#           }
#         ]
#       }
#     ]
#   }
# }

set -euo pipefail

# Consume stdin (required even if unused).
cat > /dev/null

AGENT="${ADM_AGENT:-}"
if [[ -z "$AGENT" ]]; then
    exit 0
fi

# Verify adm is available.
command -v adm &>/dev/null || exit 0

STATE_DIR=".agents/adm/state"
TOKEN_FILE="${STATE_DIR}/${AGENT}.ack_token"

mkdir -p "$STATE_DIR"

# Read previous ack token.
ACK_TOKEN=""
if [[ -f "$TOKEN_FILE" ]]; then
    ACK_TOKEN=$(cat "$TOKEN_FILE")
fi

# Build sync command.
SYNC_ARGS=(sync --agent "$AGENT" --format json)
if [[ -n "$ACK_TOKEN" ]]; then
    SYNC_ARGS+=(--ack-token "$ACK_TOKEN")
fi

# Run sync. Exit silently on failure.
RESULT=$(adm "${SYNC_ARGS[@]}" 2>/dev/null) || exit 0

# Parse response.
MSG_COUNT=$(echo "$RESULT" | jq '.messages | length' 2>/dev/null) || exit 0
BATCH_TOKEN=$(echo "$RESULT" | jq -r '.batch_token // empty' 2>/dev/null) || exit 0

# No messages: exit silently (no context injection).
if [[ "$MSG_COUNT" -eq 0 ]]; then
    exit 0
fi

# Format messages for context injection.
CONTEXT="[ADM] ${MSG_COUNT} new message(s):"
while IFS= read -r line; do
    CONTEXT="${CONTEXT}
  ${line}"
done < <(echo "$RESULT" | jq -r '.messages[] | "From \(.from): \(.body)"' 2>/dev/null)

# Escape for JSON.
ESCAPED=$(echo "$CONTEXT" | jq -Rs '.')

# Output additionalContext for Claude.
cat <<HOOKJSON
{
  "hookSpecificOutput": {
    "hookEventName": "PostToolUse",
    "additionalContext": ${ESCAPED}
  }
}
HOOKJSON

# Save batch_token only after successful output.
if [[ -n "$BATCH_TOKEN" ]]; then
    echo -n "$BATCH_TOKEN" > "$TOKEN_FILE"
fi
