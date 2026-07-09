package commands

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"ctrwatch/src/config"
)

func TestRunConfigInitCreatesConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ctrwatch.yaml")
	input := strings.NewReader("n\nprod-api\n/run/podman/podman.sock\napi, worker\ndev, prod\n")
	var output strings.Builder

	if err := runConfigInit([]string{"--output", path}, input, &output); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.Servers[0]
	if got.Host != "prod-api" || got.Socket != "/run/podman/podman.sock" {
		t.Fatalf("server = %+v", got)
	}
	if !slices.Equal(got.Containers, []string{"api", "worker"}) {
		t.Fatalf("containers = %v", got.Containers)
	}
	if !slices.Equal(got.Tags, []string{"dev", "prod"}) {
		t.Fatalf("tags = %v", got.Tags)
	}
}
