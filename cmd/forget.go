package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/agusrdz/tally/state"
)

// Forget removes all sessions for a given project name from the DB.
func Forget(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: tally forget <project>\n")
		os.Exit(1)
	}
	project := args[0]

	force := false
	for _, a := range args[1:] {
		if a == "--force" || a == "-f" {
			force = true
		}
	}

	if !force {
		fmt.Printf("remove all sessions for %q? [y/N] ", project)
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		if strings.ToLower(strings.TrimSpace(scanner.Text())) != "y" {
			fmt.Println("cancelled.")
			return
		}
	}

	n, err := state.DeleteProject(project)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tally: %v\n", err)
		os.Exit(1)
	}
	if n == 0 {
		fmt.Printf("no sessions found for %q\n", project)
		return
	}
	fmt.Printf("removed %d session(s) for %q\n", n, project)
}
