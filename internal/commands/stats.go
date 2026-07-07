package commands

import (
	"context"
	"fmt"
	"time"
)

// RunStats prints a one-shot CPU and memory snapshot for one or more containers.
func RunStats(args []string) error {
	containers := parseContainers(args)
	if len(containers) < 1 {
		return fmt.Errorf("usage: ctrwatch stats <container> [container...]")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, c := range containers {
		s, err := c.Client.StatsContainer(ctx, c.Name)
		if err != nil {
			fmt.Printf("%-20s error: %v\n", c.Name, err)
			continue
		}
		memMB := float64(s.MemoryUsage) / 1024 / 1024
		limitMB := float64(s.MemoryLimit) / 1024 / 1024
		fmt.Printf("%-20s CPU: %6.1f%%  MEM: %.0f / %.0f MB\n", c.Name, s.CPUPercent, memMB, limitMB)
	}

	return nil
}
