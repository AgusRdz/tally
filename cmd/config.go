package cmd

import (
	"fmt"
	"os"

	"github.com/agusrdz/tally/config"
	"gopkg.in/yaml.v3"
)

// Config handles the `tally config` subcommand.
func Config(args []string) {
	if len(args) == 0 || args[0] != "show" {
		fmt.Fprintf(os.Stderr, "usage: tally config show\n")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tally: config error: %v\n", err)
		os.Exit(1)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tally: failed to marshal config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("# %s\n", config.ConfigPath())
	fmt.Print(string(data))
}
