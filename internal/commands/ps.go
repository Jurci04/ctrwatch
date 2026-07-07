package commands

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"ctrwatch/internal/config"
	"ctrwatch/internal/runtime"
	"ctrwatch/internal/ssh"
)

// RunPS lists containers in a formatted table.
func RunPS(args []string) error {
	fs := flag.NewFlagSet("ps", flag.ContinueOnError)
	all := fs.Bool("all", false, "show all containers (default shows only running)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	rest := fs.Args()
	if len(rest) >= 1 && strings.HasPrefix(rest[0], "@") {
		return psFromConfig(rest[0][1:], *all)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := runtime.NewClient()
	containers, err := client.ListContainers(ctx, *all)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing containers: %v\n", err)
		return err
	}

	PrintContainers(containers, client.SocketPath)
	return nil
}

func formatPorts(ports []runtime.Port) string {
	parts := make([]string, 0, len(ports))
	for _, p := range ports {
		if p.PublicPort != 0 {
			parts = append(parts, fmt.Sprintf("%s:%d->%d/%s", p.IP, p.PublicPort, p.PrivatePort, p.Type))
		} else {
			parts = append(parts, fmt.Sprintf("%d/%s", p.PrivatePort, p.Type))
		}
	}
	return strings.Join(parts, ", ")
}

func containerName(names []string) string {
	if len(names) == 0 {
		return "-"
	}
	return strings.TrimPrefix(names[0], "/")
}

// PrintContainers prints a formatted table of containers to stdout.
func PrintContainers(containers []runtime.Container, socketPath string) {
	fmt.Printf("# socket: %s\n", socketPath)
	fmt.Printf("%-20s %-20s %-30s %-12s %-20s %v\n", "ID", "NAME", "IMAGE", "STATE", "STATUS", "PORTS")
	for _, container := range containers {
		fmt.Printf(
			"%-20s %-20s %-30s %-12s %-20s %v\n",
			shortID(container.ID),
			containerName(container.Names),
			container.Image,
			container.State,
			container.Status,
			formatPorts(container.Ports),
		)
	}
}

func psFromConfig(tag string, all bool) error {
	matched, err := resolveTaggedServers(tag)
	if err != nil {
		return err
	}

	var cleanups []func()
	defer func() {
		for _, f := range cleanups {
			f()
		}
	}()

	for _, s := range matched {
		label := s.Host
		if config.IsLocalHost(label) {
			label = "localhost"
		}

		var client *runtime.Client
		if config.IsLocalHost(s.Host) {
			client = runtime.NewClient()
		} else {
			localSock, cleanup, err := ssh.Tunnel(s.Host, s.Socket)
			if err != nil {
				return err
			}
			cleanups = append(cleanups, cleanup)
			client = runtime.NewClientForSocket(localSock)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		containers, err := client.ListContainers(ctx, all)
		if err != nil {
			return fmt.Errorf("%s: %w", label, err)
		}
		PrintContainers(filterContainers(containers, s.Containers), client.SocketPath)
	}
	return nil
}

func filterContainers(containers []runtime.Container, names []string) []runtime.Container {
	wanted := make(map[string]bool, len(names))
	for _, name := range names {
		wanted[name] = true
	}

	filtered := containers[:0]
	for _, c := range containers {
		for _, name := range c.Names {
			if wanted[strings.TrimPrefix(name, "/")] {
				filtered = append(filtered, c)
				break
			}
		}
	}
	return filtered
}

func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
