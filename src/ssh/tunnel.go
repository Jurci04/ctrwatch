package ssh

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"ctrwatch/src/utils"
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

func NewServerTunnel(server, remoteSocket string) *serverTunnel {
	return &serverTunnel{server: server, remoteSocket: remoteSocket}
}

func sshTunnel(host, remoteSocket string) (localSocket string, cleanup func(), err error) {
	utils.Debugf("ssh tunnel start host=%q remoteSocket=%q", host, remoteSocket)
	if _, err := exec.LookPath("ssh"); err != nil {
		utils.Debugf("ssh tunnel missing ssh binary err=%v", err)
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

	cmd := exec.Command("ssh",
		"-o", "ExitOnForwardFailure=yes",
		"-o", "ConnectTimeout=10",
		"-o", "ServerAliveInterval=30",
		"-o", "BatchMode=yes",
		"-L", localSocket+":"+remoteSocket,
		"-N", host,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		utils.Debugf("ssh tunnel command failed host=%q localSocket=%q err=%v", host, localSocket, err)
		return "", nil, fmt.Errorf("ssh tunnel: %w", err)
	}
	utils.Debugf("ssh tunnel command started host=%q localSocket=%q remoteSocket=%q pid=%d", host, localSocket, remoteSocket, cmd.Process.Pid)

	deadline := time.Now().Add(30 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		if _, err := os.Stat(localSocket); err != nil {
			lastErr = err
			time.Sleep(200 * time.Millisecond)
			continue
		}

		conn, err := net.DialTimeout("unix", localSocket, 2*time.Second)
		if err != nil {
			lastErr = err
			time.Sleep(200 * time.Millisecond)
			continue
		}

		if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
			_ = conn.Close()
			lastErr = err
			time.Sleep(200 * time.Millisecond)
			continue
		}

		if _, err := conn.Write([]byte("GET /_ping HTTP/1.1\r\nHost: localhost\r\n\r\n")); err != nil {
			_ = conn.Close()
			lastErr = err
			time.Sleep(200 * time.Millisecond)
			continue
		}

		resp := make([]byte, 32)
		_, err = conn.Read(resp)
		_ = conn.Close()
		if err == nil {
			utils.Debugf("ssh tunnel ready host=%q localSocket=%q remoteSocket=%q", host, localSocket, remoteSocket)
			return localSocket, func() {
				utils.Debugf("ssh tunnel cleanup host=%q localSocket=%q", host, localSocket)
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
				_ = os.Remove(localSocket)
			}, nil
		}
		lastErr = err

		time.Sleep(200 * time.Millisecond)
	}

	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	_ = os.Remove(localSocket)
	utils.Debugf("ssh tunnel timeout host=%q remoteSocket=%q lastErr=%v stderr=%q", host, remoteSocket, lastErr, bytes.TrimSpace(stderr.Bytes()))
	detail := strings.TrimSpace(stderr.String())
	if detail == "" && lastErr != nil {
		detail = lastErr.Error()
	}
	if detail == "" {
		detail = "timed out waiting for forwarded socket"
	}
	return "", nil, fmt.Errorf("ssh tunnel to %s: %s", host, detail)
}

func (tunnel *serverTunnel) dialAndReplace() error {
	localSocket, cleanup, err := sshTunnel(tunnel.server, tunnel.remoteSocket)
	if err != nil {
		return err
	}
	tunnel.mu.Lock()
	tunnel.localSocket = localSocket
	tunnel.cleanup = cleanup
	tunnel.state = "connected"
	tunnel.lastErr = nil
	tunnel.mu.Unlock()
	utils.Debugf("server tunnel dialed host=%q remoteSocket=%q localSocket=%q", tunnel.server, tunnel.remoteSocket, localSocket)
	return nil
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
	utils.Debugf("server tunnel state=connecting host=%q remoteSocket=%q", tunnel.server, tunnel.remoteSocket)

	if err := tunnel.dialAndReplace(); err != nil {
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
		utils.Debugf("server tunnel state=failed host=%q remoteSocket=%q err=%v", tunnel.server, tunnel.remoteSocket, err)
		return err
	}

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

const tunnelProbeInterval = 10 * time.Second

func (tunnel *serverTunnel) supervisorLoop(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			tunnel.cleanupActiveConnection()
			tunnel.setState("closed", nil)
			return
		}

		ticker := time.NewTicker(tunnelProbeInterval)
		failed := false
		for !failed {
			select {
			case <-ctx.Done():
				ticker.Stop()
				tunnel.cleanupActiveConnection()
				tunnel.setState("closed", nil)
				return
			case <-ticker.C:
				if err := probeTunnel(tunnel.Socket()); err != nil {
					ticker.Stop()
					tunnel.cleanupActiveConnection()
					tunnel.setState("reconnecting", err)
					utils.Debugf("server tunnel probe failed host=%q err=%v", tunnel.server, err)
					if err := tunnel.reconnectLoop(ctx); err != nil {
						if ctx.Err() != nil {
							return
						}
						tunnel.setState("failed", err)
						utils.Debugf("server tunnel reconnect failed host=%q err=%v", tunnel.server, err)
						return
					}
					failed = true
				}
			}
		}
	}
}

func (tunnel *serverTunnel) reconnectLoop(ctx context.Context) error {
	const maxDelay = 10 * time.Second
	delay := 500 * time.Millisecond

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if err := tunnel.dialAndReplace(); err == nil {
			utils.Debugf("server tunnel reconnected host=%q localSocket=%q", tunnel.server, tunnel.localSocket)
			return nil
		} else {
			tunnel.setState("reconnecting", err)
		}

		if !waitContext(ctx, delay) {
			return ctx.Err()
		}
		delay = time.Duration(float64(delay) * 2)
		if delay > maxDelay {
			delay = maxDelay
		}
		delay += time.Duration(float64(delay) * 0.25 * (rand.Float64()*2 - 1))
	}
}

func (tunnel *serverTunnel) setState(state string, err error) {
	tunnel.mu.Lock()
	tunnel.state = state
	tunnel.lastErr = err
	tunnel.mu.Unlock()
	utils.Debugf("server tunnel state=%s host=%q err=%v", state, tunnel.server, err)
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

func probeTunnel(socket string) error {
	if socket == "" {
		return fmt.Errorf("empty tunnel socket")
	}

	conn, err := net.DialTimeout("unix", socket, 2*time.Second)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

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
