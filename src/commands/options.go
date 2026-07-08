package commands

import (
	"strings"

	"ctrwatch/src/runtime"
)

// containerDef associates a container name with its runtime client.
// Multiple containers may share a client when they use the same socket.
type containerDef struct {
	Name   string
	Client *runtime.Client
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
