package commands

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"ctrwatch/internal/runtime"
	"ctrwatch/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

// pollStats periodically fetches CPU/memory stats for all containers
// and sends them to the TUI model's stats channel.
func pollStats(ctx context.Context, client *runtime.Client, containers []string, model *tui.Model) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	fetch := func() {
		stats := make(map[string]*runtime.ContainerStats, len(containers))
		for _, name := range containers {
			s, err := client.StatsContainer(ctx, name)
			if err != nil {
				stats[name] = &runtime.ContainerStats{Status: fmt.Sprintf("error: %v", err)}
				continue
			}
			info, err := client.InspectContainer(ctx, name)
			if err == nil {
				s.Status = info.State.Status
			}
			stats[name] = s
		}
		if len(stats) > 0 {
			model.StatsCh() <- stats
		}
	}

	fetch()
	for {
		select {
		case <-ticker.C:
			fetch()
		case <-ctx.Done():
			return
		}
	}
}

// RunWatch opens a split-screen TUI that streams logs from one or more containers
// and displays live CPU/memory stats per container.
func RunWatch(args []string) error {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	tail := fs.String("tail", "100", "number of previous log lines to show")
	since := fs.String("since", "", "show logs since duration, e.g. 10m, 1h")
	if err := fs.Parse(args); err != nil {
		return err
	}
	containers := parseContainers(fs.Args())
	if len(containers) < 1 {
		return fmt.Errorf("usage: ctrwatch watch [--tail N] [--since DURATION] <container> [container...]")
	}
	if len(containers) > 4 {
		return fmt.Errorf("watch supports max 4 containers")
	}

	sinceUnix, err := parseDurationSince(*since)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	opts := runtime.LogOptions{Tail: *tail, Since: sinceUnix}
	names := make([]string, len(containers))
	for i, c := range containers {
		names[i] = c.Name
	}
	model := tui.NewModel(names)

	var wg sync.WaitGroup

	for _, c := range containers {
		c := c
		lines, errs := c.Client.StreamLogs(ctx, c.Name, opts)

		wg.Add(1)
		go func() {
			defer wg.Done()
			for line := range lines {
				model.LinesCh() <- line
			}
			if err := readStreamError(errs); err != nil {
				model.LinesCh() <- runtime.LogLine{Container: c.Name, Text: fmt.Sprintf("error: %v", err)}
			}
		}()
	}

	statsCtx, statsStop := context.WithCancel(context.Background())
	go pollStats(statsCtx, containers[0].Client, names, model)

	go func() {
		wg.Wait()
		statsStop()
		close(model.LinesCh())
	}()

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}
