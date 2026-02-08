#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 || $# -gt 2 ]]; then
  echo "Usage: $0 <agent-name> [task]" >&2
  echo "Example: $0 frontend \"working on UI\"" >&2
  exit 1
fi

AGENT_NAME="$1"
TASK="${2:-ready}"

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
STATE_DIR="$ROOT_DIR/.agents/adm"
AGENT_FILE="$STATE_DIR/agent"

mkdir -p "$STATE_DIR"
printf '%s' "$AGENT_NAME" > "$AGENT_FILE"

if [[ -x "$ROOT_DIR/adm" ]]; then
  "$ROOT_DIR/adm" register --name "$AGENT_NAME" --task "$TASK" >/dev/null || true
elif command -v adm >/dev/null 2>&1; then
  adm register --name "$AGENT_NAME" --task "$TASK" >/dev/null || true
fi

echo "Active agent set to: $AGENT_NAME"
echo "Agent file: $AGENT_FILE"
