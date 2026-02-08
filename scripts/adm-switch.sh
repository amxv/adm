#!/usr/bin/env bash
# adm-switch.sh - Switch the active agent identity
#
# Usage:
#   scripts/adm-switch.sh frontend
#   scripts/adm-switch.sh backend "working on API endpoints"
#
# Arguments:
#   $1 - Agent name (required)
#   $2 - Task description (optional, defaults to "ready")
#
# This writes the agent name to .agents/adm/agent (read by hooks)
# and registers/updates the agent in the database.

set -euo pipefail

NAME="${1:?Usage: adm-switch.sh <name> [task]}"
TASK="${2:-ready}"

# Find repo root.
ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
STATE_DIR="${ROOT}/.agents/adm"
mkdir -p "${STATE_DIR}"

# Write agent identity file.
echo -n "$NAME" > "${STATE_DIR}/agent"

# Find adm binary.
ADM=""
if command -v adm &>/dev/null; then
    ADM="adm"
elif [[ -x "${ROOT}/adm" ]]; then
    ADM="${ROOT}/adm"
fi

# Register if adm is available.
if [[ -n "$ADM" ]]; then
    "$ADM" register --name "$NAME" --task "$TASK"
else
    echo "Set to: ${NAME} (adm not found, skipped registration)"
fi
