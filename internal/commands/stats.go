package commands

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"time"
)

// RunStats prints a one-shot CPU and memory snapshot for one or more containers.
func RunStats(args []string) error {
	fs := flag.NewFlagSet("stats", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "output as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	containers, cleanup, err := resolveContainers(fs.Args())
	if err != nil {
		return err
	}
	defer cleanup()
	if len(containers) < 1 {
		return fmt.Errorf("usage: ctrwatch stats [--json] <container> [container...]")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	type stat struct {
		Name        string  `json:"name"`
		CPUPercent  float64 `json:"cpu_percent"`
		MemoryUsage uint64  `json:"memory_usage"`
		MemoryLimit uint64  `json:"memory_limit"`
		Status      string  `json:"status"`
	}
	var results []stat

	for _, c := range containers {
		s, err := c.Client.StatsContainer(ctx, c.Name)
		if err != nil {
			if *jsonOut {
				results = append(results, stat{Name: c.Name, Status: fmt.Sprintf("error: %v", err)})
			} else {
				fmt.Printf("%-20s error: %v\n", c.Name, err)
			}
			continue
		}
		if *jsonOut {
			results = append(results, stat{
				Name: c.Name, CPUPercent: s.CPUPercent,
				MemoryUsage: s.MemoryUsage, MemoryLimit: s.MemoryLimit,
				Status: s.Status,
			})
		} else {
			memMB := float64(s.MemoryUsage) / 1024 / 1024
			limitMB := float64(s.MemoryLimit) / 1024 / 1024
			fmt.Printf("%-20s CPU: %6.1f%%  MEM: %.0f / %.0f MB\n", c.Name, s.CPUPercent, memMB, limitMB)
		}
	}

	if *jsonOut {
		b, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
	}

	return nil
}
