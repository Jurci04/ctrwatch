#!/usr/bin/env bash
# E2E test runner for ctrwatch.
# Builds ctrwatch, starts a mock Docker API server, and runs commands against it.
# Usage: ./test/e2e/run.sh [--build] [--binary ./ctrwatch]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
BINARY="${BINARY:-$REPO_DIR/ctrwatch}"
BUILD="${BUILD:-1}"
PASS=0
FAIL=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --binary) BINARY="$2"; shift 2 ;;
    --no-build) BUILD=0; shift ;;
    *) echo "unknown: $1"; exit 1 ;;
  esac
done

if [[ "$BUILD" == "1" ]]; then
  echo "=== Building ctrwatch..."
  go build -o "$BINARY" "$REPO_DIR"
fi

if [[ ! -x "$BINARY" ]]; then
  echo "binary not found: $BINARY (use --binary or --no-build)" >&2
  exit 1
fi

echo "=== Building mock server..."
MOCK_SERVER="$REPO_DIR/test/e2e/mockserver"
go build -o "$MOCK_SERVER/mockserver" "$MOCK_SERVER"

SOCK_DIR=$(mktemp -d)
UNIX_SOCKET="$SOCK_DIR/podman.sock"
PORT_FILE=$(mktemp)

echo "=== Starting mock server (TCP + Unix socket)..."
"$MOCK_SERVER/mockserver" --socket "$UNIX_SOCKET" > "$PORT_FILE" 2>&1 &
MOCK_PID=$!
trap "kill $MOCK_PID 2>/dev/null; rm -f $PORT_FILE $UNIX_SOCKET; rmdir $SOCK_DIR 2>/dev/null; wait $MOCK_PID 2>/dev/null" EXIT

# Read port from temp file
for i in $(seq 1 10); do
  PORT=$(grep -oP 'PORT=\K\d+' "$PORT_FILE" 2>/dev/null || echo "")
  if [[ -n "$PORT" ]]; then
    break
  fi
  sleep 0.1
done
if [[ -z "$PORT" ]]; then
  echo "ERROR: could not read mock server port" >&2
  cat "$PORT_FILE" >&2
  exit 1
fi

echo "  TCP port: $PORT"
echo "  Unix socket: $UNIX_SOCKET"

pass() { PASS=$((PASS+1)); echo "  PASS: $*"; }
fail() { FAIL=$((FAIL+1)); echo "  FAIL: $*"; }

assert_contains() {
  local label="$1" expected="$2" actual="$3"
  if echo "$actual" | grep -Fq "$expected"; then
    pass "$label"
  else
    fail "$label — expected to contain: $expected"
    echo "    output: $actual" | head -5
  fi
}

echo "=== Running E2E tests..."

# ── TCP via DOCKER_HOST ──────────────────────────────────────────

TCP_HOST="tcp://127.0.0.1:$PORT"

# Test 1: ps lists containers
OUT=$(DOCKER_HOST="$TCP_HOST" "$BINARY" ps 2>&1 || true)
assert_contains "tcp ps shows nginx" "nginx" "$OUT"
assert_contains "tcp ps shows redis" "redis" "$OUT"
assert_contains "tcp ps shows worker" "worker" "$OUT"

# Test 2: ps --all includes stopped containers
OUT=$(DOCKER_HOST="$TCP_HOST" "$BINARY" ps --all 2>&1 || true)
assert_contains "tcp ps --all shows worker (exited)" "worker" "$OUT"

# Test 3: inspect shows container metadata
OUT=$(DOCKER_HOST="$TCP_HOST" "$BINARY" inspect nginx 2>&1 || true)
assert_contains "tcp inspect shows name" "nginx" "$OUT"
assert_contains "tcp inspect shows image" "nginx:1.25" "$OUT"
assert_contains "tcp inspect shows status" "running" "$OUT"
assert_contains "tcp inspect shows mounts" "/data" "$OUT"

# Test 4: inspect unknown container
OUT=$(DOCKER_HOST="$TCP_HOST" "$BINARY" inspect nonexistent 2>&1 || true)
assert_contains "tcp inspect unknown returns error" "404" "$OUT"

# Test 5: stats show CPU and memory
OUT=$(DOCKER_HOST="$TCP_HOST" "$BINARY" stats nginx 2>&1 || true)
assert_contains "tcp stats shows nginx" "nginx" "$OUT"
assert_contains "tcp stats shows CPU" "CPU" "$OUT"
assert_contains "tcp stats shows MEM" "MEM" "$OUT"

