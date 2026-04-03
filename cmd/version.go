package cmd

import "fmt"

// Version prints the version string.
func Version(version string) {
	fmt.Printf("tally %s\n", version)
}
