package commands

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	cfgpkg "ctrwatch/src/config"
)

// RunConfig handles config maintenance commands.
func RunConfig(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: ctrwatch config check|init")
	}

	switch args[0] {
	case "check":
		return runConfigCheck(args[1:])
	case "init":
		return runConfigInit(args[1:], os.Stdin, os.Stdout)
	default:
		return fmt.Errorf("usage: ctrwatch config check|init")
	}
}

func runConfigCheck(args []string) error {
	if len(args) != 0 {
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

func runConfigInit(args []string, in io.Reader, out io.Writer) error {
	fs := flag.NewFlagSet("config init", flag.ContinueOnError)
	output := fs.String("output", cfgpkg.ConfigPath(), "config file to create")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("usage: ctrwatch config init [--output path]")
	}
	if _, err := os.Stat(*output); err == nil {
		return fmt.Errorf("config: %s already exists", *output)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("config: %w", err)
	}

	r := bufio.NewReader(in)
	_, _ = fmt.Fprintf(out, "Creating %s\n", *output)
	readSSH, err := prompt(out, r, "Read ~/.ssh/config for host aliases? [y/N]", "")
	if err != nil {
		return err
	}
	if yes(readSSH) {
		if hosts, err := cfgpkg.SSHConfigHosts(); err == nil && len(hosts) > 0 {
			_, _ = fmt.Fprintf(out, "SSH hosts: %s\n", strings.Join(hosts, ", "))
		}
	}

	host, err := prompt(out, r, "Host [localhost]", "localhost")
	if err != nil {
		return err
	}
	socket, err := prompt(out, r, "Socket [default runtime socket]", "")
	if err != nil {
		return err
	}
	containerText, err := promptRequired(out, r, "Containers (comma-separated)")
	if err != nil {
		return err
	}
	tagText, err := prompt(out, r, "Tags [dev]", "dev")
	if err != nil {
		return err
	}
	containers := cfgpkg.SplitList(containerText)
	tags := cfgpkg.SplitList(tagText)

	cfg := &cfgpkg.Config{Servers: []cfgpkg.Server{{
		Host:       host,
		Socket:     socket,
		Containers: containers,
		Tags:       tags,
	}}}
	if err := cfgpkg.Save(*output, cfg); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "Wrote %s\n", *output)
	return nil
}

func prompt(out io.Writer, r *bufio.Reader, label, fallback string) (string, error) {
	_, _ = fmt.Fprint(out, label+" ")
	text, err := r.ReadString('\n')
	text = strings.TrimSpace(text)
	if text == "" {
		if err != nil && fallback == "" {
			return "", err
		}
		return fallback, nil
	}
	return text, nil
}

func promptRequired(out io.Writer, r *bufio.Reader, label string) (string, error) {
	for {
		text, err := prompt(out, r, label, "")
		if text != "" || err != nil {
			return text, err
		}
		_, _ = fmt.Fprintln(out, "Required.")
	}
}

func yes(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "y" || value == "yes"
}
