package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/agusrdz/tally/config"
	"github.com/agusrdz/tally/estimate"
	"github.com/agusrdz/tally/state"
	"github.com/agusrdz/tally/threshold"
)

// hookInput is the PostToolUse stdin payload from Claude Code.
type hookInput struct {
	SessionID    string          `json:"session_id"`
	Cwd          string          `json:"cwd"`
	ToolName     string          `json:"tool_name"`
	ToolResponse json.RawMessage `json:"tool_response"`
}

// Root handles the PostToolUse hook invocation (no subcommand, reads stdin).
func Root() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tally: config error: %v\n", err)
		respond("")
		return
	}

	if !cfg.Enabled {
		respond("")
		return
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tally: failed to read stdin: %v\n", err)
		respond("")
		return
	}

	var input hookInput
	if err := json.Unmarshal(data, &input); err != nil {
		fmt.Fprintf(os.Stderr, "tally: failed to parse stdin: %v\n", err)
		respond("")
		return
	}

	sessionID := input.SessionID
	if sessionID == "" {
		sessionID = "manual"
	}
	s, err := state.Load(sessionID, cfg.Baselines.SessionStart)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tally: failed to load session: %v\n", err)
		respond("")
		return
	}

	// Capture cwd on first tool call of the session.
	if s.Cwd == "" && input.Cwd != "" {
		s.Cwd = input.Cwd
	}

	// Accumulate: estimate tokens from the tool_response field bytes.
	resultBytes := len(input.ToolResponse)
	tokens := estimate.Tokens(input.ToolName, resultBytes, cfg.ToolWeights)
	s.EstimatedTokens += tokens
	s.ToolCalls++

	// Evaluate thresholds and get message (may be empty).
	msg := threshold.Check(s, cfg)

	if err := state.Save(s); err != nil {
		fmt.Fprintf(os.Stderr, "tally: failed to save session: %v\n", err)
	}

	respond(msg)
}

func respond(output string) {
	resp := map[string]string{"action": "continue", "output": output}
	data, _ := json.Marshal(resp)
	fmt.Println(string(data))
}
