package ssh

import (
	"context"
	"testing"
	"time"
)

func TestBackoffDelayCapsAtThirtySeconds(t *testing.T) {
	if got := backoffDelay(0); got != 500*time.Millisecond {
		t.Fatalf("backoffDelay(0) = %s, want %s", got, 500*time.Millisecond)
	}
	if got := backoffDelay(10); got != 30*time.Second {
		t.Fatalf("backoffDelay(10) = %s, want %s", got, 30*time.Second)
	}
}

func TestWaitContextReturnsFalseWhenCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if ok := waitContext(ctx, time.Second); ok {
		t.Fatal("waitContext returned true for canceled context")
	}
}

func TestServerTunnelStartsUnknownAndRejectsStopBeforeStart(t *testing.T) {
	tunnel := newServerTunnel("example", "/run/user/1000/podman/podman.sock")
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
