package commands

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"ctrwatch/src/config"
)

func TestImportContainers(t *testing.T) {
	dir := t.TempDir()
	kubePath := filepath.Join(dir, "pod.yaml")
	if err := os.WriteFile(kubePath, []byte(`
apiVersion: v1
kind: Pod
spec:
  containers:
    - name: web
    - name: worker
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := map[string][]byte{
		"compose.yaml": []byte(`
name: shop
services:
  api:
    container_name: api-custom
  db:
    image: postgres
`),
		"api.container": []byte("ContainerName=api\nImage=example/api\n"),
		"api.kube":      []byte("Yaml=pod.yaml\n"),
	}

	want := map[string][]string{
		"compose.yaml":  {"api-custom", "shop-db-1"},
		"api.container": {"api"},
		"api.kube":      {"web", "worker"},
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, name)
			if err := os.WriteFile(path, body, 0o644); err != nil {
				t.Fatal(err)
			}
			got, err := importContainers(path)
			if err != nil {
				t.Fatal(err)
			}
			slices.Sort(got)
			if !slices.Equal(got, want[name]) {
				t.Fatalf("containers = %v, want %v", got, want[name])
			}
		})
	}
}

func TestResolveImportPathPrefersComposeFiles(t *testing.T) {
	tests := []struct {
		name string
		file string
	}{
		{name: "compose yaml", file: "compose.yaml"},
		{name: "compose yml", file: "compose.yml"},
		{name: "docker compose yaml", file: "docker-compose.yaml"},
		{name: "docker compose yml", file: "docker-compose.yml"},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			for _, later := range tests[i:] {
				if err := os.WriteFile(later.file, []byte("services: {}"), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			got, err := resolveImportPath("")
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.file {
				t.Fatalf("path = %q, want %q", got, tt.file)
			}
		})
	}
}

func TestRunImportWritesConfig(t *testing.T) {
	dir := t.TempDir()
	composePath := filepath.Join(dir, "compose.yaml")
	configPath := filepath.Join(dir, "ctrwatch.yaml")
	if err := os.WriteFile(composePath, []byte(`
services:
  web:
    container_name: web
`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := RunImport([]string{"--output", configPath, "--tag", "dev", composePath}); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Servers) != 1 {
		t.Fatalf("servers = %d, want 1", len(cfg.Servers))
	}
	server := cfg.Servers[0]
	if server.Host != "localhost" {
		t.Fatalf("host = %q, want localhost", server.Host)
	}
	if !slices.Equal(server.Containers, []string{"web"}) {
		t.Fatalf("containers = %v, want [web]", server.Containers)
	}
	if !slices.Equal(server.Tags, []string{"dev"}) {
		t.Fatalf("tags = %v, want [dev]", server.Tags)
	}
}
