package commands

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"ctrwatch/internal/format"
	"ctrwatch/internal/runtime"
)

// RunPS lists containers in a formatted table.
func RunPS(args []string) error {
	fs := flag.NewFlagSet("ps", flag.ContinueOnError)
	all := fs.Bool("all", false, "show all containers (default shows only running)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rest := fs.Args()
	if len(rest) >= 1 && strings.HasPrefix(rest[0], "@") {
		return psFromConfig(ctx, rest[0][1:], *all)
	}

	client := runtime.NewClient()
	containers, err := client.ListContainers(ctx, *all)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing containers: %v\n", err)
		return err
	}

	format.PrintContainers(containers, client.SocketPath)
	return nil
}

func psFromConfig(ctx context.Context, tag string, all bool) error {
	cfg, err := loadConfig(configPath())
	if err != nil {
		return err
	}

	var matched []Server
	for _, s := range cfg.Servers {
		for _, t := range s.Tags {
			if t == tag {
				matched = append(matched, s)
				break
			}
		}
	}
	if len(matched) == 0 {
		return fmt.Errorf("no servers with tag %q in %s", tag, configPath())
	}

	var cleanups []func()
	defer func() {
		for _, f := range cleanups {
			f()
		}
	}()

	for _, s := range matched {
		label := s.Host
		if label == "" {
			label = "local"
		}

		var client *runtime.Client
		if s.Host == "" {
			client = runtime.NewClient()
		} else {
			localSock, cleanup, err := sshTunnel(s.Host, s.Socket)
			if err != nil {
				return err
			}
			cleanups = append(cleanups, cleanup)
			client = runtime.NewClientForSocket("unix://" + localSock)
		}

		containers, err := client.ListContainers(ctx, all)
		if err != nil {
			return fmt.Errorf("%s: %w", label, err)
		}
		format.PrintContainers(containers, client.SocketPath)
	}
	return nil
}
