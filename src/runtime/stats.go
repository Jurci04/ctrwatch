package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type ContainerStats struct {
	CPUPercent    float64
	MemoryUsage   uint64
	MemoryLimit   uint64
	Status        string
	Uptime        string
	PIDsCurrent   uint64
	NetRxBytes    uint64
	NetTxBytes    uint64
	BlkReadBytes  uint64
	BlkWriteBytes uint64
}

type statsJSON struct {
	CPUStats    cpuStats            `json:"cpu_stats"`
	PreCPUStats cpuStats            `json:"precpu_stats"`
	MemoryStats memoryStats         `json:"memory_stats"`
	PIDsStats   pidsStats           `json:"pids_stats"`
	BlkioStats  blkioStats          `json:"blkio_stats"`
	Networks    map[string]netStats `json:"networks"`
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

type pidsStats struct {
	Current uint64 `json:"current"`
}

type blkioStats struct {
	IOServiceBytesRecursive []blkioEntry `json:"io_service_bytes_recursive"`
}

type blkioEntry struct {
	Op    string `json:"op"`
	Value uint64 `json:"value"`
}

type netStats struct {
	RxBytes uint64 `json:"rx_bytes"`
	TxBytes uint64 `json:"tx_bytes"`
}

func (client *Client) StatsContainer(ctx context.Context, containerID string) (*ContainerStats, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		"GET",
		fmt.Sprintf("http://localhost/containers/%s/stats?stream=false", containerID),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

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

	var netRx, netTx uint64
	for _, iface := range raw.Networks {
		netRx += iface.RxBytes
		netTx += iface.TxBytes
	}

	var blkRead, blkWrite uint64
	for _, e := range raw.BlkioStats.IOServiceBytesRecursive {
		switch e.Op {
		case "read":
			blkRead += e.Value
		case "write":
			blkWrite += e.Value
		}
	}

	return &ContainerStats{
		CPUPercent:    cpuPercent,
		MemoryUsage:   raw.MemoryStats.Usage,
		MemoryLimit:   raw.MemoryStats.Limit,
		PIDsCurrent:   raw.PIDsStats.Current,
		NetRxBytes:    netRx,
		NetTxBytes:    netTx,
		BlkReadBytes:  blkRead,
		BlkWriteBytes: blkWrite,
	}, nil
}
