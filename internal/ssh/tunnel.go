package ssh

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"
)

type serverTunnel struct {
	server       string
	remoteSocket string
	localSocket  string

	mu      sync.Mutex
	cleanup func()
	state   string
	lastErr error
	ctx     context.Context
	cancel  context.CancelFunc
	started bool
}

func newServerTunnel(server, remoteSocket string) *serverTunnel {
	return &serverTunnel{server: server, remoteSocket: remoteSocket}
}

func sshTunnel(host, remoteSocket string) (localSocket string, cleanup func(), err error) {
	if _, err := exec.LookPath("ssh"); err != nil {
		return "", nil, fmt.Errorf("ssh not found in PATH")
	}

	file, err := os.CreateTemp("", "ctrwatch-*.sock")
	if err != nil {
		return "", nil, fmt.Errorf("temp socket: %w", err)
	}
	localSocket = file.Name()
	if err := file.Close(); err != nil {
		return "", nil, fmt.Errorf("temp socket: %w", err)
	}
	_ = os.Remove(localSocket)

	cmd := exec.Command("ssh", "-L", localSocket+":"+remoteSocket, "-N", host)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return "", nil, fmt.Errorf("ssh tunnel: %w", err)
	}

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(localSocket); err != nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}

		conn, err := net.DialTimeout("unix", localSocket, 2*time.Second)
		if err != nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}

		if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
			_ = conn.Close()
			time.Sleep(200 * time.Millisecond)
			continue
		}

		if _, err := conn.Write([]byte("GET /_ping HTTP/1.1\r\nHost: localhost\r\n\r\n")); err != nil {
			_ = conn.Close()
			time.Sleep(200 * time.Millisecond)
			continue
		}

		resp := make([]byte, 32)
		_, err = conn.Read(resp)
		_ = conn.Close()
		if err == nil {
			return localSocket, func() {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
				_ = os.Remove(localSocket)
			}, nil
		}

		time.Sleep(200 * time.Millisecond)
	}

	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	_ = os.Remove(localSocket)
	return "", nil, fmt.Errorf("ssh tunnel to %s: %s", host, bytes.TrimSpace(stderr.Bytes()))
}

func (tunnel *serverTunnel) Start() error {
	tunnel.mu.Lock()
	if tunnel.started {
		tunnel.mu.Unlock()
		return fmt.Errorf("tunnel already started")
	}
	tunnel.state = "connecting"
	tunnel.lastErr = nil
	tunnel.started = true
	tunnel.ctx, tunnel.cancel = context.WithCancel(context.Background())
	ctx := tunnel.ctx
	tunnel.mu.Unlock()

	localSocket, cleanup, err := sshTunnel(tunnel.server, tunnel.remoteSocket)
	if err != nil {
		tunnel.mu.Lock()
		tunnel.state = "failed"
		tunnel.lastErr = err
		tunnel.started = false
		if tunnel.cancel != nil {
			tunnel.cancel()
			tunnel.cancel = nil
		}
		tunnel.ctx = nil
		tunnel.mu.Unlock()
		return err
	}

	tunnel.mu.Lock()
	tunnel.localSocket = localSocket
	tunnel.cleanup = cleanup
	tunnel.state = "ready"
	tunnel.lastErr = nil
	tunnel.mu.Unlock()

	go tunnel.supervisorLoop(ctx)
	return nil
}

func (tunnel *serverTunnel) Stop() error {
	tunnel.mu.Lock()
	if !tunnel.started {
		tunnel.mu.Unlock()
		return fmt.Errorf("tunnel not running")
	}
	cancel := tunnel.cancel
	cleanup := tunnel.cleanup
	localSocket := tunnel.localSocket
	tunnel.cancel = nil
	tunnel.cleanup = nil
	tunnel.localSocket = ""
	tunnel.state = "closed"
	tunnel.lastErr = nil
	tunnel.started = false
	tunnel.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if cleanup != nil {
		cleanup()
	}
	if localSocket != "" {
		if err := os.Remove(localSocket); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove local socket: %w", err)
		}
	}
	return nil
}

