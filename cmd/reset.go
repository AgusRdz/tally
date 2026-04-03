package cmd

import (
	"fmt"
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

	sessionID := state.SessionID()
	s, err := state.Load(sessionID, cfg.Baselines.SessionStart)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tally: failed to load session: %v\n", err)
		return
	}

	state.Reset(s, cfg.Baselines.CtxRestore)

	if err := state.Save(s); err != nil {
		fmt.Fprintf(os.Stderr, "tally: failed to save session: %v\n", err)
		return
	}

	// PreCompact hook output must be action:continue.
	respond("")
}
