package ssh

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ctrwatch/src/config"
	"ctrwatch/src/runtime"
)

type ResolvedServer struct {
	Server     config.Server
	Session    *ServerSession
	Client     *runtime.Client
	Containers []string
	Runtime    string
}

func ResolveServer(server config.Server) ([]ResolvedServer, error) {
	if server.Socket != "" {
		session := NewServerSession(server)
		client, err := session.Connect()
		if err != nil {
			return nil, err
		}
		found, err := findContainersOnSocket(client, server.Containers)
		if err != nil {
			_ = session.Disconnect()
			return nil, fmt.Errorf("%s: %w", server.Socket, err)
		}
		if len(found) == 0 {
			_ = session.Disconnect()
			return nil, fmt.Errorf("%s: no configured containers found", server.Socket)
		}
		return []ResolvedServer{{
			Server:     server,
			Session:    session,
			Client:     client,
			Containers: found,
			Runtime:    client.Runtime,
		}}, nil
	}

	candidates := runtime.DefaultSockets()
	if config.IsLocalHost(server.Host) {
		candidates = runtime.ExistingDefaultSockets()
	}

	var resolved []ResolvedServer
	var errs []string
	for _, socket := range candidates {
		candidate := server
		candidate.Socket = socket
		session := NewServerSession(candidate)
		client, err := session.Connect()
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", socket, err))
			continue
		}
		found, err := findContainersOnSocket(client, server.Containers)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", socket, err))
			_ = session.Disconnect()
			continue
		}
		if len(found) == 0 {
			_ = session.Disconnect()
			continue
		}
		resolved = append(resolved, ResolvedServer{
			Server:     candidate,
			Session:    session,
			Client:     client,
			Containers: found,
			Runtime:    client.Runtime,
		})
	}
	if len(resolved) == 0 {
		if len(errs) > 0 {
			return nil, fmt.Errorf("no configured containers found; %s", strings.Join(errs, "; "))
		}
		return nil, fmt.Errorf("no configured containers found")
	}
	return resolved, nil
}

var findContainersOnSocket = containersOnSocket

func containersOnSocket(client *runtime.Client, names []string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	containers, err := client.ListContainers(ctx, true)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, container := range containers {
		for _, name := range container.Names {
			seen[strings.TrimPrefix(name, "/")] = true
		}
	}
	var found []string
	for _, name := range names {
		if seen[name] {
			found = append(found, name)
		}
	}
	return found, nil
}
