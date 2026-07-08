package commands

import (
	"fmt"
	"slices"
	"strings"

	"ctrwatch/src/config"
	"ctrwatch/src/runtime"
	"ctrwatch/src/ssh"
)

// resolveContainers returns container definitions either from CLI args
// or from a config file when the first arg is @tagname.
// The returned cleanup function kills SSH tunnels.
func resolveContainers(args []string) ([]containerDef, func(), error) {
	if len(args) >= 1 && strings.HasPrefix(args[0], "@") {
		return resolveTagged(args[0][1:])
	}
	return parseContainers(args), func() {}, nil
}

// resolveTaggedServers loads config and returns servers matching the tag.
func resolveTaggedServers(tag string) ([]config.Server, error) {
	cfg, err := config.Load(config.ConfigPath())
	if err != nil {
		return nil, err
	}
	var matched []config.Server
	for _, s := range cfg.Servers {
		if slices.Contains(s.Tags, tag) {
			matched = append(matched, s)
		}
	}
	if len(matched) == 0 {
		return nil, fmt.Errorf("no servers with tag %q in %s", tag, config.ConfigPath())
	}
	return matched, nil
}

// resolveTagged loads the config, matches servers by tag, creates SSH tunnels,
// and returns container definitions with clients.
func resolveTagged(tag string) ([]containerDef, func(), error) {
	matched, err := resolveTaggedServers(tag)
	if err != nil {
		return nil, nil, err
	}

	clients := map[string]*runtime.Client{}
	var defs []containerDef
	var cleanups []func()

	for _, s := range matched {
		var c *runtime.Client
		if config.IsLocalHost(s.Host) {
			c = runtime.NewClientForSocket(s.Socket)
		} else {
			key := s.Host + "\x00" + s.Socket
			if _, ok := clients[key]; !ok {
				session := ssh.NewServerSession(s)
				client, err := session.Connect()
				if err != nil {
					for _, f := range cleanups {
						f()
					}
					return nil, nil, err
				}
				cleanups = append(cleanups, func() { _ = session.Disconnect() })
				c = client
				clients[key] = c
			} else {
				c = clients[key]
			}
		}
		for _, name := range s.Containers {
			defs = append(defs, containerDef{Name: name, Client: c})
		}
	}

	return defs, func() {
		for _, f := range cleanups {
			f()
		}
	}, nil
}
