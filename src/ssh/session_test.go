package ssh

import (
	"testing"

	"ctrwatch/src/config"
)

func TestServerSessionStartsUnknown(t *testing.T) {
	session := NewServerSession(config.Server{Host: "localhost", Socket: "/run/user/1000/podman/podman.sock"})

	if got := session.Server().Host; got != "localhost" {
		t.Fatalf("Server().Host = %q, want %q", got, "localhost")
	}
	if got := session.State(); got != "unknown" {
		t.Fatalf("State() = %q, want %q", got, "unknown")
	}
	if got := session.Client(); got != nil {
		t.Fatalf("Client() = %#v, want nil", got)
	}
	if got := session.LastError(); got != nil {
		t.Fatalf("LastError() = %v, want nil", got)
	}
}

func TestServerSessionConnectLocalHostUsesConfiguredSocket(t *testing.T) {
	session := NewServerSession(config.Server{
		Host:   "localhost",
		Socket: "/run/user/1000/podman/podman.sock",
	})

	client, err := session.Connect()
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if client == nil {
		t.Fatal("Connect() returned nil client")
	}
	if got := client.SocketPath; got != "unix:///run/user/1000/podman/podman.sock" {
		t.Fatalf("SocketPath = %q, want %q", got, "unix:///run/user/1000/podman/podman.sock")
	}
	if got := session.State(); got != "connected" {
		t.Fatalf("State() = %q, want %q", got, "connected")
	}
	if got := session.Socket(); got != "/run/user/1000/podman/podman.sock" {
		t.Fatalf("Socket() = %q, want %q", got, "/run/user/1000/podman/podman.sock")
	}

	if err := session.Disconnect(); err != nil {
		t.Fatalf("Disconnect() error = %v", err)
	}
	if got := session.State(); got != "closed" {
		t.Fatalf("State() after Disconnect = %q, want %q", got, "closed")
	}
}

func TestServerSessionConnectIsIdempotentUntilDisconnect(t *testing.T) {
	session := NewServerSession(config.Server{
		Host:   "localhost",
		Socket: "/run/user/1000/podman/podman.sock",
	})

	client1, err := session.Connect()
	if err != nil {
		t.Fatalf("first Connect() error = %v", err)
	}
	client2, err := session.Connect()
	if err != nil {
		t.Fatalf("second Connect() error = %v", err)
	}
	if client1 != client2 {
		t.Fatal("Connect() returned different clients for the same active session")
	}
	if got := session.State(); got != "connected" {
		t.Fatalf("State() after repeated Connect = %q, want %q", got, "connected")
	}

	if err := session.Disconnect(); err != nil {
		t.Fatalf("Disconnect() error = %v", err)
	}
	if got := session.Client(); got != nil {
		t.Fatalf("Client() after Disconnect = %#v, want nil", got)
	}
	if got := session.State(); got != "closed" {
		t.Fatalf("State() after Disconnect = %q, want %q", got, "closed")
	}
}
