package commands

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"ctrwatch/src/runtime"
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

	printContainers(containers, client.SocketPath)
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

func printContainers(containers []runtime.Container, socketPath string) {
	fmt.Printf("# socket: %s\n", socketPath)
	fmt.Printf("%-20s %-20s %-30s %-12s %-20s %v\n", "ID", "NAME", "IMAGE", "STATE", "STATUS", "PORTS")
	for _, container := range containers {
		fmt.Printf(
			"%-20s %-20s %-30s %-12s %-20s %v\n",
			runtime.ShortID(container.ID),
			runtime.ContainerName(container.Names),
			container.Image,
			container.State,
			container.Status,
			formatPorts(container.Ports),
		)
	}
}

func psFromConfig(tag string, all bool) error {
	defs, cleanup, err := resolveTagged(tag)
	if err != nil {
		return err
	}
	defer cleanup()

	byClient := map[*runtime.Client][]string{}
	for _, d := range defs {
		byClient[d.Client] = append(byClient[d.Client], d.Name)
	}

	for client, names := range byClient {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		containers, err := client.ListContainers(ctx, all)
		if err != nil {
			return fmt.Errorf("%s: %w", client.SocketPath, err)
		}
		printContainers(filterContainers(containers, names), client.SocketPath)
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
