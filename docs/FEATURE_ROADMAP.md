# ctrwatch Feature Roadmap

## Summary

Make `ctrwatch` TUI-first without removing the direct CLI commands. Running
`ctrwatch` should open an interactive container dashboard by default, while
`ps`, `logs`, `watch`, `inspect`, `stats`, `import`, and `config check` stay
available for scripts and quick terminal use.

The TUI should use containers from `$CTRWATCH_CONFIG`, `ctrwatch.yaml`, or
`settings.yaml`, then let users navigate containers and open the same data now
available through the direct commands.

## Phase 1: Default TUI

Goal: make the app feel interactive by default while keeping every existing
command stable.

- Change `main.go` so `ctrwatch` with no args calls the same path as
  `ctrwatch watch`.
- Keep `ctrwatch help` as the way to print usage.
- Keep `ctrwatch ps`, `logs`, `watch`, `inspect`, `stats`, `import`, and
  `config check` working exactly as explicit commands.
- Add a small command entrypoint such as `RunDefaultTUI()` if calling
  `RunWatch(nil)` would make config/default behavior unclear.
- Load containers from config by default. Use the first configured tag or a
  simple default tag such as `dev`; if that is too surprising, start with all
  local configured containers.
- Preserve explicit `@tag` support so `ctrwatch watch @prod` and later
  `ctrwatch @prod` can open a scoped dashboard.
- Show plain TUI empty states for:
  - no config file found
  - config exists but no matching containers
  - runtime/socket connection error

Tests:

- Add a small test around the default container resolution helper, not around
  `main` or `os.Exit`.
- Keep command dispatch manual unless `main` is refactored to return an exit
  code.
- Run `go test ./...`.

## Phase 2: TUI Navigation And Views

Goal: make the TUI a dashboard, not just a split log viewer.

- Keep `internal/tui.Model` as the owner of navigation state.
- Add a selected container index and selected view. Suggested views:
  - `logs`
  - `ps`
  - `inspect`
  - `stats`
- Use a left container list when width allows; fall back to a compact header on
  narrow terminals.
- Keyboard behavior:
  - `up` / `down`: move selected container
  - `left` / `right`: switch view
  - `tab` / `shift+tab`: switch view
  - `enter`: focus/unfocus current view
  - `esc`: leave focus or return to the overview
  - `q` / `ctrl+c`: quit
- Logs view can start with the current behavior: buffered log lines per
  container, sanitized and colorized.
- Ps view should show the selected container plus nearby configured containers
  in a compact table: name, short ID if known, image, state, status, ports.
- Inspect view should show the selected container's image, status, created time,
  restart count, ports, mounts, labels, and env count. Do not dump huge env
  values by default.
- Stats view should show CPU, memory usage, memory limit, and last refresh/error.
- Add loading and error rows per container instead of failing the whole TUI.

Tests:

- Add `Model.Update` tests for arrow navigation and view switching.
- Add `View()` tests for no overlap, stable height, selected container marker,
  and focused view behavior.
- Keep tests string-based; do not require a real terminal.

## Phase 3: Shared Data Paths

Goal: avoid copying command logic into the TUI.

- Keep `internal/runtime` as the low-level Docker-compatible API client.
- Keep `internal/commands` responsible for flags, args, stdout/stderr, and SSH
  cleanup.
- Let the TUI receive data structs, not formatted command output.
- Extract small helpers only when both CLI and TUI need them:
  - container resolution from args/config
  - list/filter containers
  - inspect one container
  - stats for one container
  - log streaming options
- Do not shell out from the TUI to run `ctrwatch ps`, `inspect`, or `stats`.
- Prefer tiny structs over a broad service abstraction until a second real
  caller needs one.
- Keep formatting split:
  - CLI formats terminal text tables.
  - TUI formats panels and rows.

Implementation shape:

- First, make `watch` collect list/inspect/stats data in goroutines and send
  typed messages into the model.
- Then reuse the same helpers from CLI commands if duplication appears.
- Do not introduce a runtime interface just for tests; fake `http.RoundTripper`
  already covers runtime unit tests.

Tests:

- Keep runtime API tests on fake HTTP responses.
- Add command-helper tests only for behavior shared by CLI and TUI.
- Avoid subprocess tests for command output unless formatting becomes important.

## Phase 4: Runtime Confidence

Goal: make Podman a verified target without making normal tests flaky.

- Keep Docker-compatible Engine API support as the current core.
- Add opt-in Podman integration tests behind `CTRWATCH_INTEGRATION=1`.
- Detect common Podman sockets:
  - `/run/podman/podman.sock`
  - `/run/user/$UID/podman/podman.sock`
- Skip integration tests when the env var is missing or no socket exists.
- Test only stable behavior:
  - client can list containers
  - inspect works for a known container when a test container name is provided
  - stats returns a response or a clear runtime error
- Do not start or stop containers from tests at first. That is useful later, but
  it makes the first Podman check too invasive.
- Add README notes for verified Docker/Podman behavior and best-effort
  Docker-compatible runtimes.

Suggested env vars:

- `CTRWATCH_INTEGRATION=1`
- `CTRWATCH_PODMAN_SOCKET=/run/user/1000/podman/podman.sock`
- `CTRWATCH_TEST_CONTAINER=name`

Tests:

- Normal: `go test ./...`
- Integration: `CTRWATCH_INTEGRATION=1 go test ./internal/runtime -run Podman`

## Phase 5: More Runtime Backends

Goal: support more runtimes without abstracting too early.

- Do not add a runtime provider interface until the first non-Docker-compatible
  backend is actually being implemented.
- Keep Docker and Podman on the current Docker-compatible client.
- When adding a non-compatible backend, introduce the smallest provider shape
  the TUI needs:
  - list containers
  - stream logs
  - inspect container
  - get stats when supported
- Start with read-only support. Avoid start/stop/restart actions until the TUI
  read path is solid.
- Recommended order:
  1. containerd / nerdctl, because it is closest to current container workflows
  2. Kubernetes contexts and namespaces
  3. LXC / LXD
  4. Nomad
- Keep unsupported runtimes out of config examples until there is working code
  and at least one test path.

Tests:

- Unit-test each provider with fake clients or sample payloads.
- Add integration tests only behind explicit env vars.
- Keep the default test suite daemon-free.

## Cleanup Backlog

- Keep tests small, package-local, and deterministic.
- Avoid subprocess tests for simple command wrappers.
- Avoid daemon, SSH, or terminal requirements in normal unit tests.
- Add focused TUI tests for navigation, view switching, selected container
  behavior, and empty states.
- Review `go.mod` so directly imported packages are declared intentionally.
- Remove duplicated formatting only when a TUI feature needs shared data.
- Consider splitting `internal/tui/model.go` after Phase 2 if it becomes hard to
  scan:
  - `model.go` for state and update logic
  - `view.go` for rendering
  - `messages.go` for TUI message types
- Keep this split optional; do it only when the file is painful to edit.

## Acceptance Criteria

- `ctrwatch` opens the TUI by default.
- Existing direct commands still work.
- The TUI can navigate configured containers and show logs, ps/table, inspect,
  and stats without leaving the app.
- `go test ./...` passes.
- Podman real-runtime checks are opt-in and skipped by default.
- Normal tests do not require Docker, Podman, SSH, or a real terminal.
