package main

import (
	"fmt"
	"os"

	"github.com/agusrdz/tally/cmd"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		// No subcommand: PostToolUse handler
		cmd.Root()
		return
	}

	switch os.Args[1] {
	case "reset":
		cmd.Reset()
	case "status":
		cmd.Status()
	case "config":
		cmd.Config(os.Args[2:])
	case "--version", "version":
		cmd.Version(version)
	case "--help", "help", "-h":
		cmd.Help(version)
	case "init", "setup":
		cmd.Init(version)
	case "uninstall":
		cmd.Uninstall(version)
	case "update":
		cmd.Update(version)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\nrun 'tally help'\n", os.Args[1])
		os.Exit(1)
	}
}
