#!/usr/bin/env bash
# Runtime smoke test for ADM CLI.
# Runs all core commands in an isolated temp workspace with a .git marker.
# Phase 7: Runtime Validation Gate
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# --- helpers ---

PASS=0
FAIL=0

assert_eq() {
  local got="$1" expected="$2" msg="$3"
  if [[ "$got" != "$expected" ]]; then
    echo "  FAIL: ${msg}. expected='${expected}' got='${got}'" >&2
    FAIL=$((FAIL + 1))
    return 1
  fi
  PASS=$((PASS + 1))
}

assert_contains() {
  local haystack="$1" needle="$2" msg="$3"
  if [[ "$haystack" != *"$needle"* ]]; then
    echo "  FAIL: ${msg}. missing='${needle}'" >&2
    FAIL=$((FAIL + 1))
    return 1
  fi
  PASS=$((PASS + 1))
}

assert_not_contains() {
  local haystack="$1" needle="$2" msg="$3"
  if [[ "$haystack" == *"$needle"* ]]; then
    echo "  FAIL: ${msg}. should not contain='${needle}'" >&2
    FAIL=$((FAIL + 1))
    return 1
  fi
  PASS=$((PASS + 1))
}

assert_exit_nonzero() {
  local cmd="$1" msg="$2"
  if eval "$cmd" >/dev/null 2>&1; then
    echo "  FAIL: ${msg}. command should have failed but succeeded" >&2
    FAIL=$((FAIL + 1))
    return 1
  fi
  PASS=$((PASS + 1))
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "ERROR: required command not found: $1" >&2
    exit 1
  fi
}

# --- setup ---

require_cmd go
require_cmd jq

echo "=== ADM Runtime Smoke Test ==="
echo ""

echo "Building adm binary..."
cd "$ROOT_DIR"
go build -o adm ./cmd/adm

TMP_DIR="$(mktemp -d)"
trap '/bin/rm -rf "$TMP_DIR"' EXIT

# Create isolated workspace with .git marker
mkdir -p "$TMP_DIR/.git"
mkdir -p "$TMP_DIR/src/auth"
cp "$ROOT_DIR/adm" "$TMP_DIR/adm"
cd "$TMP_DIR"

ADM="./adm"
echo "Workspace: $TMP_DIR"
echo ""

# ============================================================
# 1. Register
# ============================================================
echo "[1/7] Register"

out=$($ADM register --name alice --task "building auth module")
assert_contains "$out" "registered" "register alice"

out=$($ADM register --name bob --task "writing tests")
assert_contains "$out" "registered" "register bob"

out=$($ADM register --name carol --task "code review")
assert_contains "$out" "registered" "register carol"

# Re-register updates task
out=$($ADM register --name alice --task "refactoring auth")
assert_contains "$out" "registered" "re-register alice"

echo "  $PASS passed"
echo ""

# ============================================================
# 2. Status
# ============================================================
echo "[2/7] Status"

out=$($ADM status)
assert_contains "$out" "alice" "status shows alice"
assert_contains "$out" "bob" "status shows bob"
assert_contains "$out" "carol" "status shows carol"
assert_contains "$out" "refactoring auth" "status shows updated task for alice"
assert_contains "$out" "online" "status shows online agents"

echo "  $PASS passed"
echo ""

# ============================================================
# 3. Send + Sync (delivery lifecycle)
# ============================================================
echo "[3/7] Send + Sync delivery lifecycle"

# Send direct message
out=$($ADM send --from alice --to bob --msg "hey bob, check auth module")
assert_contains "$out" "sent" "send from alice to bob"

# Send another
$ADM send --from carol --to bob --msg "bob, review PR #42" >/dev/null

# Sync 1: bob gets pending messages (offered)
sync1=$($ADM sync --agent bob --format json)
msg_count=$(echo "$sync1" | jq '.messages | length')
assert_eq "$msg_count" "2" "bob receives 2 messages on first sync"

batch_token=$(echo "$sync1" | jq -r '.batch_token')
if [[ -n "$batch_token" && "$batch_token" != "" ]]; then
  PASS=$((PASS + 1))
else
  echo "  FAIL: batch_token should be non-empty" >&2
  FAIL=$((FAIL + 1))
fi

# Verify message content
msg_body=$(echo "$sync1" | jq -r '.messages[0].body')
assert_contains "$msg_body" "auth module" "first message body correct"

# Sync 2: ack previous batch, no new messages
sync2=$($ADM sync --agent bob --ack-token "$batch_token" --format json)
msg_count2=$(echo "$sync2" | jq '.messages | length')
assert_eq "$msg_count2" "0" "no new messages after ack"

empty_token=$(echo "$sync2" | jq -r '.batch_token')
assert_eq "$empty_token" "" "empty batch_token when no messages"

# Send to unknown agent should fail
assert_exit_nonzero "$ADM send --from alice --to nobody --msg test" "send to unregistered agent fails"

# Send from unknown agent should fail
assert_exit_nonzero "$ADM send --from nobody --to bob --msg test" "send from unregistered agent fails"

echo "  $PASS passed"
echo ""

# ============================================================
# 4. Broadcast
# ============================================================
echo "[4/7] Broadcast"

$ADM broadcast --from alice --msg "team standup in 5 min" >/dev/null

