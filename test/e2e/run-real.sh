#!/usr/bin/env bash
# Real-container integration tests for ctrwatch.
# Requires Docker or Podman runtime.
# Usage: CTRWATCH_INTEGRATION=1 ./test/e2e/run-real.sh [--binary ./ctrwatch] [--runtime docker|podman]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
BINARY="${BINARY:-$REPO_DIR/ctrwatch}"
RUNTIME="${RUNTIME:-}"
PASS=0
FAIL=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --binary) BINARY="$2"; shift 2 ;;
    --runtime) RUNTIME="$2"; shift 2 ;;
    *) echo "unknown: $1"; exit 1 ;;
  esac
done

if [[ -z "${CTRWATCH_INTEGRATION:-}" ]]; then
  echo "SKIP: set CTRWATCH_INTEGRATION=1 to run real-container tests" >&2
  exit 0
fi

if [[ ! -x "$BINARY" ]]; then
  echo "building ctrwatch..."
  go build -o "$BINARY" "$REPO_DIR/."
fi

if [[ -n "$RUNTIME" ]]; then
  if [[ "$RUNTIME" != "docker" && "$RUNTIME" != "podman" ]]; then
    echo "unknown runtime: $RUNTIME" >&2
    exit 1
  fi
  if ! command -v "$RUNTIME" &>/dev/null || ! "$RUNTIME" info &>/dev/null 2>&1; then
    echo "SKIP: $RUNTIME runtime unavailable" >&2
    exit 0
  fi
else
  if command -v docker &>/dev/null && docker info &>/dev/null 2>&1; then
    RUNTIME="docker"
  elif command -v podman &>/dev/null && podman info &>/dev/null 2>&1; then
    RUNTIME="podman"
  else
    echo "SKIP: no Docker or Podman runtime available" >&2
    exit 0
  fi
fi

echo "=== Using runtime: $RUNTIME"

# Start a test container
CONTAINER="ctrwatch-e2e-test-$$"
echo "=== Starting test container: $CONTAINER"
$RUNTIME run -d --name "$CONTAINER" --rm alpine:latest sleep 60

cleanup() {
  echo "=== Cleaning up..."
  $RUNTIME rm -f "$CONTAINER" 2>/dev/null || true
}
trap cleanup EXIT

pass() { PASS=$((PASS+1)); echo "  PASS: $*"; }
fail() { FAIL=$((FAIL+1)); echo "  FAIL: $*"; }

assert_contains() {
  local label="$1" expected="$2" actual="$3"
  if echo "$actual" | grep -Fq "$expected"; then
    pass "$label"
  else
    fail "$label — expected to contain: $expected"
    echo "    output: $(echo "$actual" | head -3)"
  fi
}

echo "=== Running real-container tests..."

# Test 1: ps lists the test container
OUT=$("$BINARY" ps 2>&1 || true)
assert_contains "ps shows test container" "$CONTAINER" "$OUT"

# Test 2: ps --all includes it
OUT=$("$BINARY" ps --all 2>&1 || true)
assert_contains "ps --all shows test container" "$CONTAINER" "$OUT"

# Test 3: inspect works
OUT=$("$BINARY" inspect "$CONTAINER" 2>&1 || true)
assert_contains "inspect shows name" "$CONTAINER" "$OUT"
assert_contains "inspect shows alpine" "alpine" "$OUT"

# Test 4: stats works
OUT=$("$BINARY" stats "$CONTAINER" 2>&1 || true)
assert_contains "stats shows container" "$CONTAINER" "$OUT"
assert_contains "stats shows CPU" "CPU" "$OUT"
assert_contains "stats shows MEM" "MEM" "$OUT"

# Test 5: logs works (container may not produce logs, but command should succeed)
EXIT=0
timeout 3 "$BINARY" logs --tail 10 "$CONTAINER" >/dev/null 2>&1 || EXIT=$?
if [[ "$EXIT" -eq 124 || "$EXIT" -eq 0 ]]; then
  pass "logs executed (exit=$EXIT)"
else
  fail "logs crashed (exit=$EXIT)"
fi

# Test 6: TUI watch starts (requires TTY, skip in non-interactive contexts)
if command -v script &>/dev/null; then
  EXIT=0
  timeout 3 script -q -c "$BINARY watch $CONTAINER" /dev/null >/dev/null 2>&1 || EXIT=$?
  if [[ "$EXIT" -eq 124 || "$EXIT" -eq 0 ]]; then
    pass "watch executed (exit=$EXIT)"
  else
    fail "watch crashed (exit=$EXIT)"
  fi
else
  pass "watch skipped (no PTY available)"
fi

echo
echo "=== Results: $PASS passed, $FAIL failed ==="
if [[ "$FAIL" -gt 0 ]]; then
  exit 1
fi
