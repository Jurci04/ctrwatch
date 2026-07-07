package commands

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// RunInspect prints detailed metadata about a single container.
func RunInspect(args []string) error {
	containers := parseContainers(args)
	if len(containers) < 1 {
		return fmt.Errorf("usage: ctrwatch inspect <container>")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c := containers[0]
	info, err := c.Client.InspectContainer(ctx, c.Name)
	if err != nil {
		return err
	}

	fmt.Printf("Name:      %s\n", strings.TrimPrefix(info.Name, "/"))
	fmt.Printf("ID:        %s\n", info.ID[:12])
	fmt.Printf("Image:     %s\n", info.Config.Image)
	fmt.Printf("Status:    %s\n", info.State.Status)
	fmt.Printf("Created:   %s\n", info.Created.Format(time.RFC1123))
	fmt.Printf("Restarts:  %d\n", info.RestartCount)

	if len(info.Mounts) > 0 {
		fmt.Println("Mounts:")
		for _, m := range info.Mounts {
			rw := "ro"
			if m.RW {
				rw = "rw"
			}
			fmt.Printf("  %s %s -> %s (%s)\n", m.Type, m.Source, m.Destination, rw)
		}
	}

	if len(info.Config.Env) > 0 {
		fmt.Println("Environment:")
		for _, e := range info.Config.Env {
			fmt.Printf("  %s\n", e)
		}
	}

	if len(info.Config.Labels) > 0 {
		fmt.Println("Labels:")
		for k, v := range info.Config.Labels {
			fmt.Printf("  %s=%s\n", k, v)
		}
	}

	if len(info.NetworkSettings.Ports) > 0 {
		fmt.Println("Ports:")
		for proto, bindings := range info.NetworkSettings.Ports {
			for _, b := range bindings {
				fmt.Printf("  %s:%s -> %s\n", b.HostIP, b.HostPort, proto)
			}
		}
	}

	return nil
}
