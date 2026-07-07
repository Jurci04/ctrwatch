package commands

import (
	"fmt"
	"strings"

	cfgpkg "ctrwatch/internal/config"
)

// RunConfig handles config maintenance commands.
func RunConfig(args []string) error {
	if len(args) != 1 || args[0] != "check" {
		return fmt.Errorf("usage: ctrwatch config check")
	}

	path := cfgpkg.ConfigPath()
	cfg, err := cfgpkg.Load(path)
	if err != nil {
		return err
	}
	fmt.Printf("%s: ok (%d servers)\n", path, len(cfg.Servers))
	for _, s := range cfg.Servers {
		host := s.Host
		if cfgpkg.IsLocalHost(host) {
			host = "localhost"
		}
		fmt.Printf("- %s %s containers=%d tags=%s\n", host, s.Socket, len(s.Containers), strings.Join(s.Tags, ","))
	}
	return nil
}
