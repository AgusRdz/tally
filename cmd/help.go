package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/agusrdz/tally/hooks"
	"github.com/mattn/go-isatty"
)

// Help prints usage information.
func Help(version string) {
	const colW = 32
	section := func(name string) string { return bold(cyan(name)) + "\n" }
	row := func(cmd, desc string) string {
		return fmt.Sprintf("  %-*s%s\n", colW, cmd, dim(desc))
	}

	var b strings.Builder

	b.WriteString(fmt.Sprintf("%s %s — context tally tracker for Claude Code\n\n", bold("tally"), version))

	b.WriteString(bold("Usage") + "\n")
	b.WriteString(row("tally", "PostToolUse hook: accumulate token estimate, emit warnings"))
	b.WriteString(row("tally <subcommand>", "Run a management subcommand"))
	b.WriteString("\n")

	b.WriteString(section("Setup"))
	b.WriteString(row("init", "Install Claude Code PostToolUse + PreCompact hooks"))
	b.WriteString(row("uninstall", "Remove hooks from ~/.claude/settings.json"))
	b.WriteString("\n")

	b.WriteString(section("Monitoring"))
	b.WriteString(row("status", "Show current session estimate and fill %"))
	b.WriteString(row("reset", "Reset session (also called by PreCompact hook)"))
	b.WriteString("\n")

	b.WriteString(section("Config"))
	b.WriteString(row("config show", "Show resolved config as YAML"))
	b.WriteString("\n")

	b.WriteString(section("Other"))
	b.WriteString(row("version", "Show version"))
	b.WriteString(row("help", "Show this help"))

	fmt.Print(b.String())
}

// Init installs the PostToolUse and PreCompact hooks.
func Init(version string) {
	hooks.Install(version)
}

// Uninstall removes the hooks and config.
func Uninstall(version string) {
	hooks.Uninstall()
	home, _ := os.UserHomeDir()
	fmt.Printf("  config location: %s/.config/tally/config.yml\n", home)
	fmt.Println("\nbinary not removed — delete manually or via your package manager")
}

func isTTY() bool {
	return isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
}

func bold(s string) string {
	if !isTTY() {
		return s
	}
	return "\033[1m" + s + "\033[0m"
}

func dim(s string) string {
	if !isTTY() {
		return s
	}
	return "\033[2m" + s + "\033[0m"
}

func cyan(s string) string {
	if !isTTY() {
		return s
	}
	return "\033[36m" + s + "\033[0m"
}
