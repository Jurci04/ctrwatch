# ctrwatch

Small Go CLI/TUI for watching local containers through the Docker API (compatible with Podman, Docker, and any Docker API runtime).

## Features

- List running containers (`ps`)
- Stream logs from one or more containers with `--tail` and `--since` (`logs`)
- Split-screen TUI log view with live CPU/memory stats (`watch`)
- Container metadata inspection (`inspect`)
- One-shot CPU/memory stats (`stats`)
- Import local container groups from Compose, Podman Quadlet, or running containers (`import`)
- Tagged local/remote container groups from `ctrwatch.yaml`, `settings.yaml`, or `$CTRWATCH_CONFIG`

## Usage

```bash
ctrwatch ps
ctrwatch ps --all
ctrwatch ps @dev
ctrwatch logs [--tail N] [--since DURATION] api worker db
ctrwatch watch [--tail N] [--since DURATION] api worker db
ctrwatch inspect api-1
ctrwatch stats api worker db
ctrwatch import docker-compose.yml
ctrwatch import --from-running --tag dev
ctrwatch config check
```

## Watch TUI

```bash
ctrwatch watch @prod-workers
```

Keys:

- `tab` / arrows: select container
- `enter` / space: focus selected container
- `a`: show all containers
- `q` / `esc` / `ctrl+c`: quit

Containers can specify a socket with `name@socket_path`:

```bash
ctrwatch logs api-1@/run/podman/podman.sock worker-1
ctrwatch logs api-1@unix:///run/podman/podman.sock
ctrwatch watch api-1@/var/run/docker.sock worker-1@/run/podman/podman.sock
```

Socket resolution order: `name@socket` in arg → `DOCKER_HOST` env → auto-detect.

## Config

Tagged containers are loaded from `$CTRWATCH_CONFIG`, `ctrwatch.yaml`, or
`settings.yaml`:

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

`host: localhost`, `host: 127.0.0.1`, or omitted `host` means local runtime.
Remote hosts use SSH and the configured runtime socket.

## Import

Import a Docker Compose file or Podman Quadlet file into the config:

```bash
ctrwatch import compose.yaml --tag dev
ctrwatch import api.container --tag dev
ctrwatch import app.kube --tag dev
ctrwatch import --from-running --tag dev
```

Compose import uses `container_name` when present. Otherwise ctrwatch derives
the usual Compose name (`project-service-1`) and prints a warning.

Quadlet import supports `.container`, `.pod`, and `.kube` files. For `.kube`
files, ctrwatch reads `Yaml=...` and imports container names from that pod YAML.

## TODO

- Kubernetes contexts/namespaces through the Kubernetes API
- Nomad job files
- systemd services that run Docker/Podman manually
- containerd/nerdctl containers
- LXC/LXD containers
- remote import over SSH
- Docker Swarm stack files
- Helm rendered manifests

## Build

```bash
go build -o bin/ctrwatch .
```

## Quick install on a server

```bash
curl -sfL https://raw.githubusercontent.com/jurci/ctrwatch/main/install.sh | sh
```

Or download the binary from [GitHub Releases](https://github.com/jurci/ctrwatch/releases/latest)
and place it in your `PATH`.

## Requirements

- Linux
- Docker or Podman daemon running
- Access to a container runtime socket
