#!/usr/bin/env bash
# pre-tool-claim-check.sh - Claude Code PreToolUse hook for ADM file claim warnings
#
# Runs before Edit/Write/MultiEdit tool calls. Checks if the target file
# is claimed by another agent and injects a warning. The edit is NEVER
# blocked - claims are soft signals, not locks.
#
# Requirements:
#   - adm binary on PATH
#   - jq installed
#   - ADM_AGENT environment variable set to the agent name
#
# Configuration in .claude/settings.json:
# {
#   "hooks": {
#     "PreToolUse": [
#       {
#         "matcher": "Edit|Write|MultiEdit",
#         "hooks": [
#           {
#             "type": "command",
#             "command": "/path/to/hooks/claude/pre-tool-claim-check.sh",
#             "timeout": 5
#           }
#         ]
#       }
#     ]
#   }
# }

set -euo pipefail

# Read tool input from stdin.
INPUT=$(cat)

AGENT="${ADM_AGENT:-}"
if [[ -z "$AGENT" && -f ".agents/adm/state/session.json" ]]; then
    AGENT=$(jq -r '.agent // empty' ".agents/adm/state/session.json" 2>/dev/null)
fi
if [[ -z "$AGENT" && -f ".agents/adm/agent" ]]; then
    AGENT=$(cat ".agents/adm/agent" 2>/dev/null)
fi
if [[ -z "$AGENT" ]]; then
    exit 0
fi

# Verify adm is available.
command -v adm &>/dev/null || exit 0

# Extract file path from tool input.
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty' 2>/dev/null)
if [[ -z "$FILE_PATH" ]]; then
    exit 0
fi

# Check claim. Exit silently on failure.
RESULT=$(adm check-claim --file "$FILE_PATH" --agent "$AGENT" 2>/dev/null) || exit 0

CLAIMED=$(echo "$RESULT" | jq -r '.claimed // false' 2>/dev/null)
if [[ "$CLAIMED" != "true" ]]; then
    exit 0
fi

OWNER=$(echo "$RESULT" | jq -r '.owner // "unknown"' 2>/dev/null)
PATTERN=$(echo "$RESULT" | jq -r '.pattern // "unknown"' 2>/dev/null)

WARNING="[ADM WARNING] File '${FILE_PATH}' is claimed by agent '${OWNER}' (pattern: ${PATTERN}). Coordinate with them before editing."
ESCAPED=$(echo "$WARNING" | jq -Rs '.')

# Allow the edit but inject a warning.
cat <<HOOKJSON
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "allow",
    "additionalContext": ${ESCAPED}
  }
}
HOOKJSON
