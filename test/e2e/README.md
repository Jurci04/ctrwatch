# E2E Tests

## Mock server tests (no runtime required)

```bash
./test/e2e/run.sh
```

Builds ctrwatch + a mock Docker Engine API server, runs 33 tests against both
TCP and Unix socket modes. Zero dependencies beyond Go.

## Real container tests (requires Docker or Podman)

```bash
CTRWATCH_INTEGRATION=1 ./test/e2e/run-real.sh
```

Starts an Alpine container and runs ps/inspect/stats/logs/watch against it.
Skips automatically when no runtime is detected. Pass `--runtime docker` or
`--runtime podman` to avoid auto-detection when both are installed.

## Podman

The mock tests cover the Docker-compatible Engine API that Podman also exposes.
To test against a real Podman socket:

```bash
# Install Podman (https://podman.io/docs/installation)
sudo apt install podman  # Debian/Ubuntu

# Run with Podman
CTRWATCH_INTEGRATION=1 ./test/e2e/run-real.sh --runtime podman
```

The runtime client auto-detects Podman sockets at:
- `/run/podman/podman.sock`
- `/run/user/$UID/podman/podman.sock`

Use `ctrwatch.yaml` to select a specific Podman socket.

## Manual Podman checklist

Use this before claiming Podman support is release-ready.

Build once:

```bash
go build -o ./ctrwatch .
```

Start two Podman containers with visible logs:

```bash
podman run -d --name ctrwatch-podman-api --rm alpine sh -c 'while true; do echo api; sleep 1; done'
podman run -d --name ctrwatch-podman-worker --rm alpine sh -c 'while true; do echo worker; sleep 1; done'
```

Add the rootless Podman socket to `ctrwatch.yaml`:

```yaml
servers:
  - host: localhost
    socket: /run/user/1000/podman/podman.sock
    containers:
      - ctrwatch-podman-api
      - ctrwatch-podman-worker
    tags: [podman]
```

Run CLI checks:

```bash
./ctrwatch ps @podman
./ctrwatch inspect @podman
./ctrwatch stats @podman
timeout 3 ./ctrwatch logs --tail 5 @podman
```

Run the real integration test against Podman:

```bash
CTRWATCH_INTEGRATION=1 ./test/e2e/run-real.sh --runtime podman --binary ./ctrwatch
```

Run the TUI:

```bash
./ctrwatch
```

Check that both containers appear in PS, stats update, logs stream, inspect
opens, and diff/top do not crash.

Test Docker and Podman together by writing a temporary config:

```bash
cat > /tmp/ctrwatch-mixed.yaml <<EOF
servers:
  - host: localhost
    socket: /var/run/docker.sock
    containers:
      - docker-container-name
    tags: [mixed]
  - host: localhost
    socket: /run/user/$(id -u)/podman/podman.sock
    containers:
      - ctrwatch-podman-api
      - ctrwatch-podman-worker
    tags: [mixed]
EOF

CTRWATCH_CONFIG=/tmp/ctrwatch-mixed.yaml ./ctrwatch stats @mixed
CTRWATCH_CONFIG=/tmp/ctrwatch-mixed.yaml ./ctrwatch
```

For remote Podman, use an SSH alias from `~/.ssh/config` and the remote user's
socket path:

```yaml
servers:
  - host: prod-api
    socket: /run/user/1000/podman/podman.sock
    containers:
      - api
    tags: [podman-remote]
```

Cleanup:

```bash
podman rm -f ctrwatch-podman-api ctrwatch-podman-worker
rm -f /tmp/ctrwatch-mixed.yaml
```