# Test 6: stats multiple containers
OUT=$(DOCKER_HOST="$TCP_HOST" "$BINARY" stats nginx redis 2>&1 || true)
assert_contains "tcp stats multi shows nginx" "nginx" "$OUT"
assert_contains "tcp stats multi shows redis" "redis" "$OUT"

# Test 7: logs streams log lines
OUT=$(DOCKER_HOST="$TCP_HOST" timeout 2 "$BINARY" logs --tail 10 nginx 2>&1 || true)
assert_contains "tcp logs shows startup" "Starting nginx" "$OUT"
assert_contains "tcp logs shows INFO" "INFO:" "$OUT"

# Test 8: help works
OUT=$("$BINARY" help 2>&1 || true)
assert_contains "help shows ps" "ps" "$OUT"
assert_contains "help shows logs" "logs" "$OUT"

# Test 9: config check with no config
OUT=$(CTRWATCH_CONFIG=/nonexistent.yaml DOCKER_HOST="$TCP_HOST" "$BINARY" config check 2>&1 || true)
assert_contains "config check fails without config" "Error" "$OUT"

# Test 10: unknown command
OUT=$("$BINARY" bogus 2>&1 || true)
assert_contains "unknown command shows usage" "Usage" "$OUT"

# Test 11: default (no args) opens TUI
EXIT=0
DOCKER_HOST="$TCP_HOST" timeout 1 "$BINARY" 2>/dev/null || EXIT=$?
if [[ "$EXIT" -eq 124 || "$EXIT" -eq 0 ]]; then
  pass "default TUI starts (exit=$EXIT)"
else
  fail "default TUI crashed (exit=$EXIT)"
fi

# ── Unix socket (Podman-compatible) ───────────────────────────────

# Test 12: name@unix-socket syntax
OUT=$(DOCKER_HOST="$TCP_HOST" "$BINARY" inspect "nginx@unix://$UNIX_SOCKET" 2>&1 || true)
assert_contains "socket override unix shows name" "nginx" "$OUT"

# Test 13: DOCKER_HOST with unix socket
OUT=$(DOCKER_HOST="unix://$UNIX_SOCKET" "$BINARY" ps 2>&1 || true)
assert_contains "unix socket ps shows nginx" "nginx" "$OUT"
assert_contains "unix socket ps shows redis" "redis" "$OUT"

# Test 14: inspect via unix socket
OUT=$(DOCKER_HOST="unix://$UNIX_SOCKET" "$BINARY" inspect nginx 2>&1 || true)
assert_contains "unix socket inspect shows image" "nginx:1.25" "$OUT"

# Test 15: stats via unix socket
OUT=$(DOCKER_HOST="unix://$UNIX_SOCKET" "$BINARY" stats redis 2>&1 || true)
assert_contains "unix socket stats shows redis" "redis" "$OUT"

# Test 16: logs via unix socket
OUT=$(DOCKER_HOST="unix://$UNIX_SOCKET" timeout 2 "$BINARY" logs --tail 10 nginx 2>&1 || true)
assert_contains "unix socket logs shows startup" "Starting nginx" "$OUT"

# Test 17: socket override with bare path
OUT=$(DOCKER_HOST="$TCP_HOST" "$BINARY" inspect "nginx@$UNIX_SOCKET" 2>&1 || true)
assert_contains "socket override bare path" "nginx" "$OUT"

# ── JSON output ──────────────────────────────────────────────────

# Test 18: inspect --json returns valid JSON
OUT=$(DOCKER_HOST="$TCP_HOST" "$BINARY" inspect --json nginx 2>&1 || true)
assert_contains "inspect --json has image" "nginx:1.25" "$OUT"
assert_contains "inspect --json has Id" "abc123" "$OUT"

# Test 19: stats --json returns valid JSON
OUT=$(DOCKER_HOST="$TCP_HOST" "$BINARY" stats --json nginx 2>&1 || true)
assert_contains "stats --json has name" "nginx" "$OUT"
assert_contains "stats --json has cpu" "cpu_percent" "$OUT"

# ── Podman auto-detection ────────────────────────────────────────

# Test 20: Client auto-detects socket via DOCKER_HOST
OUT=$(DOCKER_HOST="unix://$UNIX_SOCKET" "$BINARY" ps --all 2>&1 || true)
assert_contains "auto-detect ps shows worker" "worker" "$OUT"

echo
echo "=== Results: $PASS passed, $FAIL failed ==="
if [[ "$FAIL" -gt 0 ]]; then
  exit 1
fi
