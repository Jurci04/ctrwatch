# E2E Tests

## Mock server tests (no runtime required)

```bash
./test/e2e/run.sh
```

Builds ctrwatch + a mock Docker Engine API server, runs 29 tests against both
TCP (`DOCKER_HOST`) and Unix socket modes. Zero dependencies beyond Go.

## Real container tests (requires Docker or Podman)

```bash
CTRWATCH_INTEGRATION=1 ./test/e2e/run-real.sh
```

Starts an Alpine container and runs ps/inspect/stats/logs/watch against it.
Skips automatically when no runtime is detected.

## Podman

The mock tests cover the Docker-compatible Engine API that Podman also exposes.
To test against a real Podman socket:

```bash
# Install Podman (https://podman.io/docs/installation)
sudo apt install podman  # Debian/Ubuntu

# Run with Podman socket
DOCKER_HOST=unix:///run/user/$(id -u)/podman/podman.sock \
  CTRWATCH_INTEGRATION=1 ./test/e2e/run-real.sh
```

The runtime client auto-detects Podman sockets at:
- `/run/podman/podman.sock`
- `/run/user/$UID/podman/podman.sock`

Or set `DOCKER_HOST` to any Docker-compatible socket path.
