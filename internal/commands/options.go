package commands

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"ctrwatch/internal/runtime"
)

// containerDef associates a container name with its runtime client.
// Multiple containers may share a client when they use the same socket.
type containerDef struct {
	Name   string
	Client *runtime.Client
}

// parseDurationSince converts a human-readable duration (e.g. "10m", "1h")
// to a Unix timestamp string suitable for the container API.
func parseDurationSince(since string) (string, error) {
	if since == "" {
		return "", nil
	}
	d, err := time.ParseDuration(since)
	if err != nil {
		return "", fmt.Errorf("invalid --since duration %q: %w", since, err)
	}
	return strconv.FormatInt(time.Now().Add(-d).Unix(), 10), nil
}

// parseContainers parses container arguments that may include an optional
// socket override (name@socket_path). It deduplicates clients by socket.
func parseContainers(names []string) []containerDef {
	clients := map[string]*runtime.Client{}
	defs := make([]containerDef, 0, len(names))
	for _, a := range names {
		name, sock := a, ""
		if before, after, ok := strings.Cut(a, "@"); ok {
			name, sock = before, after
		}
		c, ok := clients[sock]
		if !ok {
			if sock == "" {
				c = runtime.NewClient()
			} else {
				c = runtime.NewClientForSocket(sock)
			}
			clients[sock] = c
		}
		defs = append(defs, containerDef{Name: name, Client: c})
	}
	return defs
}
