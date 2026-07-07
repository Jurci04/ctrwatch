# ctrwatch

Small Go CLI/TUI for watching local containers through the Docker API (compatible with Podman, Docker, and any Docker API runtime).

## Features

- List running containers (`ps`)
- Stream logs from one or more containers with `--tail` and `--since` (`logs`)
- Split-screen TUI log view with live CPU/memory stats (`watch`)
- Container metadata inspection (`inspect`)
- One-shot CPU/memory stats (`stats`)

## Usage

```bash
ctrwatch ps
ctrwatch ps --all
ctrwatch logs [--tail N] [--since DURATION] api worker db
ctrwatch watch [--tail N] [--since DURATION] api worker db
ctrwatch inspect api-1
ctrwatch stats api worker db
```

Containers can specify a socket with `name@socket_path`:

```bash
ctrwatch logs api-1@/run/podman/podman.sock worker-1
ctrwatch watch api-1@/var/run/docker.sock worker-1@/run/podman/podman.sock
```

Socket resolution order: `name@socket` in arg → `DOCKER_HOST` env → auto-detect.

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

## Build from source

```bash
go build -o bin/ctrwatch .
```

## Requirements

- Linux
- Docker or Podman daemon running
- Access to a container runtime socket
