package commands

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"ctrwatch/internal/runtime"
	"ctrwatch/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

// pollStats periodically fetches CPU/memory stats for all containers
// and sends them to the TUI model's stats channel.
func pollStats(ctx context.Context, containers []containerDef, model *tui.Model) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	statuses := make(map[string]string, len(containers))

	fetch := func(getStatus bool) {
		stats := make(map[string]*runtime.ContainerStats, len(containers))
		for _, c := range containers {
			s, err := c.Client.StatsContainer(ctx, c.Name)
			if err != nil {
				stats[c.Name] = &runtime.ContainerStats{Status: fmt.Sprintf("error: %v", err)}
				continue
			}
			if getStatus {
				info, err := c.Client.InspectContainer(ctx, c.Name)
				if err == nil {
					statuses[c.Name] = info.State.Status
				}
			}
			s.Status = statuses[c.Name]
			stats[c.Name] = s
		}
		if len(stats) > 0 {
			model.StatsCh() <- stats
		}
	}

	fetch(true)
	for {
		select {
		case <-ticker.C:
			fetch(false)
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
	containers, cleanup, err := resolveContainers(fs.Args())
	if err != nil {
		return err
	}
	defer cleanup()
	if len(containers) < 1 {
		return fmt.Errorf("usage: ctrwatch watch [--tail N] [--since DURATION] <container> [container...]")
	}
	if len(containers) > 4 {
		return fmt.Errorf("watch supports max 4 containers")
	}

	sinceUnix := ""
	if *since != "" {
		d, err := time.ParseDuration(*since)
		if err != nil {
			return fmt.Errorf("invalid --since duration %q: %w", *since, err)
		}
		sinceUnix = strconv.FormatInt(time.Now().Add(-d).Unix(), 10)
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

		wg.Go(func() {

			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			var batch []runtime.LogLine

			for {
				select {
				case line, ok := <-lines:
					if !ok {
						if len(batch) > 0 {
							model.LinesCh() <- batch
						}
						goto done
					}
					batch = append(batch, line)
				case <-ticker.C:
					if len(batch) > 0 {
						model.LinesCh() <- batch
						batch = nil
					}
				}
			}

		done:
			if err := readStreamError(errs); err != nil {
				model.LinesCh() <- []runtime.LogLine{
					{Container: c.Name, Text: fmt.Sprintf("error: %v", err)},
				}
			}
		})
	}

	statsCtx, statsStop := context.WithCancel(context.Background())
	go pollStats(statsCtx, containers, model)

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
