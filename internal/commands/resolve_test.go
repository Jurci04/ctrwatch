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
