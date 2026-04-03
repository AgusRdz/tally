package threshold

import (
	"fmt"

	"github.com/agusrdz/tally/config"
	"github.com/agusrdz/tally/state"
)

// Level represents which threshold has been crossed.
type Level int

const (
	LevelNone    Level = iota
	LevelWarn          // warn_threshold_pct crossed
	LevelCompact       // compact_threshold_pct crossed
)

// Check evaluates the session state against config thresholds.
// Returns the message to emit (empty string = silent).
func Check(s *state.Session, cfg *config.Config) string {
	total := s.TotalTokens()
	fillPct := float64(total) / float64(cfg.MaxTallyTokens) * 100.0

	switch level(fillPct, cfg) {
	case LevelCompact:
		// After compact_threshold: only emit every ReminderIntervalCalls tool calls.
		if s.CompactRecommended {
			if s.ToolCalls%cfg.ReminderIntervalCalls != 0 {
				return ""
			}
		}
		s.CompactRecommended = true
		s.WarningsEmitted++
		return fmt.Sprintf(
			"⚡ tally: context ~%.0f%% full (est. %s/%s tokens) | %d tool calls\nRecommend: /compact now, at a clean task boundary, to avoid mid-task compaction.",
			fillPct,
			formatNum(total),
			formatNum(cfg.MaxTallyTokens),
			s.ToolCalls,
		)

	case LevelWarn:
		// Warn threshold: emit once, then silence until compact threshold.
		if s.WarningsEmitted > 0 {
			return ""
		}
		s.WarningsEmitted++
		return fmt.Sprintf(
			"⚠ tally: context ~%.0f%% full (est. %s/%s tokens) | %d tool calls\nConsider wrapping up the current task before the next major operation.",
			fillPct,
			formatNum(total),
			formatNum(cfg.MaxTallyTokens),
			s.ToolCalls,
		)

	default:
		return ""
	}
}

func level(fillPct float64, cfg *config.Config) Level {
	if fillPct >= cfg.CompactThresholdPct {
		return LevelCompact
	}
	if fillPct >= cfg.WarnThresholdPct {
		return LevelWarn
	}
	return LevelNone
}

// FillPct returns the fill percentage for display.
func FillPct(s *state.Session, cfg *config.Config) float64 {
	return float64(s.TotalTokens()) / float64(cfg.MaxTallyTokens) * 100.0
}

func formatNum(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	return fmt.Sprintf("%s,%s", formatNum(n/1000), fmt.Sprintf("%03d", n%1000))
}
