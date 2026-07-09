package commands

import (
	"fmt"
	"slices"
	"strings"

	"ctrwatch/src/config"
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

	var defs []containerDef
	var cleanups []func()

	for _, s := range matched {
		resolved, err := ssh.ResolveServer(s)
		if err != nil {
			for _, f := range cleanups {
				f()
			}
			return nil, nil, err
		}
		for _, endpoint := range resolved {
			session := endpoint.Session
			cleanups = append(cleanups, func() { _ = session.Disconnect() })
			for _, name := range endpoint.Containers {
				defs = append(defs, containerDef{Name: name, Client: endpoint.Client})
			}
		}
	}

	return defs, func() {
		for _, f := range cleanups {
			f()
		}
	}, nil
}
