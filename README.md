# ctrwatch

[![CI](https://github.com/Jurci04/ctrwatch/actions/workflows/ci.yml/badge.svg)](https://github.com/Jurci04/ctrwatch/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/Jurci04/ctrwatch)](https://github.com/Jurci04/ctrwatch/releases/latest)
![Go Version](https://img.shields.io/github/go-mod/go-version/Jurci04/ctrwatch)

`ctrwatch` is a small Go TUI for watching containers across local and
SSH-reachable servers. It works with Docker, Podman, and other
Docker-compatible runtime sockets.

Run `ctrwatch` with no arguments to open the TUI. Without a config file it
auto-detects a local Docker or Podman socket. With `ctrwatch.yaml`, it uses the
configured sockets so Docker, Podman, and remote hosts can be watched together.
You can browse logs, inspect metadata, view stats, diff filesystem changes, see
running processes, and connect to remote servers via SSH.

It is intentionally agentless and read-only: no remote daemon to install, no
database to run, and no container lifecycle actions by default.

## Why ctrwatch

Use `ctrwatch` when you want one terminal view across several small servers
without installing anything on those servers beyond the runtime you already
use.

| Tool | Best fit |
|------|----------|
| ctrwatch | Agentless, read-only multi-server triage over SSH |
| lazydocker | Full single-host Docker/Compose management |
| ctop | Single-host container metrics |
| tori-cli | SSH-first monitoring with agent/history/alerts |
| dtop | Multi-host metrics dashboard |

## Install

Install the latest Linux release:

```bash
curl -sfL https://raw.githubusercontent.com/Jurci04/ctrwatch/main/install.sh | sh
```

Or download a binary from
[GitHub Releases](https://github.com/Jurci04/ctrwatch/releases/latest) and place
it in your `PATH`.

Build from source:

```bash
go build -o bin/ctrwatch .
```

## Requirements

- Linux
- Docker, Podman, or another Docker-compatible runtime socket
- Access to the runtime socket
- `ssh` for remote hosts

ctrwatch talks to runtimes through Docker-compatible API sockets. Docker usually
starts its socket with the daemon. For rootless Podman, enable the user socket:

```bash
systemctl --user enable --now podman.socket
ls -l /run/user/$(id -u)/podman/podman.sock
```

## Quick Start

```bash
ctrwatch
```

Opens the TUI. Use arrow keys to navigate, `enter` to focus a container, and
`←`/`→` to switch between views. Container rows show their runtime source, such
as `docker`, `podman`, or `tcp`.

CLI commands are also available for scripting:

```bash
ctrwatch ps
ctrwatch logs --tail 200 api
ctrwatch inspect api
ctrwatch stats api worker
ctrwatch config check
```

## TUI

Seven views, switchable with `←`/`→`:

| View | Description |
|------|-------------|
| Logs | Live log output, split-screen for focused container |
| PS | Container list with name, ID, image, state, status |
| Inspect | Container metadata — overview, mounts, config, ports |
| Stats | CPU/memory per container with sparkline history |
| Diff | Filesystem changes since container start |
| Top | Running processes inside container |
| Servers | Browse and connect to remote servers from config |

Keys:

- `↑`/`↓`: select container
- `←`/`→`: switch view
- `enter`: focus/unfocus selected container
- `esc`: unfocus / clear filter
- `s`: jump to servers view
- `d`: toggle container log panel
- `q`: quit

## Config

Tagged containers are loaded from `$CTRWATCH_CONFIG`, `ctrwatch.yaml`, or
`settings.yaml`.

First run:

```bash
ctrwatch config init
```

Or open `ctrwatch`, switch to Servers, and press `i` to create `ctrwatch.yaml`
from the TUI. Press `ctrl+a` in the TUI setup form to load SSH aliases from
`~/.ssh/config`.

For SSH troubleshooting, run with `CTRWATCH_DEBUG=1`; debug logs are appended to
`./logs/app.log`.

```yaml
servers:
  - host: localhost
    socket: /var/run/docker.sock
    containers:
      - api
      - worker
    tags: [dev]

  - host: localhost
    socket: /run/user/1000/podman/podman.sock
    containers:
      - podman-api
    tags: [dev]

  - host: user@example.com
    socket: /run/podman/podman.sock
    containers:
      - web
      - jobs
    tags: [prod]
```

`host: localhost`, `host: 127.0.0.1`, or an omitted `host` means local runtime.
Remote hosts use SSH and the configured runtime socket.

Local config entries connect automatically on startup. Remote entries are listed
in the Servers view and connect when selected.

`host` can be any normal SSH target, including aliases from `~/.ssh/config`.
That means `User`, `IdentityFile`, `ProxyJump`, agent auth, and other SSH
client settings stay in your SSH config instead of `ctrwatch.yaml`:

```sshconfig
Host prod-api
  HostName 203.0.113.10
  User deploy
  IdentityFile ~/.ssh/prod_ed25519
  ProxyJump bastion
```

```yaml
servers:
  - host: prod-api
    socket: /var/run/docker.sock
    containers:
      - api
      - worker
    tags: [prod]
```

Containers can also specify a socket directly:

```bash
ctrwatch logs api@/run/podman/podman.sock worker
ctrwatch logs api@unix:///run/podman/podman.sock
ctrwatch stats api@/var/run/docker.sock worker@/run/podman/podman.sock
```

Socket behavior:

- The TUI always loads existing local default sockets (Docker/Podman) unless
  that exact local socket is already configured.
- A config entry with no `socket` probes default Docker/Podman socket paths and
  uses only sockets where the configured containers are found.
- A config entry with `socket` uses only that socket.

Container argument socket resolution order:

1. `name@socket` in the container argument
2. configured `socket` in `ctrwatch.yaml`
3. auto-detected local Docker or Podman socket

## Import

Import a Docker Compose file or Podman Quadlet file:

```bash
ctrwatch import compose.yaml --tag dev
ctrwatch import api.container --tag dev
ctrwatch import app.kube --tag dev
```

Import currently running local containers:

```bash
ctrwatch import --from-running --tag dev
```

Compose import uses `container_name` when present. Otherwise `ctrwatch` derives
the usual Compose name, such as `project-service-1`, and prints a warning.

Quadlet import supports `.container`, `.pod`, and `.kube` files. For `.kube`
files, `ctrwatch` reads `Yaml=...` and imports container names from that pod
YAML.

## Runtime Support

Current support is centered on Docker-compatible Engine API sockets:

- Docker: expected primary target
- Podman: supported through its Docker-compatible socket
- Other Docker-compatible runtimes: best effort

### Podman

Local Podman works when its Docker-compatible socket is available. Common
socket paths:

- rootless: `/run/user/$UID/podman/podman.sock`
- system: `/run/podman/podman.sock`

Examples:

```bash
ctrwatch ps @podman
ctrwatch stats @podman
ctrwatch logs api@/run/user/$(id -u)/podman/podman.sock
```

Remote Podman works the same way as Docker: connect over SSH and point
`socket` at the remote Podman socket:

```yaml
servers:
  - host: prod-api
    socket: /var/run/docker.sock
    containers:
      - web
    tags: [prod]

  - host: prod-api
    socket: /run/user/1000/podman/podman.sock
    containers:
      - api
      - worker
    tags: [prod]
```

Runtime selection is explicit so one shell environment cannot hide Docker or
Podman containers from the other runtime. Use `ctrwatch.yaml` for normal setup,
or `name@socket` for a one-off CLI override.

Future runtime work is tracked in
[docs/FEATURE_ROADMAP.md](docs/FEATURE_ROADMAP.md). The preferred expansion
path is to reuse existing user configuration: Docker contexts, Kubernetes
contexts, Podman connections / `podman machine`, and Docker TCP/TLS endpoints.
Direct AWS/Azure/GCP integrations are not planned for now.

## Development

Run tests:

```bash
go test ./...
```

If your environment has a read-only Go build cache, set a writable cache:

```bash
GOCACHE=/tmp/ctrwatch-go-cache go test ./...
```

Check coverage:

```bash
go test ./... -cover
```

E2E tests (no runtime dependencies):

```bash
./test/e2e/run.sh
```

Normal tests should stay daemon-free, SSH-free, fast, and deterministic.

## Contributing

Issues and pull requests are welcome. Please keep changes focused, update docs
when behavior changes, and use conventional-style commit subjects such as:

```text
feat(tui): add inspect view
fix(runtime): handle empty stats response
docs(readme): document Podman socket setup
test(config): cover tag resolution
chore(deps): update Go module metadata
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full contribution guide.

## Roadmap

See [docs/FEATURE_ROADMAP.md](docs/FEATURE_ROADMAP.md) for the full
implementation plan. The main direction is:

- keep `ctrwatch` agentless, read-only, and SSH-first
- make remote server state and reconnect behavior reliable
- add filter across all views
- surface health/problem summaries before detailed drill-down
- make first-run config setup easier
- add more runtime backends only when there is a concrete implementation path
