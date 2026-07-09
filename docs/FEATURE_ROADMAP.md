# ctrwatch Feature Roadmap

## Summary

`ctrwatch` is an agentless, read-only container cockpit for people who watch
containers across local and SSH-reachable servers. It should stay safe to run
against production hosts: no server-side agent, no lifecycle actions by
default, and no required database.

The TUI is the default entrypoint for interactive triage. Direct CLI commands
stay available for scripting.

## Positioning

- **Agentless remote access**: use the existing Docker/Podman socket through
  local access or system `ssh`; do not require a daemon on the remote host.
- **Read-only by default**: prioritize production-safe inspection over
  container management.
- **Multi-server triage**: make it obvious which server or container needs
  attention before the user drills into logs, stats, inspect, diff, or top.
- **Small tool, boring setup**: config should be easy to create, easy to read,
  and compatible with normal SSH config aliases, keys, agents, and jump hosts.

## Completed

- Server state tracking: each server shows `local`, `connected`, `connecting`,
  `reconnecting`, or `failed`. — **done**
- Surface last SSH/runtime error in the servers view. — **done**
- SSH tunnel supervisor with probe, reconnect, and bounded exponential backoff
  (inlined, no dependency). — **done**
- CLI `config init` multi-server loop and TUI setup wizard (`e` key). — **done**
- Setup wizard textinput widget integration (cursor, scroll, placeholder,
  focus/blur). — **done**
- Setup form field navigation via `tab`/`shift+tab`, `enter` to save. — **done**
- SSH alias cycling (`ctrl+a`) and container discovery (`ctrl+p`) in setup. — **done**
- Socket detection hints only for localhost. — **done**
- Log container selector (`m` key): checkbox list with `[x]`/`[ ]` to choose
  which containers appear in the split-screen panels. — **done**
- Binary: 7.6MB static, stripped, CGO_ENABLED=0, -trimpath. — **done**
- Build dependencies removed: `backoff/v7`, `gopkg.in/check.v1`. — **done**
- Ponytail audit cleanup: -15 lines (duplicate socketPath, dead line,
  single-caller helpers). — **done**

## Next

Ranked by expected value-to-effort ratio.

### 1. Stale Data Label While Reconnecting

Keep stale container data visible while reconnecting, clearly marked
stale. — **still needed** (`TODO(tui)` in `model.go` and
`runtime_commands.go`)

Effort: small (stale data labeling in view, one docs section).

### 2. Container Name/ID Filter Across All Views

Type `/` in the TUI to filter the container list by name or ID substring.
Filter applies across all views — only matching containers appear in PS,
stats, diff, etc. Makes the tool usable with dozens of containers.

Effort: medium (input mode, incremental filter, clear on esc).
Tests: model update tests for filter state.

### 3. Health And Problem Summary

Show problems before details.

- Parse container health status from inspect data.
- Highlight unhealthy, restarting, exited, and stale containers in PS/stats.
- Add a compact summary line or panel, e.g. `prod-api: 2 unhealthy, 1
  restarting; homebox: reconnecting`.
- Keep detailed logs/inspect/stats one keypress away.

Effort: small to medium (status extraction and summary rendering).
Tests: mock inspect/status data for unhealthy/restarting/stale cases.

### 4. Event Stream View

Effort: medium (new runtime method + streaming view).
Tests: mock server emits timed events.

### 5. CLI `config add <host>` Shortcut

`ctrwatch config add <host> [--socket <path>] [--tag <tag>]` — CLI-only
shortcut to add a server without the interactive init wizard.

Effort: small.

### 6. Podman Manual Confidence Pass

Run and record a real Podman session before adding Podman connection discovery.

- Verify rootless socket: `/run/user/$UID/podman/podman.sock`.
- Verify system socket when available: `/run/podman/podman.sock`.
- Verify configured Podman sockets, direct CLI commands, and default TUI.
- Verify a config with Docker and Podman sockets at the same time.
- Verify remote Podman over SSH with a configured `socket` path.

Effort: small (manual runbook and fixes for anything found).
Tests: `CTRWATCH_INTEGRATION=1 ./test/e2e/run-real.sh --runtime podman`.

### 7. Table column sorting in PS view

Click or key-triggered sort by name, status, CPU, memory, etc. in the TUI
PS view.

Effort: medium (sort state, key bindings).
Tests: sort order unit tests.

## Future Access And Runtime Sources

Add new sources only when they preserve the core shape: no agent, read-only by
default, and reuse credentials/config the user already has.

Priority order:

1. **Docker contexts**: discover or accept Docker contexts so users can reuse
   existing SSH/TCP/TLS Docker endpoints.
2. **Kubernetes contexts**: read pods, logs, status, restart counts, and events
   through kubeconfig-compatible auth. Keep this as pod/container triage, not a
   full Kubernetes dashboard.
3. **Podman connections / podman machine**: Podman is already supported through
   Docker-compatible sockets; add discovery for configured Podman connections
   and common `podman machine` sockets.
4. **Docker TCP/TLS endpoints**: extend existing TCP support with explicit TLS
   certificate config only when there is real demand.

## Not planned

- **Lifecycle management** (`start`, `stop`, `restart`, `compose up/down`).
  Docker Compose and `docker` CLI already do this. ctrwatch stays read-only
  unless explicit user demand proves a narrow, opt-in action mode is worth the
  extra risk.
- **Direct cloud-provider integrations** (AWS/Azure/GCP APIs). Prefer Docker
  contexts, kubeconfig, or existing runtime sockets over cloud-specific SDKs.
- **Non-container monitoring** (tmux sessions, cron jobs, systemd units).
  Out of scope. Use dedicated tools for those domains.
- **Configurable refresh intervals**. Hard-coded 10s is fine; YAGNI until
  someone asks.

## Acceptance Criteria

- `go test ./...` passes.
- `./test/e2e/run.sh` passes (zero runtime dependencies).
- `CTRWATCH_INTEGRATION=1 ./test/e2e/run-real.sh` passes with Docker or Podman.
- Normal tests do not require Docker, Podman, SSH, or a real terminal.
