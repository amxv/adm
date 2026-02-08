#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "ERROR: required command not found: $1" >&2
    exit 1
  fi
}

assert_eq() {
  local got="$1" expected="$2" msg="$3"
  if [[ "$got" != "$expected" ]]; then
    echo "ASSERT FAILED: ${msg}. expected='${expected}' got='${got}'" >&2
    exit 1
  fi
}

assert_contains() {
  local haystack="$1" needle="$2" msg="$3"
  if [[ "$haystack" != *"$needle"* ]]; then
    echo "ASSERT FAILED: ${msg}. missing='${needle}'" >&2
    exit 1
  fi
}

require_cmd go
require_cmd jq
require_cmd sqlite3
require_cmd bash

cd "$ROOT_DIR"
go build -o adm ./cmd/adm

TMP_DIR="$(mktemp -d)"
trap '/bin/rm -rf "$TMP_DIR"' EXIT

mkdir -p "$TMP_DIR/.git"
mkdir -p "$TMP_DIR/hooks"
cp "$ROOT_DIR/adm" "$TMP_DIR/adm"
cp -R "$ROOT_DIR/hooks/claude" "$TMP_DIR/hooks/"
cp -R "$ROOT_DIR/hooks/codex" "$TMP_DIR/hooks/"
chmod +x "$TMP_DIR/adm" "$TMP_DIR/hooks/claude"/*.sh "$TMP_DIR/hooks/codex"/*.sh

cd "$TMP_DIR"

echo "[1/6] Registering agents in isolated workspace..."
./adm register --name sender --task smoke >/dev/null
./adm register --name codex-e2e --task smoke >/dev/null
./adm register --name claude-e2e --task smoke >/dev/null
./adm register --name claimer --task smoke >/dev/null

echo "[2/6] Testing Codex hook (offer on first sync)..."
./adm send --from sender --to codex-e2e --msg "codex hook smoke" >/dev/null
codex_out_1="$(ADM_AGENT=codex-e2e ADM_SYNC_MAX_BYTES=4096 TMP_SMOKE_DIR="$TMP_DIR" env -u BASH_ENV bash -c 'cd "$TMP_SMOKE_DIR"; source "./hooks/codex/shell-hook.sh"; adm_sync' 2>&1 || true)"
assert_contains "$codex_out_1" "[A2A_MSG_BEGIN" "Codex hook should emit message begin marker"
assert_contains "$codex_out_1" "codex hook smoke" "Codex hook should emit message body"
offered_count="$(sqlite3 .agents/adm/adm.db "select count(*) from message_receipts where recipient_name='codex-e2e' and state='offered';")"
assert_eq "$offered_count" "1" "Message should be offered after first Codex sync"
[[ -s .agents/adm/state/codex-e2e.ack_token ]] || { echo "ASSERT FAILED: missing ack token after first Codex sync" >&2; exit 1; }

echo "[3/6] Testing Codex hook (ack on second sync)..."
ADM_AGENT=codex-e2e ADM_SYNC_MAX_BYTES=4096 TMP_SMOKE_DIR="$TMP_DIR" env -u BASH_ENV bash -c 'cd "$TMP_SMOKE_DIR"; source "./hooks/codex/shell-hook.sh"; adm_sync' >/dev/null 2>&1 || true
delivered_count="$(sqlite3 .agents/adm/adm.db "select count(*) from message_receipts where recipient_name='codex-e2e' and state='delivered';")"
assert_eq "$delivered_count" "1" "Message should be delivered after second Codex sync"

echo "[4/6] Testing Claude PostToolUse hook..."
./adm send --from sender --to claude-e2e --msg "claude post hook smoke" >/dev/null
post_json="$(echo '{}' | ADM_SYNC_DISABLE=1 ADM_AGENT=claude-e2e PATH="$TMP_DIR:$PATH" ./hooks/claude/post-tool-sync.sh)"
printf '%s' "$post_json" | jq -e '.hookSpecificOutput.hookEventName == "PostToolUse"' >/dev/null
post_ctx="$(printf '%s' "$post_json" | jq -r '.hookSpecificOutput.additionalContext')"
assert_contains "$post_ctx" "claude post hook smoke" "PostToolUse additionalContext should include message body"

echo "[5/6] Testing Claude PreToolUse claim warning hook..."
./adm claim --agent claimer gg/docs/spec.md >/dev/null
pre_json="$(echo '{"tool_input":{"file_path":"gg/docs/spec.md"}}' | ADM_SYNC_DISABLE=1 ADM_AGENT=reviewer PATH="$TMP_DIR:$PATH" ./hooks/claude/pre-tool-claim-check.sh)"
printf '%s' "$pre_json" | jq -e '.hookSpecificOutput.hookEventName == "PreToolUse"' >/dev/null
printf '%s' "$pre_json" | jq -e '.hookSpecificOutput.permissionDecision == "allow"' >/dev/null
pre_ctx="$(printf '%s' "$pre_json" | jq -r '.hookSpecificOutput.additionalContext')"
assert_contains "$pre_ctx" "claimer" "PreToolUse warning should include claim owner"

echo "[6/6] All hook smoke tests passed."
echo "Workspace: $TMP_DIR"
