# ctrwatch Feature Roadmap

## Summary

`ctrwatch` is a container monitor — it watches containers, not processes or
sessions. The TUI is the default entrypoint, direct CLI commands stay available
for scripting.

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
- Mock E2E tests run against both TCP and Unix socket transports (32+ tests).
- Real-container integration tests (`CTRWATCH_INTEGRATION=1`) work with Docker
  and Podman.
- `DOCKER_HOST` environment variable fully supported for any Docker-compatible
  runtime.

### Phase 5: Views Expansion

- **Container diff / changes view**: filesystem changes since container start.
- **Container top / processes view**: running processes inside a container.
- **Historical stats sparkline**: last N CPU samples rendered as ASCII sparkline
  in the stats view.

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
- Consistent bordered panel style for all views.

## Next

Ranked by expected value-to-effort ratio.

### 1. Thorough Testing

Run a full TUI session with real containers to verify all views render
correctly. Check edge cases: empty states, SSH disconnect, resize, rapid
navigation.

### 2. Container Name/ID Filter Across All Views

Type `/` in the TUI to filter the container list by name or ID substring.
Filter applies across all views — only matching containers appear in PS,
stats, diff, etc. Makes the tool usable with dozens of containers.

Effort: medium (input mode, incremental filter, clear on esc).
Tests: model update tests for filter state.

### 3. Event Stream View

Subscribe to the Docker Events API (`GET /events`) and show a rolling feed
of container lifecycle events (start, stop, die, health_status, etc.).

Effort: medium (new runtime method + streaming view).
Tests: mock server emits timed events.

### 4. Export inspect / stats as JSON

Add `--json` flag to `inspect` and `stats` commands for pipeable machine-readable
output. Useful for feeding into `jq` or other tools.

Effort: trivial (conditional JSON encoding).
Tests: capture and validate JSON output.

### 5. Multiple socket/config profiles

Support `--config <path>` flag so users can quickly switch between projects
or environments without editing `ctrwatch.yaml`.

Effort: small (add flag, pass through config path).
Tests: test flag overrides env var.

### 6. Health check view

Show container health status from inspect (when `Healthcheck` is configured).
Highlight unhealthy containers in the PS view and TUI.

Effort: small (parse health field, add color).
Tests: mock inspect includes health data.

### 7. Table column sorting in PS view

Click or key-triggered sort by name, status, CPU, memory, etc. in the TUI
PS view.

Effort: medium (sort state, key bindings).
Tests: sort order unit tests.

## Not planned

- **Lifecycle management** (`start`, `stop`, `restart`, `compose up/down`).
  Docker Compose and `docker` CLI already do this. ctrwatch stays read-only.
- **Non-container monitoring** (tmux sessions, cron jobs, systemd units).
  Out of scope. Use dedicated tools for those domains.
- **Configurable refresh intervals**. Hard-coded 10s is fine; YAGNI until
  someone asks.

## Acceptance Criteria

- `go test ./...` passes.
- `./test/e2e/run.sh` passes (zero runtime dependencies).
- `CTRWATCH_INTEGRATION=1 ./test/e2e/run-real.sh` passes with Docker or Podman.
- Normal tests do not require Docker, Podman, SSH, or a real terminal.
