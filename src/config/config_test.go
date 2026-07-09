package config

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestLoadPreservesBlankSocketAndValidates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ctrwatch.yaml")
	if err := os.WriteFile(path, []byte(`
servers:
  - host: localhost
    containers: [api]
    tags: [dev]
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	server := cfg.Servers[0]
	if server.Socket != "" {
		t.Fatalf("socket = %q", server.Socket)
	}
	if !IsLocalHost(server.Host) {
		t.Fatalf("host %q should be local", server.Host)
	}
}

func TestMergeServerReplacesLocalAndDedupesContainers(t *testing.T) {
	cfg := &Config{Servers: []Server{{
		Containers: []string{"old"},
		Tags:       []string{"dev"},
	}}}

	MergeServer(cfg, Server{
		Host:       "127.0.0.1",
		Containers: []string{"worker", "api", "api"},
		Tags:       []string{"dev"},
	})

	if len(cfg.Servers) != 1 {
		t.Fatalf("servers = %d, want 1", len(cfg.Servers))
	}
	server := cfg.Servers[0]
	if server.Host != "localhost" {
		t.Fatalf("host = %q, want localhost", server.Host)
	}
	if !slices.Equal(server.Containers, []string{"api", "worker"}) {
		t.Fatalf("containers = %v", server.Containers)
	}
}

func TestConfigPathPrefersEnvThenExistingFiles(t *testing.T) {
	t.Setenv("CTRWATCH_CONFIG", "/tmp/custom.yaml")
	if got := ConfigPath(); got != "/tmp/custom.yaml" {
		t.Fatalf("ConfigPath = %q", got)
	}

	t.Setenv("CTRWATCH_CONFIG", "")
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("settings.yaml", []byte("servers: []"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ConfigPath(); got != "settings.yaml" {
		t.Fatalf("ConfigPath = %q, want settings.yaml", got)
	}
}

func TestSSHConfigHostsFrom(t *testing.T) {
	got := SSHConfigHostsFrom(`
Host prod-api *.internal prod-api
  HostName 203.0.113.10
Host staging bastion?
`)
	want := []string{"prod-api", "staging"}
	if !slices.Equal(got, want) {
		t.Fatalf("hosts = %v, want %v", got, want)
	}
}
