package commands

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"ctrwatch/internal/format"
	"ctrwatch/internal/runtime"
)

// RunPS lists containers in a formatted table.
func RunPS(args []string) error {
	fs := flag.NewFlagSet("ps", flag.ContinueOnError)
	all := fs.Bool("all", false, "show all containers (default shows only running)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client := runtime.NewClient()
	containers, err := client.ListContainers(ctx, *all)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing containers: %v\n", err)
		return err
	}

	format.PrintContainers(containers, client.SocketPath)
	return nil
}
