package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/agusrdz/tally/config"
	"github.com/agusrdz/tally/state"
)

// Reset handles PreCompact and manual resets.
// It resets estimated tokens and sets the baseline to ctx_restore offset.
func Reset() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tally: config error: %v\n", err)
		return
	}

	// Read session_id from stdin if available (PreCompact hook context).
	sessionID := "manual"
	if data, err := io.ReadAll(os.Stdin); err == nil && len(data) > 0 {
		var payload struct {
			SessionID string `json:"session_id"`
		}
		if json.Unmarshal(data, &payload) == nil && payload.SessionID != "" {
			sessionID = payload.SessionID
		}
	}

	s, err := state.Load(sessionID, cfg.Baselines.SessionStart)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tally: failed to load session: %v\n", err)
		return
	}

	state.Reset(s, cfg.Baselines.CtxRestore)

	if err := state.Save(s); err != nil {
		fmt.Fprintf(os.Stderr, "tally: failed to save session: %v\n", err)
	}
}
