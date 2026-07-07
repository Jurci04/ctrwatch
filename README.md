# ctrwatch

`ctrwatch` is a small Go CLI/TUI for watching containers.
It works with Docker, Podman, and other Docker-compatible runtime sockets.

Use it for quick container lists, streaming logs, split-screen log watching,
metadata inspection, one-shot stats, and tagged groups of local or remote
containers.

## Install

Install the latest Linux release:

```bash
curl -sfL https://raw.githubusercontent.com/jurci/ctrwatch/main/install.sh | sh
```

Or download a binary from
[GitHub Releases](https://github.com/jurci/ctrwatch/releases/latest) and place
it in your `PATH`.

Build from source:

```bash
go build -o bin/ctrwatch .
```

## Requirements

- Linux
- Docker, Podman, or another Docker-compatible runtime socket (for now, support for others will be added in the future)
- Access to the runtime socket
- `ssh` for remote hosts

## Quick Start

```bash
ctrwatch ps
ctrwatch watch api worker
ctrwatch logs --tail 200 api
ctrwatch inspect api
ctrwatch stats api worker
```

Use a configured tag:

```bash
ctrwatch ps @dev
ctrwatch watch @prod
```

Import local container names into config:

```bash
ctrwatch import compose.yaml --tag dev
ctrwatch import --from-running --tag dev
```

## Commands

```text
ctrwatch ps [--all] [@tag]
ctrwatch logs [--tail N] [--since DURATION] <container>... | @tag
ctrwatch watch [--tail N] [--since DURATION] <container>... | @tag
ctrwatch inspect <container> | @tag
ctrwatch stats <container>... | @tag
ctrwatch import [--tag TAG] [file]
ctrwatch import --from-running
ctrwatch config check
ctrwatch help
```

## Watch TUI

`watch` opens a terminal UI for live logs and CPU/memory stats:

```bash
ctrwatch watch @prod-workers
```

Keys:

- `tab` / arrows: select container
- `enter` / space: focus selected container
- `a`: show all containers
- `q` / `esc` / `ctrl+c`: quit

The planned next step is a TUI-first dashboard where running `ctrwatch` opens
the TUI by default and the TUI can navigate logs, ps/table, inspect, and stats
views. See [docs/FEATURE_ROADMAP.md](docs/FEATURE_ROADMAP.md).

## Config

Tagged containers are loaded from `$CTRWATCH_CONFIG`, `ctrwatch.yaml`, or
`settings.yaml`.

```yaml
servers:
  - host: localhost
    containers:
      - api
      - worker
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

Containers can also specify a socket directly:

```bash
ctrwatch logs api@/run/podman/podman.sock worker
ctrwatch logs api@unix:///run/podman/podman.sock
ctrwatch watch api@/var/run/docker.sock worker@/run/podman/podman.sock
```

Socket resolution order:

1. `name@socket` in the container argument
2. `DOCKER_HOST`
3. auto-detected Docker or Podman socket

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

Future runtime work is tracked in
[docs/FEATURE_ROADMAP.md](docs/FEATURE_ROADMAP.md), including opt-in Podman
integration tests and possible containerd, Kubernetes, LXC/LXD, and Nomad
support.

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

See [docs/FEATURE_ROADMAP.md](docs/FEATURE_ROADMAP.md) for the implementation
plan. The main direction is:

- make the TUI the default interactive entrypoint
- keep direct CLI commands for scripting
- add TUI views for logs, ps/table, inspect, and stats
- verify Podman with opt-in integration tests
- add more runtime backends only when there is a concrete implementation path
