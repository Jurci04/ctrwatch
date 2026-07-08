package runtime

import "testing"

func TestNewClientForSocketAcceptsUnixPrefix(t *testing.T) {
	client := NewClientForSocket("unix:///run/podman/podman.sock")
	if client.SocketPath != "unix:///run/podman/podman.sock" {
		t.Fatalf("SocketPath = %q", client.SocketPath)
	}
}
