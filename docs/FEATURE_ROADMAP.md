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

### Phase 1: Default TUI

- `ctrwatch` with no args opens the TUI.
- Loads configured containers from `ctrwatch.yaml` (first tag). Falls back to
  empty TUI with helpful message when no config exists.
- Auto-detects local Docker/Podman containers on startup.
- `ctrwatch help` still prints usage. All direct commands unchanged.

### Phase 2: TUI Navigation And Views

- Seven switchable views: logs, ps, inspect, stats, diff, top, servers.
- `←`/`→` switch views, `↑`/`↓` cycle containers, `enter` focuses, `esc` backs
  out. `s` jumps to servers view. `d` toggles container log panel.
- Context-aware footer shows relevant keybindings per view.
- All views share the same bordered-panel style (`╭─ title ─╮`) for visual
  consistency.
- View tests for PS table, inspect metadata, stats table, view switching, empty
  state, and focused/esc behavior.

### Phase 3: Shared Data Paths

- TUI model owns `[]*runtime.Client` and fetches data on demand via `tea.Cmd`.
- Log streaming and stats polling goroutines live in the model.
- Multiple servers supported simultaneously via SSH tunnels.

### Phase 4: Runtime Confidence

- Runtime client supports TCP (`tcp://`, `http://`) in addition to Unix sockets.
- Mock E2E tests run against both TCP and Unix socket transports (33 tests).
- Real-container integration tests (`CTRWATCH_INTEGRATION=1`) work with Docker
  and Podman.
- Runtime selection is explicit through config sockets or `name@socket`
  arguments.
- Configured local and remote servers can point at separate Docker and Podman
  sockets at the same time.
- TUI rows show the runtime source (`docker`, `podman`, `tcp`, or fallback
  `runtime`) so mixed container lists stay readable.
- Local config entries connect automatically on startup; remote entries remain
  explicit in the Servers view.
- Real-container E2E tests support `--runtime docker|podman` to avoid mixing
  runtimes when both are installed.

### Phase 5: Views Expansion

- **Container diff / changes view**: filesystem changes since container start.
- **Container top / processes view**: running processes inside a container.
- **Historical stats sparkline**: last N CPU samples rendered as ASCII sparkline
  in the stats view.
- **JSON output**: `inspect --json` and `stats --json` provide pipeable
  machine-readable output.

### Phase 6: Server Browser

- Config file servers listed in a dedicated view.
- Press enter to SSH-connect to a server and browse its containers.
- Multiple servers can be connected simultaneously.
- Local containers always auto-detected on startup.

### Phase 7: UI/UX Polish

- Keyword-only log coloring (not whole-line red).
- Unified `●` marker across all views.
- Removed `watch` command (redundant with default TUI).
- Simplified keybindings (removed ctrl+c, space, tab, a).
- Context-aware key hints in footer.

### Phase 8: Repo Hygiene

- Install instructions use the correct `Jurci04/ctrwatch` owner.
- README has CI, release, and Go Report Card badges.
- CI runs `gofmt`, `go vet`, `go test -race`, and `golangci-lint`.
- The mock E2E server binary is built from source and ignored instead of
  committed.
- Mock E2E TUI smoke test uses a PTY and exits cleanly after cleanup.

### SSH Lifecycle Ownership

- `ServerSession` is now the public SSH interface for connect, disconnect,
  state, socket, and last-error access.
- The tunnel implementation is private and owns bounded reconnect with the
  runtime ping probe.
- TUI and config resolution both use the session abstraction instead of
  calling the tunnel helper directly.
- Added focused SSH session/tunnel tests for core state and helper behavior.
- Container selection is clamped after connect/disconnect events so stale
  server selections cannot panic the TUI.

## Next

Ranked by expected value-to-effort ratio.

### 1. SSH Reliability And Server State

Make remote monitoring reliable enough to leave open all day.

- Track each server as `local`, `connected`, `connecting`, `reconnecting`, or
  `failed`.
- Surface the last SSH/runtime error in the servers view.
- Auto-reconnect dropped SSH tunnels with bounded exponential backoff.
- Keep stale container data visible while reconnecting, clearly marked stale.
- Document that `host:` can be an SSH config alias, including `User`,
  `IdentityFile`, `ProxyJump`, and agent-based auth.

Effort: medium (server state model, reconnect command, visible status).
Tests: model update tests for state transitions; mock failing reconnect path.

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

### 5. First-run Config Setup

When no config exists, help the user create one instead of only showing an
empty state.

- Add `ctrwatch config init` to write a minimal `ctrwatch.yaml`.
- Add `ctrwatch config add <host> [--socket <path>] [--tag <tag>]`.
- In the empty TUI, point to the exact command to add a local or SSH server.

Effort: small (config write helpers, command wiring).
Tests: temp-dir config write tests.

Effort: small (add flag, pass through config path).
Tests: test flag overrides env var.

### 7. Podman Manual Confidence Pass

Run and record a real Podman session before adding Podman connection discovery.

- Verify rootless socket: `/run/user/$UID/podman/podman.sock`.
- Verify system socket when available: `/run/podman/podman.sock`.
- Verify configured Podman sockets, direct CLI commands, and default TUI.
- Verify a config with Docker and Podman sockets at the same time.
- Verify remote Podman over SSH with a configured `socket` path.

Effort: small (manual runbook and fixes for anything found).
Tests: `CTRWATCH_INTEGRATION=1 ./test/e2e/run-real.sh --runtime podman`.

### 8. Table column sorting in PS view

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
