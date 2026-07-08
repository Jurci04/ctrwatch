// ctrwatch is a CLI/TUI for monitoring local containers through the container API.
package main

import (
	"fmt"
	"os"

	"ctrwatch/src/commands"
)

func printUsage() {
	fmt.Println("Usage: ctrwatch <command> [args]")
	fmt.Println("Commands:")
	fmt.Println("  ps [--all] [@tag]          List containers")
	fmt.Println("  logs [--tail N] [--since D] <container>... | @tag")
	fmt.Println("  inspect [--json] <container> | @tag")
	fmt.Println("  stats <container>... | @tag  Show CPU/memory stats")
	fmt.Println("  import [--tag TAG] [file]    Import Compose/Podman/Kube names")
	fmt.Println("  import --from-running        Import running local containers")
	fmt.Println("  config check                 Validate config")
	fmt.Println()
	fmt.Println("Default (no command) opens the TUI with local containers and a server browser.")
}

func main() {
	if len(os.Args) < 2 {
		if err := commands.RunDefaultTUI(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	var err error

	switch os.Args[1] {
	case "ps":
		err = commands.RunPS(os.Args[2:])
	case "logs":
		err = commands.RunLogs(os.Args[2:])
	case "inspect":
		err = commands.RunInspect(os.Args[2:])
	case "stats":
		err = commands.RunStats(os.Args[2:])
	case "import":
		err = commands.RunImport(os.Args[2:])
	case "config":
		err = commands.RunConfig(os.Args[2:])
	case "help":
		printUsage()
		os.Exit(0)
	default:
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
