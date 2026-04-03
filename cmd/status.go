package cmd

import (
	"fmt"
	"os"

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

	fmt.Printf("tally status\n")
	fmt.Printf("  session:    %s\n", s.SessionID)
	fmt.Printf("  estimated:  %s tokens (est.)\n", formatNum(s.EstimatedTokens))
	fmt.Printf("  baseline:   %s tokens\n", formatNum(s.BaselineTokens))
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
