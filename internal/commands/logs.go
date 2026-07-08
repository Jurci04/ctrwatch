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
)

// RunLogs streams logs from one or more containers to stdout.
// Each line is prefixed with the container name.
func RunLogs(args []string) error {
	fs := flag.NewFlagSet("logs", flag.ContinueOnError)
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
		return fmt.Errorf("usage: ctrwatch logs [--tail N] [--since DURATION] <container> [container...]")
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

	mergedLines := make(chan runtime.LogLine)
	mergedErrors := make(chan error, len(containers))

	var wg sync.WaitGroup

	for _, c := range containers {
		lines, errs := c.Client.StreamLogs(ctx, c.Name, opts)

		wg.Go(func() {

			for line := range lines {
				select {
				case mergedLines <- line:
				case <-ctx.Done():
					return
				}
			}

			if err := runtime.ReadStreamError(errs); err != nil {
				select {
				case mergedErrors <- err:
				case <-ctx.Done():
				}
			}
		})
	}

	go func() {
		wg.Wait()
		close(mergedLines)
		close(mergedErrors)
	}()

	for {
		select {
		case line, ok := <-mergedLines:
			if !ok {
				return runtime.ReadStreamError(mergedErrors)
			}
			fmt.Printf("[%-20s] %s\n", line.Container, line.Text)

		case err, ok := <-mergedErrors:
			if ok && err != nil {
				return err
			}

		case <-ctx.Done():
			return nil
		}
	}
}
