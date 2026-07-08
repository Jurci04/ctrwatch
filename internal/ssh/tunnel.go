package ssh

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Tunnel spawns an SSH tunnel that forwards a remote Unix socket to a local
// Unix socket. Returns the local socket path and a cleanup function.
//
// ponytail: uses system ssh for auth (keys, agent, config). Upgrade to
// Go-native SSH client if Windows portability or subprocess management matters.
func Tunnel(host, remoteSocket string) (localSocket string, cleanup func(), err error) {
	if _, err := exec.LookPath("ssh"); err != nil {
		return "", nil, fmt.Errorf("ssh not found in PATH")
	}
	f, err := os.CreateTemp("", "ctrwatch-*.sock")
	if err != nil {
		return "", nil, fmt.Errorf("temp socket: %w", err)
	}
	localSocket = f.Name()
	f.Close()
	os.Remove(localSocket)
	cmd := exec.Command("ssh",
		"-L", localSocket+":"+remoteSocket,
		"-N", host,
	)
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
		conn.SetDeadline(time.Now().Add(2 * time.Second))
		_, err = conn.Write([]byte("GET /_ping HTTP/1.1\r\nHost: localhost\r\n\r\n"))
		if err != nil {
			conn.Close()
			time.Sleep(200 * time.Millisecond)
			continue
		}
		resp := make([]byte, 32)
		_, err = conn.Read(resp)
		conn.Close()
		if err == nil {
			return localSocket, func() {
				cmd.Process.Kill()
				cmd.Wait()
				os.Remove(localSocket)
			}, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	cmd.Process.Kill()
	cmd.Wait()
	os.Remove(localSocket)
	return "", nil, fmt.Errorf("ssh tunnel to %s: %s", host, strings.TrimSpace(stderr.String()))
}
