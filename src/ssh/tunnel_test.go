package ssh

import (
	"context"
	"testing"
	"time"
)

func TestWaitContextReturnsFalseWhenCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if ok := waitContext(ctx, time.Second); ok {
		t.Fatal("waitContext returned true for canceled context")
	}
}

func TestServerTunnelStartsUnknownAndRejectsStopBeforeStart(t *testing.T) {
	tunnel := NewServerTunnel("example", "/run/user/1000/podman/podman.sock")
	if got := tunnel.State(); got != "unknown" {
		t.Fatalf("State() = %q, want %q", got, "unknown")
	}
	if got := tunnel.LastError(); got != nil {
		t.Fatalf("LastError() = %v, want nil", got)
	}
	if got := tunnel.Socket(); got != "" {
		t.Fatalf("Socket() = %q, want empty", got)
	}
	if err := tunnel.Stop(); err == nil {
		t.Fatal("Stop() before Start() returned nil error")
	}
}
