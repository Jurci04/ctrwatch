package commands

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveTaggedLocalConfig(t *testing.T) {
	dir := t.TempDir()
	socket := runtimeSocket(t, dir, "runtime.sock", "api", "worker")
	path := filepath.Join(dir, "ctrwatch.yaml")
	if err := os.WriteFile(path, []byte(fmt.Sprintf(`
servers:
  - host: localhost
    socket: %s
    containers: [api, worker]
    tags: [dev]
`, socket)), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CTRWATCH_CONFIG", path)

	defs, cleanup, err := resolveTagged("dev")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if len(defs) < 2 || defs[0].Name != "api" || defs[1].Name != "worker" {
		t.Fatalf("defs = %#v", defs)
	}
	if defs[0].Client == nil {
		t.Fatal("client is nil")
	}
}

func TestResolveTaggedLocalConfigKeepsSeparateSockets(t *testing.T) {
	dir := t.TempDir()
	dockerSocket := runtimeSocket(t, dir, "docker.sock", "docker-api")
	podmanSocket := runtimeSocket(t, dir, "podman.sock", "podman-api")
	path := filepath.Join(dir, "ctrwatch.yaml")
	if err := os.WriteFile(path, []byte(fmt.Sprintf(`
servers:
  - host: localhost
    socket: %s
    containers: [docker-api]
    tags: [dev]
  - host: localhost
    socket: %s
    containers: [podman-api]
    tags: [dev]
`, dockerSocket, podmanSocket)), 0o644); err != nil {
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
	if defs[0].Client.SocketPath != "unix://"+dockerSocket {
		t.Fatalf("docker socket = %q", defs[0].Client.SocketPath)
	}
	if defs[1].Client.SocketPath != "unix://"+podmanSocket {
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

func runtimeSocket(t *testing.T, dir, name string, containers ...string) string {
	t.Helper()
	socket := filepath.Join(dir, name)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/containers/json", func(w http.ResponseWriter, r *http.Request) {
		var rows []string
		for _, name := range containers {
			rows = append(rows, fmt.Sprintf(`{"Names":["/%s"]}`, name))
		}
		_, _ = w.Write([]byte("[" + strings.Join(rows, ",") + "]"))
	})
	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		_ = server.Shutdown(context.Background())
		_ = listener.Close()
	})
	return socket
}
