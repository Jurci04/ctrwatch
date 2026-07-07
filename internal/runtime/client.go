// Package runtime provides a client for the container Engine API.
// Compatible with Docker, Podman, and any runtime exposing the container API.
package runtime

import (
	"context"
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
}

func detectSocket() string {
	if h := os.Getenv("DOCKER_HOST"); h != "" {
		return h
	}
	candidates := []string{
		"/var/run/docker.sock",
		"/run/podman/podman.sock",
		"/run/user/1000/podman/podman.sock",
	}
	for _, s := range candidates {
		if _, err := os.Stat(s); err == nil {
			return "unix://" + s
		}
	}
	return "unix:///var/run/docker.sock"
}

func clientForAddr(addr string) *Client {
	var dial func(ctx context.Context, network, addr string) (net.Conn, error)
	if strings.HasPrefix(addr, "unix://") {
		sock := strings.TrimPrefix(addr, "unix://")
		dial = func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", sock)
		}
	} else {
		dial = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		}
	}
	return &Client{
		httpClient: &http.Client{Transport: &http.Transport{DialContext: dial}},
		SocketPath: addr,
	}
}

// NewClient creates a Client connected to the first available daemon socket.
// The socket is resolved from DOCKER_HOST, then common socket paths.
func NewClient() *Client {
	return clientForAddr(detectSocket())
}

// NewClientForSocket creates a Client connected to the given Unix socket path.
func NewClientForSocket(socketPath string) *Client {
	return clientForAddr("unix://" + socketPath)
}
