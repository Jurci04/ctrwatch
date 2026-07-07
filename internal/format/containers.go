// Package format provides formatted terminal output for container data.
package format

import (
	"fmt"
	"strings"

	"ctrwatch/internal/runtime"
)

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

// PrintContainers prints a formatted table of containers to stdout.
// The socket path is displayed as a comment line above the table.
func PrintContainers(containers []runtime.Container, socketPath string) {
	fmt.Printf("# socket: %s\n", socketPath)
	fmt.Printf("%-20s %-20s %-30s %-12s %-20s %v\n", "ID", "NAME", "IMAGE", "STATE", "STATUS", "PORTS")

	for _, container := range containers {
		fmt.Printf(
			"%-20s %-20s %-30s %-12s %-20s %v\n",
			container.ID[:12],
			containerName(container.Names),
			container.Image,
			container.State,
			container.Status,
			formatPorts(container.Ports),
		)
	}
}

func containerName(names []string) string {
	if len(names) == 0 {
		return "-"
	}

	return strings.TrimPrefix(names[0], "/")
}
