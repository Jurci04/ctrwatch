package config

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config defines the YAML structure for remote server definitions.
type Config struct {
	Interval string   `yaml:"interval,omitempty"`
	Servers  []Server `yaml:"servers"`
}

func SplitList(value string) []string {
	var values []string
	for _, part := range strings.Split(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			values = append(values, part)
		}
	}
	return values
}

// PollInterval returns the configured polling interval or the default 10s.
func (c *Config) PollInterval() time.Duration {
	if c.Interval == "" {
		return 10 * time.Second
	}
	d, err := time.ParseDuration(c.Interval)
	if err != nil {
		return 10 * time.Second
	}
	return d
}

// Server defines a single Docker/Podman endpoint and the containers to target.
type Server struct {
	Host       string   `yaml:"host,omitempty"`
	Socket     string   `yaml:"socket,omitempty"`
	Containers []string `yaml:"containers"`
	Tags       []string `yaml:"tags,omitempty"`
}

// ConfigPath returns the config file path from CTRWATCH_CONFIG or the default.
func ConfigPath() string {
	if p := os.Getenv("CTRWATCH_CONFIG"); p != "" {
		return p
	}
	if _, err := os.Stat("ctrwatch.yaml"); err == nil {
		return "ctrwatch.yaml"
	}
	if _, err := os.Stat("settings.yaml"); err == nil {
		return "settings.yaml"
	}
	return "ctrwatch.yaml"
}

// Load reads and parses a YAML config file.
func Load(path string) (*Config, error) {
	f, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(f, &cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	if len(cfg.Servers) == 0 {
		return nil, fmt.Errorf("config: no servers defined")
	}
	for _, s := range cfg.Servers {
		if len(s.Containers) == 0 {
			return nil, fmt.Errorf("config: server %q has no containers", s.Host)
		}
	}
	return &cfg, nil
}

// IsLocalHost reports whether host names the local runtime.
func IsLocalHost(host string) bool {
	return host == "" || host == "localhost" || host == "127.0.0.1"
}

// Save writes config as YAML.
func Save(path string, cfg *Config) error {
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	return nil
}

// MergeServer adds or replaces a server matching host, socket, and tags.
func MergeServer(cfg *Config, server Server) {
	if IsLocalHost(server.Host) {
		server.Host = "localhost"
	}
	slices.Sort(server.Containers)
	server.Containers = slices.Compact(server.Containers)
	for i, s := range cfg.Servers {
		host := s.Host
		if IsLocalHost(host) {
			host = "localhost"
		}
		if host == server.Host && s.Socket == server.Socket && slices.Equal(s.Tags, server.Tags) {
			cfg.Servers[i].Host = server.Host
			cfg.Servers[i].Containers = server.Containers
			return
		}
	}
	cfg.Servers = append(cfg.Servers, server)
}
