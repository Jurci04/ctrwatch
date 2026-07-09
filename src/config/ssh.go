package config

import (
	"os"
	"path/filepath"
	"strings"
)

func SSHConfigHosts() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(filepath.Join(home, ".ssh", "config"))
	if err != nil {
		return nil, err
	}
	return SSHConfigHostsFrom(string(b)), nil
}

func SSHConfigHostsFrom(text string) []string {
	var hosts []string
	seen := map[string]bool{}
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || strings.ToLower(fields[0]) != "host" {
			continue
		}
		for _, host := range fields[1:] {
			if strings.ContainsAny(host, "*?") || seen[host] {
				continue
			}
			seen[host] = true
			hosts = append(hosts, host)
		}
	}
	return hosts
}
