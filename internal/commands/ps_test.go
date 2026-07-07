package commands

import (
	"slices"
	"strings"
	"testing"

	"ctrwatch/internal/runtime"
)

func TestShortID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want string
	}{
		{name: "long", id: "123456789012345", want: "123456789012"},
		{name: "short", id: "123", want: "123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shortID(tt.id); got != tt.want {
				t.Fatalf("shortID = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseContainersDeduplicatesSocketClients(t *testing.T) {
	defs := parseContainers([]string{"api@/tmp/podman.sock", "worker@/tmp/podman.sock"})

	if len(defs) != 2 {
		t.Fatalf("defs = %d, want 2", len(defs))
	}
	if defs[0].Name != "api" || defs[1].Name != "worker" {
		t.Fatalf("defs = %#v", defs)
	}
	if defs[0].Client != defs[1].Client {
		t.Fatal("expected shared client for same socket")
	}
}

func TestFilterContainersMatchesDockerNames(t *testing.T) {
	containers := []runtime.Container{
		{ID: "1", Names: []string{"/api"}},
		{ID: "2", Names: []string{"/db"}},
	}

	got := filterContainers(containers, []string{"db"})
	if len(got) != 1 || got[0].ID != "2" {
		t.Fatalf("filtered = %#v", got)
	}
}

func TestFormatPorts(t *testing.T) {
	got := formatPorts([]runtime.Port{
		{IP: "127.0.0.1", PublicPort: 8080, PrivatePort: 80, Type: "tcp"},
		{PrivatePort: 5432, Type: "tcp"},
	})
	want := []string{"127.0.0.1:8080->80/tcp", "5432/tcp"}
	if !slices.Equal(strings.Split(got, ", "), want) {
		t.Fatalf("ports = %q", got)
	}
}
