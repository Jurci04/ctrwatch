package commands

import (
	"context"
	"time"

	"ctrwatch/internal/config"
	"ctrwatch/internal/runtime"
	"ctrwatch/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

func RunDefaultTUI() error {
	var servers []config.Server
	cfg, err := config.Load(config.ConfigPath())
	if err == nil {
		servers = cfg.Servers
	}

	var names []string
	var clients []*runtime.Client
	if cfg == nil {
		client := runtime.NewClient()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		containers, err := client.ListContainers(ctx, true)
		if err != nil {
			containers = nil
		}

		names = make([]string, len(containers))
		clients = make([]*runtime.Client, len(containers))
		for i, c := range containers {
			names[i] = runtime.ContainerName(c.Names)
			clients[i] = client
		}
	}

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
