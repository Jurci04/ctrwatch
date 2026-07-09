package commands

import (
	"context"
	"time"

	"ctrwatch/src/config"
	"ctrwatch/src/runtime"
	"ctrwatch/src/tui"

	tea "github.com/charmbracelet/bubbletea"
)

func RunDefaultTUI() error {
	var servers []config.Server
	cfg, err := config.Load(config.ConfigPath())
	if err == nil {
		servers = cfg.Servers
	}

	names, clients := loadLocalDefaults(servers)

	opts := runtime.LogOptions{Tail: "100"}
	interval := 10 * time.Second
	if cfg != nil {
		interval = cfg.PollInterval()
	}

	model := tui.NewModel(names, clients, opts, interval, servers)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

func loadLocalDefaults(servers []config.Server) ([]string, []*runtime.Client) {
	configured := map[string]bool{}
	for _, server := range servers {
		if config.IsLocalHost(server.Host) {
			configured[server.Socket] = true
		}
	}

	var names []string
	var clients []*runtime.Client
	for _, socket := range runtime.ExistingDefaultSockets() {
		if configured[socket] {
			continue
		}
		client := runtime.NewClientForSocket(socket)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		containers, err := client.ListContainers(ctx, true)
		cancel()
		if err != nil {
			continue
		}

		for _, c := range containers {
			names = append(names, runtime.ContainerName(c.Names))
			clients = append(clients, client)
		}
	}
	return names, clients
}
