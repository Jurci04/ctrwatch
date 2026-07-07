package commands

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

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
	containers := parseContainers(fs.Args())
	if len(containers) < 1 {
		return fmt.Errorf("usage: ctrwatch logs [--tail N] [--since DURATION] <container> [container...]")
	}

	sinceUnix, err := parseDurationSince(*since)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	opts := runtime.LogOptions{Tail: *tail, Since: sinceUnix}

	mergedLines := make(chan runtime.LogLine)
	mergedErrors := make(chan error, len(containers))

	var wg sync.WaitGroup

	for _, c := range containers {
		c := c
		lines, errs := c.Client.StreamLogs(ctx, c.Name, opts)

		wg.Add(1)
		go func() {
			defer wg.Done()

			for line := range lines {
				select {
				case mergedLines <- line:
				case <-ctx.Done():
					return
				}
			}

			if err := readStreamError(errs); err != nil {
				select {
				case mergedErrors <- err:
				case <-ctx.Done():
				}
			}
		}()
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
				return readStreamError(mergedErrors)
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

// readStreamError reads a single value from the error channel.
// The runtime sends at most one error per stream.
func readStreamError(errs <-chan error) error {
	err, ok := <-errs
	if !ok || err == nil {
		return nil
	}
	return err
}
