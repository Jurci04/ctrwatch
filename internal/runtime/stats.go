package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// ContainerStats holds CPU and memory metrics for a container.
type ContainerStats struct {
	CPUPercent  float64
	MemoryUsage uint64
	MemoryLimit uint64
	// Status is populated by the caller from the inspect endpoint.
	// The stats API does not provide this field directly.
	Status string
}

type statsJSON struct {
	CPUStats    cpuStats    `json:"cpu_stats"`
	PreCPUStats cpuStats    `json:"precpu_stats"`
	MemoryStats memoryStats `json:"memory_stats"`
}

type cpuStats struct {
	CPUUsage   cpuUsage `json:"cpu_usage"`
	SystemCPU  uint64   `json:"system_cpu_usage"`
	OnlineCPUs uint32   `json:"online_cpus"`
}

type cpuUsage struct {
	TotalUsage uint64 `json:"total_usage"`
}

type memoryStats struct {
	Usage uint64 `json:"usage"`
	Limit uint64 `json:"limit"`
}

// StatsContainer fetches a one-shot CPU and memory snapshot for a container.
func (client *Client) StatsContainer(ctx context.Context, containerID string) (*ContainerStats, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("http://localhost/containers/%s/stats?stream=false", containerID), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("stats %s: %s", containerID, resp.Status)
	}

	var raw statsJSON
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	cpuDelta := raw.CPUStats.CPUUsage.TotalUsage - raw.PreCPUStats.CPUUsage.TotalUsage
	systemDelta := raw.CPUStats.SystemCPU - raw.PreCPUStats.SystemCPU

	cpuPercent := 0.0
	if systemDelta > 0 && raw.CPUStats.OnlineCPUs > 0 {
		cpuPercent = (float64(cpuDelta) / float64(systemDelta)) * float64(raw.CPUStats.OnlineCPUs) * 100
	}

	return &ContainerStats{
		CPUPercent:  cpuPercent,
		MemoryUsage: raw.MemoryStats.Usage,
		MemoryLimit: raw.MemoryStats.Limit,
	}, nil
}