func (tunnel *serverTunnel) State() string {
	tunnel.mu.Lock()
	defer tunnel.mu.Unlock()
	if tunnel.state == "" {
		return "unknown"
	}
	return tunnel.state
}

func (tunnel *serverTunnel) LastError() error {
	tunnel.mu.Lock()
	defer tunnel.mu.Unlock()
	return tunnel.lastErr
}

func (tunnel *serverTunnel) Socket() string {
	tunnel.mu.Lock()
	defer tunnel.mu.Unlock()
	return tunnel.localSocket
}

func (tunnel *serverTunnel) supervisorLoop(ctx context.Context) {
	const checkInterval = 10 * time.Second
	for {
		if ctx.Err() != nil {
			tunnel.cleanupActiveConnection()
			tunnel.setState("closed", nil)
			return
		}

		ticker := time.NewTicker(checkInterval)
		failed := false
		for !failed {
			select {
			case <-ctx.Done():
				ticker.Stop()
				tunnel.cleanupActiveConnection()
				tunnel.setState("closed", nil)
				return
			case <-ticker.C:
				if err := probeTunnel(tunnel.socketPath()); err != nil {
					ticker.Stop()
					tunnel.cleanupActiveConnection()
					tunnel.setState("reconnecting", err)
					if err := tunnel.reconnectLoop(ctx); err != nil {
						if ctx.Err() != nil {
							return
						}
						tunnel.setState("failed", err)
						return
					}
					failed = true
				}
			}
		}
	}
}

func (tunnel *serverTunnel) reconnectLoop(ctx context.Context) error {
	for attempt := 0; ; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		localSocket, cleanup, err := sshTunnel(tunnel.server, tunnel.remoteSocket)
		if err == nil {
			tunnel.replaceConnection(localSocket, cleanup)
			tunnel.setState("ready", nil)
			return nil
		}

		tunnel.setState("reconnecting", err)
		if !waitContext(ctx, backoffDelay(attempt)) {
			return ctx.Err()
		}
	}
}

func (tunnel *serverTunnel) setState(state string, err error) {
	tunnel.mu.Lock()
	tunnel.state = state
	tunnel.lastErr = err
	tunnel.mu.Unlock()
}

func (tunnel *serverTunnel) cleanupActiveConnection() {
	tunnel.mu.Lock()
	cleanup := tunnel.cleanup
	localSocket := tunnel.localSocket
	tunnel.cleanup = nil
	tunnel.localSocket = ""
	tunnel.mu.Unlock()

	if cleanup != nil {
		cleanup()
	}
	if localSocket != "" {
		_ = os.Remove(localSocket)
	}
}

func (tunnel *serverTunnel) replaceConnection(localSocket string, cleanup func()) {
	tunnel.mu.Lock()
	tunnel.localSocket = localSocket
	tunnel.cleanup = cleanup
	tunnel.mu.Unlock()
}

func (tunnel *serverTunnel) socketPath() string {
	tunnel.mu.Lock()
	defer tunnel.mu.Unlock()
	return tunnel.localSocket
}

func probeTunnel(socket string) error {
	if socket == "" {
		return fmt.Errorf("empty tunnel socket")
	}

	conn, err := net.DialTimeout("unix", socket, 2*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		return err
	}
	if _, err := conn.Write([]byte("GET /_ping HTTP/1.1\r\nHost: localhost\r\n\r\n")); err != nil {
		return err
	}
	resp := make([]byte, 32)
	_, err = conn.Read(resp)
	return err
}

func backoffDelay(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	delay := 500 * time.Millisecond
	for i := 0; i < attempt; i++ {
		delay *= 2
		if delay >= 30*time.Second {
			return 30 * time.Second
		}
	}
	if delay > 30*time.Second {
		return 30 * time.Second
	}
	return delay
}

func waitContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
