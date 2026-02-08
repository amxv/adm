#!/usr/bin/env bash
# shell-hook.sh - Codex shell hook for ADM message delivery
#
# This file supports two modes:
# 1) Shim mode (preferred for Codex): defines `adm_sync` for external callers.
# 2) Interactive shell mode: can be sourced directly to attach to PROMPT_COMMAND.

_adm_find_repo_root() {
    local dir="${1:-$PWD}"
    while :; do
        if [[ -d "${dir}/.git" || -f "${dir}/.git" ]]; then
            printf '%s\n' "$dir"
            return 0
        fi
        local parent
        parent="$(dirname "$dir")"
        [[ "$parent" == "$dir" ]] && break
        dir="$parent"
    done
    return 1
}

_adm_get_agent_name() {
    # Preferred explicit name; fallback to shim-provided ID.
    if [[ -n "${ADM_AGENT:-}" ]]; then
        printf '%s\n' "$ADM_AGENT"
        return 0
    fi
    if [[ -n "${ADM_AGENT_ID:-}" ]]; then
        printf '%s\n' "$ADM_AGENT_ID"
        return 0
    fi
    return 1
}

adm_sync() {
    command -v jq >/dev/null 2>&1 || return 0

    local agent
    agent="$(_adm_get_agent_name)" || return 0

    local root
    root="$(_adm_find_repo_root "${ADM_CWD:-$PWD}")" || return 0

    local adm_bin=""
    if command -v adm >/dev/null 2>&1; then
        adm_bin="adm"
    elif [[ -x "${root}/adm" ]]; then
        adm_bin="${root}/adm"
    else
        return 0
    fi

    local state_dir token_file ack_token
    state_dir="${root}/.agents/adm/state"
    token_file="${state_dir}/${agent}.ack_token"
    mkdir -p "$state_dir" 2>/dev/null || return 0

    ack_token=""
    [[ -f "$token_file" ]] && ack_token="$(cat "$token_file" 2>/dev/null)"

    local -a args=(sync --agent "$agent" --format json)
    [[ -n "$ack_token" ]] && args+=(--ack-token "$ack_token")

    local result
    result="$("${adm_bin}" "${args[@]}" 2>/dev/null)" || return 0

    local msg_count batch_token
    msg_count="$(printf '%s' "$result" | jq '.messages | length' 2>/dev/null)" || return 0
    batch_token="$(printf '%s' "$result" | jq -r '.batch_token // empty' 2>/dev/null)" || return 0
    [[ "${msg_count:-0}" -eq 0 ]] && return 0

    local formatted
    formatted="$(printf '%s' "$result" | jq -r '.messages[] | "[A2A_MSG_BEGIN id=\(.id) from=\(.from) at=\(.created_at)]\n\(.body)\n[A2A_MSG_END]"' 2>/dev/null)" || return 0

    local max_bytes
    max_bytes="${ADM_SYNC_MAX_BYTES:-4096}"
    if [[ "${#formatted}" -gt "$max_bytes" ]]; then
        formatted="${formatted:0:max_bytes}
[A2A_MSG_TRUNCATED]"
    fi

    # Emit on stderr to avoid breaking stdout-oriented command pipelines.
    printf '\n%s\n' "$formatted" >&2

    # Save token only after successful output.
    if [[ -n "$batch_token" ]]; then
        printf '%s' "$batch_token" >| "$token_file"
    fi
}

# Optional interactive mode for manual shell usage.
if [[ $- == *i* ]]; then
    _ADM_LAST_SYNC=0
    _ADM_COOLDOWN="${ADM_COOLDOWN:-2}"
    _adm_prompt_sync() {
        local now
        now=$(date +%s)
        if (( now - _ADM_LAST_SYNC < _ADM_COOLDOWN )); then
            return
        fi
        _ADM_LAST_SYNC=$now
        adm_sync
    }
    if [[ -n "${PROMPT_COMMAND:-}" ]]; then
        PROMPT_COMMAND="_adm_prompt_sync; ${PROMPT_COMMAND}"
    else
        PROMPT_COMMAND="_adm_prompt_sync"
    fi
fi
