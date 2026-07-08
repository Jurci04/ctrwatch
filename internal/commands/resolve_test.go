package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveTaggedLocalConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ctrwatch.yaml")
	if err := os.WriteFile(path, []byte(`
servers:
  - host: localhost
    containers: [api, worker]
    tags: [dev]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CTRWATCH_CONFIG", path)

	defs, cleanup, err := resolveTagged("dev")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if len(defs) != 2 || defs[0].Name != "api" || defs[1].Name != "worker" {
		t.Fatalf("defs = %#v", defs)
	}
	if defs[0].Client.SocketPath != "unix:///var/run/docker.sock" {
		t.Fatalf("socket = %q", defs[0].Client.SocketPath)
	}
}

func TestResolveTaggedLocalConfigKeepsSeparateSockets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ctrwatch.yaml")
	if err := os.WriteFile(path, []byte(`
servers:
  - host: localhost
    socket: /var/run/docker.sock
    containers: [docker-api]
    tags: [dev]
  - host: localhost
    socket: /run/user/1000/podman/podman.sock
    containers: [podman-api]
    tags: [dev]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CTRWATCH_CONFIG", path)

	defs, cleanup, err := resolveTagged("dev")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if len(defs) != 2 {
		t.Fatalf("defs = %#v", defs)
	}
	if defs[0].Client == defs[1].Client {
		t.Fatal("expected separate clients for separate local sockets")
	}
	if defs[0].Client.SocketPath != "unix:///var/run/docker.sock" {
		t.Fatalf("docker socket = %q", defs[0].Client.SocketPath)
	}
	if defs[1].Client.SocketPath != "unix:///run/user/1000/podman/podman.sock" {
		t.Fatalf("podman socket = %q", defs[1].Client.SocketPath)
	}
}

func TestResolveTaggedServersNoMatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ctrwatch.yaml")
	if err := os.WriteFile(path, []byte(`
servers:
  - containers: [api]
    tags: [dev]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CTRWATCH_CONFIG", path)

	if _, err := resolveTaggedServers("prod"); err == nil {
		t.Fatal("expected missing tag error")
	}
}
