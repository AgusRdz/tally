package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/agusrdz/tally/config"
	"github.com/agusrdz/tally/state"
	"github.com/agusrdz/tally/threshold"
)

// Status prints the current session estimate to stdout (human-readable, not hook JSON).
func Status(args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tally: config error: %v\n", err)
		os.Exit(1)
	}

	sessionID := state.LatestSessionID()
	for _, a := range args {
		if a == "--manual" {
			sessionID = "manual"
			break
		}
	}

	s, err := state.Load(sessionID, cfg.Baselines.SessionStart)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tally: failed to load session: %v\n", err)
		os.Exit(1)
	}

	total := s.TotalTokens()
	fillPct := threshold.FillPct(s, cfg)
	started := time.Unix(s.StartedAt, 0).Format("2006-01-02 15:04")

	project := s.Cwd
	if project == "" {
		project = "unknown"
	} else {
		project = filepath.Base(project)
	}

	baselineLabel := "session start"
	if s.BaselineTokens < 10000 {
		baselineLabel = "post-compact"
	}

	fmt.Printf("tally status\n")
	fmt.Printf("  project:    %s\n", project)
	fmt.Printf("  started:    %s\n", started)
	fmt.Printf("  session:    %s\n", s.SessionID)
	fmt.Printf("  estimated:  %s tokens (est.)\n", formatNum(s.EstimatedTokens))
	fmt.Printf("  baseline:   %s tokens (%s)\n", formatNum(s.BaselineTokens), baselineLabel)
	fmt.Printf("  total:      %s tokens (~%.0f%% of %s)\n",
		formatNum(total), fillPct, formatNum(cfg.MaxTallyTokens))
	fmt.Printf("  tool calls: %d\n", s.ToolCalls)
	fmt.Printf("  warnings:   %d\n", s.WarningsEmitted)
}

func formatNum(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	return fmt.Sprintf("%s,%s", formatNum(n/1000), fmt.Sprintf("%03d", n%1000))
}
