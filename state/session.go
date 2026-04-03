package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Session holds the persistent per-session tally state.
type Session struct {
	SessionID          string         `json:"session_id"`
	Cwd                string         `json:"cwd,omitempty"`
	StartedAt          int64          `json:"started_at"`
	LastUpdated        int64          `json:"last_updated"`
	EstimatedTokens    int            `json:"estimated_tokens"`
	BaselineTokens     int            `json:"baseline_tokens"`
	ToolCalls          int            `json:"tool_calls"`
	ToolBreakdown      map[string]int `json:"tool_breakdown,omitempty"`
	WarnEmitted        bool           `json:"warn_emitted"`
	WarningsEmitted    int            `json:"warnings_emitted"`
	CompactRecommended bool           `json:"compact_recommended"`
	LastReminderCall   int            `json:"last_reminder_call"`
	IsSubagent         bool           `json:"is_subagent"`
}

// AddTokens accumulates tokens for a tool into both the total and breakdown map.
func (s *Session) AddTokens(tool string, tokens int) {
	s.EstimatedTokens += tokens
	if s.ToolBreakdown == nil {
		s.ToolBreakdown = make(map[string]int)
	}
	s.ToolBreakdown[tool] += tokens
}

// SessionID returns the session ID from the environment, falling back to "manual".
func SessionID() string {
	if id := os.Getenv("CLAUDE_SESSION_ID"); id != "" {
		return id
	}
	return "manual"
}

// AllSessions returns all non-manual sessions, sorted most-recently-updated first.
func AllSessions() ([]*Session, error) {
	cacheDir, _ := os.UserCacheDir()
	dir := filepath.Join(cacheDir, "tally")

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var sessions []*Session
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "session_") || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(strings.TrimPrefix(e.Name(), "session_"), ".json")
		if id == "manual" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var s Session
		if err := json.Unmarshal(data, &s); err != nil {
			continue
		}
		sessions = append(sessions, &s)
	}

	// Sort most-recently-updated first.
	for i := 1; i < len(sessions); i++ {
		for j := i; j > 0 && sessions[j].LastUpdated > sessions[j-1].LastUpdated; j-- {
			sessions[j], sessions[j-1] = sessions[j-1], sessions[j]
		}
	}

	return sessions, nil
}

// SessionForCwd returns the most recently updated session whose Cwd matches the given directory.
// Falls back to globally latest if no cwd match found.
func SessionForCwd(cwd string) string {
	sessions, err := AllSessions()
	if err != nil || len(sessions) == 0 {
		return "manual"
	}
	// Prefer exact cwd match, most recent first.
	for _, s := range sessions {
		if s.Cwd == cwd {
			return s.SessionID
		}
	}
	// Fallback: globally latest.
	return sessions[0].SessionID
}

func statePath(sessionID string) string {
	cacheDir, _ := os.UserCacheDir()
	return filepath.Join(cacheDir, "tally", "session_"+sessionID+".json")
}

// Load reads the session state from disk. Returns a new default session if not found.
func Load(sessionID string, sessionStartBaseline int) (*Session, error) {
	path := statePath(sessionID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			isSubagent := os.Getenv("CLAUDE_SUBAGENT") == "1"
			baseline := sessionStartBaseline
			if isSubagent {
				baseline = 0
			}
			return &Session{
				SessionID:      sessionID,
				StartedAt:      time.Now().Unix(),
				LastUpdated:    time.Now().Unix(),
				BaselineTokens: baseline,
				IsSubagent:     isSubagent,
			}, nil
		}
		return nil, err
	}

	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// Save writes the session state atomically (write to .tmp, then rename).
func Save(s *Session) error {
	s.LastUpdated = time.Now().Unix()

	path := statePath(s.SessionID)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Reset resets the session for a PreCompact event.
func Reset(s *Session, ctxRestoreBaseline int) {
	s.EstimatedTokens = 0
	s.BaselineTokens = ctxRestoreBaseline
	s.ToolBreakdown = nil
	s.WarnEmitted = false
	s.WarningsEmitted = 0
	s.CompactRecommended = false
	s.LastReminderCall = 0
	s.ToolCalls = 0
}

// TotalTokens returns baseline + estimated.
func (s *Session) TotalTokens() int {
	return s.BaselineTokens + s.EstimatedTokens
}
