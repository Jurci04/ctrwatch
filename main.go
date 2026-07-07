// ctrwatch is a CLI/TUI for monitoring local containers through the container API.
package main

import (
	"fmt"
	"os"

	"ctrwatch/internal/commands"
)

func printUsage() {
	fmt.Println("Usage: ctrwatch <command> <optional args>")
	fmt.Println("Commands:")
	fmt.Println("  ps [--all] - List containers")
	fmt.Println("  logs <container> [container...] - Show logs for containers")
	fmt.Println("  watch <container> [container...] - TUI split-view for containers")
	fmt.Println("  inspect <container> - Show container details")
	fmt.Println("  stats <container> [container...] - Show CPU/memory stats")
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	var err error

	switch os.Args[1] {
	case "ps":
		err = commands.RunPS(os.Args[2:])
	case "logs":
		err = commands.RunLogs(os.Args[2:])
	case "watch":
		err = commands.RunWatch(os.Args[2:])
	case "inspect":
		err = commands.RunInspect(os.Args[2:])
	case "stats":
		err = commands.RunStats(os.Args[2:])
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
