# ctrwatch Feature Roadmap

## Direction

`ctrwatch` is an agentless, read-only container cockpit for local and
SSH-reachable Docker-compatible runtimes. It should remain safe for production
triage: no remote agent, lifecycle actions, required database, or duplicated
credential system.

The TUI stays the default interactive entrypoint. Direct CLI commands remain
available for scripts.

## Shipped

- Docker-compatible local, TCP, and SSH runtime access, including Podman
  sockets and multiple configured sources.
- Read-only logs, container lists, stats, inspect metadata, filesystem changes,
  and running processes.
- SSH tunnel supervision with reconnect state and error reporting.
- CLI config/import workflows and an in-TUI server setup editor with SSH alias
  and container discovery.
- Log panel selection, runtime labels, configurable stats polling, and a small
  dependency-free E2E harness.

## Next

Ranked by expected value-to-effort ratio.

### 1. Reconnect State And Stale Data

Feed live session state and errors into the model continuously. Keep the last
container data visible during reconnects and label it stale instead of clearing
it on a transient failure.

Tests: reconnecting, failed, recovered, and stale rendering states.

### 2. Dynamic Container Polling

Start and maintain stats polling when containers are added after TUI startup,
including the common case where startup has no local containers and a remote
server is connected later.

Tests: connect the first server after startup and receive stats without
restarting the TUI.

### 3. Unique Container Source Identity

Key internal log, stats, selection, and cancellation state by source plus
container identity. Containers with the same runtime and name on different
servers must not share state.

Tests: two servers exposing the same container name remain independent.

### 4. Podman Confidence Pass

Run the real integration suite against rootless and system Podman sockets,
mixed Docker/Podman config, and remote Podman over SSH. Record supported paths
and fix only failures reproduced by the run.

Command: `CTRWATCH_INTEGRATION=1 ./test/e2e/run-real.sh --runtime podman`.

### 5. Container Filter

Use `/` to filter containers by name or ID across Logs, PS, and Stats. `esc`
clears the filter before changing focus.

Tests: incremental filtering, no matches, selection clamping, and clear.

### 6. Health And Problem Summary

Surface unhealthy, restarting, exited, reconnecting, and stale resources before
details. Add a compact summary and consistent highlighting while keeping every
detail one keypress away.

Tests: unhealthy, restarting, exited, reconnecting, and stale cases.

### 7. Runtime Events

Add a read-only event stream only after the problem summary is useful; events
should explain state changes rather than become another general-purpose view.

Tests: mock timed lifecycle and health events.

## Later Runtime Sources

Add sources only when they reuse existing credentials and preserve the
agentless, read-only model.

1. Docker contexts, including existing SSH endpoints.
2. Podman connections and common `podman machine` sockets.
3. Explicit Docker TCP/TLS certificate configuration when demand justifies it.

Kubernetes contexts are deferred: kubeconfig, pod/container modeling, and API
behavior would materially expand the tool beyond Docker-compatible runtimes.

Table sorting is also deferred until filtering proves insufficient. A separate
`config add` command is not planned because `config init` and the TUI editor
already cover that workflow.

## Not Planned

- Container lifecycle or Compose orchestration; use the runtime and Compose
  CLIs for start, stop, restart, and deployment actions.
- Cloud-provider SDK integrations; prefer contexts, kubeconfig-aware dedicated
  tools, or runtime sockets.
- Non-container monitoring such as systemd, cron, or tmux.

## Acceptance Criteria

- `go test ./...` and `go vet ./...` pass.
- `./test/e2e/run.sh` passes without Docker, Podman, SSH, or a real terminal.
- `CTRWATCH_INTEGRATION=1 ./test/e2e/run-real.sh` passes when a supported real
  runtime is available.
