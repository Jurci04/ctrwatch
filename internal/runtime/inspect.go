package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ContainerInspect holds detailed metadata about a single container.
type ContainerInspect struct {
	ID              string                   `json:"Id"`
	Name            string                   `json:"Name"`
	Created         time.Time                `json:"Created"`
	State           ContainerState           `json:"State"`
	Config          ContainerConfig          `json:"Config"`
	Mounts          []ContainerMount         `json:"Mounts"`
	NetworkSettings ContainerNetworkSettings `json:"NetworkSettings"`
	RestartCount    int                      `json:"RestartCount"`
}

// ContainerNetworkSettings holds port mappings from the inspect endpoint.
type ContainerNetworkSettings struct {
	Ports map[string][]NetworkPort `json:"Ports"`
}

// NetworkPort represents a single host-to-container port binding.
type NetworkPort struct {
	HostIP   string `json:"HostIp"`
	HostPort string `json:"HostPort"`
}

// ContainerState holds the runtime state of a container.
type ContainerState struct {
	Status     string `json:"Status"`
	StartedAt  string `json:"StartedAt"`
	FinishedAt string `json:"FinishedAt"`
}

// ContainerConfig holds the configuration from the container image.
type ContainerConfig struct {
	Image  string            `json:"Image"`
	Env    []string          `json:"Env"`
	Labels map[string]string `json:"Labels"`
}

// ContainerMount represents a volume or bind mount.
type ContainerMount struct {
	Type        string `json:"Type"`
	Source      string `json:"Source"`
	Destination string `json:"Destination"`
	Mode        string `json:"Mode"`
	RW          bool   `json:"RW"`
}

// InspectContainer returns detailed metadata for a single container.
func (client *Client) InspectContainer(ctx context.Context, containerID string) (*ContainerInspect, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("http://localhost/containers/%s/json", containerID), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("inspect %s: %s", containerID, resp.Status)
	}

	var info ContainerInspect
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &info, nil
}

type Change struct {
	Path string `json:"Path"`
	Kind int    `json:"Kind"`
}

type TopResponse struct {
	Titles    []string   `json:"Titles"`
	Processes [][]string `json:"Processes"`
}

func (client *Client) DiffContainer(ctx context.Context, containerID string) ([]Change, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("http://localhost/containers/%s/changes", containerID), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("diff %s: %s", containerID, resp.Status)
	}
	var changes []Change
	if err := json.NewDecoder(resp.Body).Decode(&changes); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return changes, nil
}

func (client *Client) TopContainer(ctx context.Context, containerID string) (*TopResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("http://localhost/containers/%s/top", containerID), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("top %s: %s", containerID, resp.Status)
	}
	var top TopResponse
	if err := json.NewDecoder(resp.Body).Decode(&top); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &top, nil
}
