package commands

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"ctrwatch/internal/runtime"

	"gopkg.in/yaml.v3"
)

// Config defines the YAML structure for remote server definitions.
type Config struct {
	Servers []Server `yaml:"servers"`
}

// Server defines a single Docker/Podman endpoint and the containers to target.
type Server struct {
	Host       string   `yaml:"host,omitempty"`
	Socket     string   `yaml:"socket,omitempty"`
	Containers []string `yaml:"containers"`
	Tags       []string `yaml:"tags,omitempty"`
}

func configPath() string {
	if p := os.Getenv("CTRWATCH_CONFIG"); p != "" {
		return p
	}
	return "ctrwatch.yaml"
}

// loadConfig reads and parses a YAML config file.
func loadConfig(path string) (*Config, error) {
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
	for i, s := range cfg.Servers {
		if s.Socket == "" {
			cfg.Servers[i].Socket = "/var/run/docker.sock"
		}
		if len(s.Containers) == 0 {
			return nil, fmt.Errorf("config: server %q has no containers", s.Host)
		}
	}
	return &cfg, nil
}

// sshTunnel spawns an SSH tunnel that forwards a remote Unix socket to a local
// Unix socket. Returns the local socket path and a cleanup function.
//
// ponytail: uses system ssh for auth (keys, agent, config). Upgrade to
// Go-native SSH client if Windows portability or subprocess management matters.
func sshTunnel(host, remoteSocket string) (localSocket string, cleanup func(), err error) {
	if _, err := exec.LookPath("ssh"); err != nil {
		return "", nil, fmt.Errorf("ssh not found in PATH")
	}
	localSocket = filepath.Join(os.TempDir(), fmt.Sprintf("ctrwatch-%d.sock", rand.Int63()))
	cmd := exec.Command("ssh",
		"-L", localSocket+":"+remoteSocket,
		"-N", host,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return "", nil, fmt.Errorf("ssh tunnel: %w", err)
	}
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(localSocket); err == nil {
			return localSocket, func() {
				cmd.Process.Kill()
				cmd.Wait()
				os.Remove(localSocket)
			}, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	cmd.Process.Kill()
	cmd.Wait()
	os.Remove(localSocket)
	return "", nil, fmt.Errorf("ssh tunnel to %s: %s", host, strings.TrimSpace(stderr.String()))
}

// resolveContainers returns container definitions either from CLI args
// or from a config file when the first arg is @tagname.
// The returned cleanup function kills SSH tunnels.
func resolveContainers(args []string) ([]containerDef, func(), error) {
	if len(args) >= 1 && strings.HasPrefix(args[0], "@") {
		return resolveTagged(args[0][1:])
	}
	return parseContainers(args), func() {}, nil
}

// resolveTagged loads the config, matches servers by tag, creates SSH tunnels,
// and returns the matching container definitions.
func resolveTagged(tag string) ([]containerDef, func(), error) {
	cfg, err := loadConfig(configPath())
	if err != nil {
		return nil, nil, err
	}

	var matched []Server
	for _, s := range cfg.Servers {
		for _, t := range s.Tags {
			if t == tag {
				matched = append(matched, s)
				break
			}
		}
	}
	if len(matched) == 0 {
		return nil, nil, fmt.Errorf("no servers with tag %q in %s", tag, configPath())
	}

	clients := map[string]*runtime.Client{}
	var defs []containerDef
	var cleanups []func()

	for _, s := range matched {
		var c *runtime.Client
		if s.Host == "" {
			c = runtime.NewClient()
		} else {
			if _, ok := clients[s.Host]; !ok {
				localSock, cleanup, err := sshTunnel(s.Host, s.Socket)
				if err != nil {
					for _, f := range cleanups {
						f()
					}
					return nil, nil, err
				}
				cleanups = append(cleanups, cleanup)
				c = runtime.NewClientForSocket("unix://" + localSock)
				clients[s.Host] = c
			} else {
				c = clients[s.Host]
			}
		}
		for _, name := range s.Containers {
			defs = append(defs, containerDef{Name: name, Client: c})
		}
	}

	return defs, func() {
		for _, f := range cleanups {
			f()
		}
	}, nil
}
