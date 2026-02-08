#!/usr/bin/env bash
# shell-hook.sh - Codex shell hook for ADM message delivery
#
# Source this file in your shell profile or Codex shell configuration to
# receive ADM messages between bash commands. Messages are displayed inline
# in the terminal output.
#
# Requirements:
#   - adm binary on PATH
#   - jq installed
#   - ADM_AGENT environment variable set to the agent name
#
# Usage:
#   export ADM_AGENT="my-agent"
#   source /path/to/hooks/codex/shell-hook.sh
#
# How it works:
#   Hooks into PROMPT_COMMAND so adm sync runs before each prompt.
#   A cooldown guard (default 2 seconds) prevents excessive calls during
#   rapid command sequences.

# Skip if not interactive or agent name not set.
[[ $- == *i* ]] || return 0
[[ -n "${ADM_AGENT:-}" ]] || return 0

# Skip if adm or jq not available.
command -v adm &>/dev/null || return 0
command -v jq &>/dev/null || return 0

_ADM_LAST_SYNC=0
_ADM_COOLDOWN="${ADM_COOLDOWN:-2}"

_adm_sync() {
    local now
    now=$(date +%s)

    # Cooldown guard.
    if (( now - _ADM_LAST_SYNC < _ADM_COOLDOWN )); then
        return
    fi
    _ADM_LAST_SYNC=$now

    local state_dir=".agents/adm/state"
    local token_file="${state_dir}/${ADM_AGENT}.ack_token"

    mkdir -p "$state_dir" 2>/dev/null || return

    # Read previous ack token.
    local ack_token=""
    [[ -f "$token_file" ]] && ack_token=$(cat "$token_file")

    # Build sync command.
    local -a args=(sync --agent "$ADM_AGENT" --format json)
    [[ -n "$ack_token" ]] && args+=(--ack-token "$ack_token")

    # Run sync.
    local result
    result=$(adm "${args[@]}" 2>/dev/null) || return

    local msg_count batch_token
    msg_count=$(echo "$result" | jq '.messages | length' 2>/dev/null) || return
    batch_token=$(echo "$result" | jq -r '.batch_token // empty' 2>/dev/null) || return

    if [[ "$msg_count" -eq 0 ]]; then
        return
    fi

    # Display messages.
    echo ""
    echo "=== ADM: ${msg_count} new message(s) ==="
    echo "$result" | jq -r '.messages[] | "  From \(.from): \(.body)"' 2>/dev/null
    echo "==================================="
    echo ""

    # Save token only after successful display.
    if [[ -n "$batch_token" ]]; then
        echo -n "$batch_token" > "$token_file"
    fi
}

# Hook into PROMPT_COMMAND.
if [[ -n "${PROMPT_COMMAND:-}" ]]; then
    PROMPT_COMMAND="_adm_sync; ${PROMPT_COMMAND}"
else
    PROMPT_COMMAND="_adm_sync"
fi