# bob should receive broadcast
sync_bob=$($ADM sync --agent bob --format json)
bc_count=$(echo "$sync_bob" | jq '.messages | length')
assert_eq "$bc_count" "1" "bob receives broadcast"

bc_body=$(echo "$sync_bob" | jq -r '.messages[0].body')
assert_contains "$bc_body" "standup" "broadcast body correct for bob"

# carol should also receive it
sync_carol=$($ADM sync --agent carol --format json)
cc_count=$(echo "$sync_carol" | jq '.messages | length')
assert_eq "$cc_count" "1" "carol receives broadcast"

# alice (sender) should NOT receive own broadcast
sync_alice=$($ADM sync --agent alice --format json)
sa_count=$(echo "$sync_alice" | jq '.messages | length')
assert_eq "$sa_count" "0" "alice does not receive own broadcast"

echo "  $PASS passed"
echo ""

# ============================================================
# 5. Claim / Check-Claim / Unclaim
# ============================================================
echo "[5/7] Claim / Check-Claim / Unclaim"

$ADM claim --agent alice "src/auth/*.go" >/dev/null

# Check claim from bob's perspective
check_out=$($ADM check-claim --file src/auth/login.go --agent bob)
assert_contains "$check_out" "alice" "check-claim shows alice owns auth files"
# Verify JSON shape: claimed=true
if echo "$check_out" | jq -e '.claimed == true' >/dev/null 2>&1; then
  PASS=$((PASS + 1))
else
  echo "  FAIL: check-claim should show claimed=true for bob" >&2
  FAIL=$((FAIL + 1))
fi

# Check claim from alice's own perspective (should not be flagged)
check_self=$($ADM check-claim --file src/auth/login.go --agent alice)
if echo "$check_self" | jq -e '.claimed == false' >/dev/null 2>&1; then
  PASS=$((PASS + 1))
else
  echo "  FAIL: alice checking own claim should show claimed=false" >&2
  FAIL=$((FAIL + 1))
fi

# Unclaim
$ADM unclaim --agent alice "src/auth/*.go" >/dev/null

# After unclaim, not claimed
check_after=$($ADM check-claim --file src/auth/login.go --agent bob)
if echo "$check_after" | jq -e '.claimed == false' >/dev/null 2>&1; then
  PASS=$((PASS + 1))
else
  echo "  FAIL: not claimed after unclaim" >&2
  FAIL=$((FAIL + 1))
fi

# Re-claim (idempotent upsert)
$ADM claim --agent alice "src/auth/*.go" >/dev/null
$ADM claim --agent alice "src/auth/*.go" >/dev/null
# Should still be just one claim (not duplicated) - bob sees alice as owner
check_dup=$($ADM check-claim --file src/auth/login.go --agent bob)
if echo "$check_dup" | jq -e '.claimed == true and .owner == "alice"' >/dev/null 2>&1; then
  PASS=$((PASS + 1))
else
  echo "  FAIL: duplicate claim should still show single alice ownership" >&2
  FAIL=$((FAIL + 1))
fi

echo "  $PASS passed"
echo ""

# ============================================================
# 6. Inbox (read-only)
# ============================================================
echo "[6/7] Inbox (read-only)"

# Send a fresh message to carol
$ADM send --from bob --to carol --msg "carol, approve the deploy" >/dev/null

# Inbox should show the message without changing state
inbox_out=$($ADM inbox --agent carol)
assert_contains "$inbox_out" "approve the deploy" "inbox shows pending message"

# Call inbox again - message should still be there (read-only, no state change)
inbox_out2=$($ADM inbox --agent carol)
assert_contains "$inbox_out2" "approve the deploy" "inbox is read-only, message persists"

# Sync should still pick up the message (inbox didn't consume it)
sync_carol2=$($ADM sync --agent carol --format json)
sc2_count=$(echo "$sync_carol2" | jq '.messages | length')
# carol had 1 broadcast offered earlier + 1 new direct = at least 1 pending
if [[ "$sc2_count" -ge 1 ]]; then
  PASS=$((PASS + 1))
else
  echo "  FAIL: sync after inbox should still deliver messages (got $sc2_count)" >&2
  FAIL=$((FAIL + 1))
fi

echo "  $PASS passed"
echo ""

# ============================================================
# 7. Version
# ============================================================
echo "[7/7] Version"

ver_out=$($ADM --version 2>&1 || $ADM version 2>&1 || echo "no-version")
if [[ "$ver_out" != "no-version" ]]; then
  PASS=$((PASS + 1))
  echo "  version output: $ver_out"
else
  echo "  SKIP: version command not available"
fi

echo "  $PASS passed"
echo ""

# ============================================================
# Summary
# ============================================================
echo "=== Smoke Test Summary ==="
echo "  Passed: $PASS"
echo "  Failed: $FAIL"

if [[ "$FAIL" -gt 0 ]]; then
  echo ""
  echo "SMOKE TEST FAILED"
  exit 1
fi

echo ""
echo "ALL SMOKE TESTS PASSED"
echo ""

# ============================================================
# Run hook smoke tests
# ============================================================
echo "=== Running Hook Smoke Tests ==="
"${SCRIPT_DIR}/smoke-hooks.sh"
