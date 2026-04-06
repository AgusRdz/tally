package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/agusrdz/tally/state"
)

type projectStats struct {
	name          string
	sessionCount  int
	totalTokens   int
	totalCalls    int
	toolBreakdown map[string]int
}

func History() {
	sessions, err := state.AllSessions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tally: error loading sessions: %v\n", err)
		os.Exit(1)
	}

	// Filter out subagents
	var filtered []*state.Session
	for _, s := range sessions {
		if !s.IsSubagent {
			filtered = append(filtered, s)
		}
	}

	if len(filtered) == 0 {
		fmt.Print("  no sessions recorded yet.\n")
		return
	}

	// Group by project
	projects := map[string]*projectStats{}
	for _, s := range filtered {
		name := "unknown"
		if s.Cwd != "" {
			name = filepath.Base(s.Cwd)
		}
		p, ok := projects[name]
		if !ok {
			p = &projectStats{name: name, toolBreakdown: map[string]int{}}
			projects[name] = p
		}
		p.sessionCount++
		p.totalTokens += s.TotalTokens()
		p.totalCalls += s.ToolCalls
		for tool, tokens := range s.ToolBreakdown {
			p.toolBreakdown[tool] += tokens
		}
	}

	// Sort projects by total tokens desc
	sorted := make([]*projectStats, 0, len(projects))
	for _, p := range projects {
		sorted = append(sorted, p)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].totalTokens > sorted[j].totalTokens
	})

	// Totals
	totalSessions := len(filtered)
	grandTotal := 0
	for _, p := range sorted {
		grandTotal += p.totalTokens
	}

	fmt.Println("tally history")
	fmt.Println()
	fmt.Printf("  %d projects  |  %d sessions  |  %s tokens (est.)\n",
		len(sorted), totalSessions, formatNum(grandTotal))
	fmt.Println()
	fmt.Printf("  %-18s %8s  %10s  %7s   %s\n", "project", "sessions", "tokens", "calls", "top tool")
	fmt.Println("  " + "──────────────────────────────────────────────────────────")

	for _, p := range sorted {
		topTool := topToolName(p.toolBreakdown)
		fmt.Printf("  %-18s %8d  %10s  %7d   %s\n",
			p.name, p.sessionCount, formatNum(p.totalTokens), p.totalCalls, topTool)
	}

	// Aggregate all tool breakdown across all sessions (using EstimatedTokens only for %)
	allTools := map[string]int{}
	estimatedGrand := 0
	for _, s := range filtered {
		estimatedGrand += s.EstimatedTokens
		for tool, tokens := range s.ToolBreakdown {
			allTools[tool] += tokens
		}
	}

	// Sort tools by tokens desc, top 5
	type toolEntry struct {
		name   string
		tokens int
	}
	toolList := make([]toolEntry, 0, len(allTools))
	for t, tok := range allTools {
		toolList = append(toolList, toolEntry{t, tok})
	}
	sort.Slice(toolList, func(i, j int) bool {
		return toolList[i].tokens > toolList[j].tokens
	})
	if len(toolList) > 5 {
		toolList = toolList[:5]
	}

	fmt.Println()
	fmt.Println("  top tools (all time):")
	for _, te := range toolList {
		pct := 0.0
		if estimatedGrand > 0 {
			pct = float64(te.tokens) / float64(estimatedGrand) * 100
		}
		fmt.Printf("    %-12s %s tokens  (%.0f%%)\n", te.name, formatNum(te.tokens), pct)
	}
}

func topToolName(breakdown map[string]int) string {
	best := ""
	bestVal := -1
	for tool, tokens := range breakdown {
		if tokens > bestVal {
			bestVal = tokens
			best = tool
		}
	}
	return best
}
