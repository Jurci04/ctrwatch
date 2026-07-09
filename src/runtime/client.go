// Package runtime provides a client for the container Engine API.
// Compatible with Docker, Podman, and any runtime exposing the container API.
package runtime

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
)

// Client is an HTTP client that speaks the container Engine API over a Unix socket or TCP.
type Client struct {
	httpClient *http.Client
	// SocketPath is the resolved daemon address (unix:// or tcp://).
	SocketPath string
	Runtime    string
}

func detectSocket() string {
	for _, s := range ExistingDefaultSockets() {
		return "unix://" + s
	}
	return "unix:///var/run/docker.sock"
}

func DefaultSockets() []string {
	return []string{
		"/var/run/docker.sock",
		"/run/podman/podman.sock",
		fmt.Sprintf("/run/user/%d/podman/podman.sock", os.Getuid()),
	}
}

func ExistingDefaultSockets() []string {
	var sockets []string
	for _, s := range DefaultSockets() {
		if _, err := os.Stat(s); err == nil {
			sockets = append(sockets, s)
		}
	}
	if len(sockets) == 0 {
		return []string{"/var/run/docker.sock"}
	}
	return sockets
}

func clientForAddr(addr string) *Client {
	var dial func(ctx context.Context, _, _ string) (net.Conn, error)

	switch {
	case strings.HasPrefix(addr, "tcp://"), strings.HasPrefix(addr, "http://"):
		host := addr
		for _, p := range []string{"tcp://", "http://"} {
			host = strings.TrimPrefix(host, p)
		}
		dial = func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "tcp", host)
		}
	default:
		sock := strings.TrimPrefix(addr, "unix://")
		dial = func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", sock)
		}
	}

	return &Client{
		httpClient: &http.Client{
			Transport: &http.Transport{
				DialContext: dial,
			},
		},
		SocketPath: addr,
		Runtime:    RuntimeKind(addr),
	}
}

func RuntimeKind(addr string) string {
	a := strings.ToLower(addr)
	switch {
	case strings.Contains(a, "podman"):
		return "podman"
	case strings.Contains(a, "docker"):
		return "docker"
	case strings.HasPrefix(a, "tcp://"), strings.HasPrefix(a, "http://"):
		return "tcp"
	default:
		return "runtime"
	}
}

// NewClient creates a Client connected to the first available daemon socket.
// The socket is resolved from common local Docker and Podman socket paths.
func NewClient() *Client {
	return clientForAddr(detectSocket())
}

// NewClientForSocket creates a Client connected to the given socket path.
// Supports unix://, tcp://, http:// prefixes and bare paths (defaults to unix://).
// ReadStreamError reads a single value from the stream error channel.
// The channel carries at most one error per stream.
func ReadStreamError(errs <-chan error) error {
	err, ok := <-errs
	if !ok || err == nil {
		return nil
	}
	return err
}

func ShortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func ContainerName(names []string) string {
	if len(names) == 0 {
		return "-"
	}
	return strings.TrimPrefix(names[0], "/")
}

func NewClientForSocket(socketPath string) *Client {
	if strings.HasPrefix(socketPath, "unix://") ||
		strings.HasPrefix(socketPath, "tcp://") ||
		strings.HasPrefix(socketPath, "http://") {
		return clientForAddr(socketPath)
	}
	return clientForAddr("unix://" + socketPath)
}
