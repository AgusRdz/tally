package state

import (
	"database/sql"
	"fmt"
	"os"
	"time"
)

// Session holds the persistent per-session tally state.
type Session struct {
	SessionID          string
	Cwd                string
	StartedAt          int64
	LastUpdated        int64
	EstimatedTokens    int
	BaselineTokens     int
	ToolCalls          int
	ToolBreakdown      map[string]int // computed on load from tool_events
	WarnEmitted        bool
	WarningsEmitted    int
	CompactRecommended bool
	LastReminderCall   int
	IsSubagent         bool
}

// TotalTokens returns baseline + estimated.
func (s *Session) TotalTokens() int {
	return s.BaselineTokens + s.EstimatedTokens
}

// SessionID returns the session ID from the environment, falling back to "manual".
func SessionID() string {
	if id := os.Getenv("CLAUDE_SESSION_ID"); id != "" {
		return id
	}
	return "manual"
}

// Load reads the session from the DB. Returns a new default session if not found.
func Load(sessionID string, sessionStartBaseline int) (*Session, error) {
	db, err := DB()
	if err != nil {
		return nil, fmt.Errorf("db unavailable: %w", err)
	}

	s, err := scanSession(db, sessionID)
	if err == sql.ErrNoRows {
		// Create a default session.
		isSubagentEnv := os.Getenv("CLAUDE_SUBAGENT") == "1"
		baseline := sessionStartBaseline
		if isSubagentEnv {
			baseline = 0
		}
		isSubagentInt := 0
		if isSubagentEnv {
			isSubagentInt = 1
		}
		now := time.Now().Unix()
		_, insertErr := db.Exec(`INSERT INTO sessions
			(session_id, cwd, started_at, last_updated, estimated_tokens, baseline_tokens,
			 tool_calls, warn_emitted, warnings_emitted, compact_recommended, last_reminder_call, is_subagent)
			VALUES (?, ?, ?, ?, 0, ?, 0, 0, 0, 0, 0, ?)`,
			sessionID, "", now, now, baseline, isSubagentInt,
		)
		if insertErr != nil {
			return nil, fmt.Errorf("failed to create session: %w", insertErr)
		}
		return &Session{
			SessionID:      sessionID,
			StartedAt:      now,
			LastUpdated:    now,
			BaselineTokens: baseline,
			IsSubagent:     isSubagentEnv,
		}, nil
	}
	if err != nil {
		return nil, err
	}

	s.ToolBreakdown, _ = queryBreakdown(db, sessionID)
	return s, nil
}

// Save upserts the session row. Does NOT write tool_events — use AddTokens for that.
func Save(s *Session) error {
	db, err := DB()
	if err != nil {
		return fmt.Errorf("db unavailable: %w", err)
	}
	s.LastUpdated = time.Now().Unix()

	_, err = db.Exec(`INSERT INTO sessions
		(session_id, cwd, started_at, last_updated, estimated_tokens, baseline_tokens,
		 tool_calls, warn_emitted, warnings_emitted, compact_recommended, last_reminder_call, is_subagent)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(session_id) DO UPDATE SET
		  cwd = excluded.cwd,
		  last_updated = excluded.last_updated,
		  estimated_tokens = excluded.estimated_tokens,
		  baseline_tokens = excluded.baseline_tokens,
		  tool_calls = excluded.tool_calls,
		  warn_emitted = excluded.warn_emitted,
		  warnings_emitted = excluded.warnings_emitted,
		  compact_recommended = excluded.compact_recommended,
		  last_reminder_call = excluded.last_reminder_call,
		  is_subagent = excluded.is_subagent`,
		s.SessionID, s.Cwd, s.StartedAt, s.LastUpdated,
		s.EstimatedTokens, s.BaselineTokens, s.ToolCalls,
		boolToInt(s.WarnEmitted), s.WarningsEmitted, boolToInt(s.CompactRecommended),
		s.LastReminderCall, boolToInt(s.IsSubagent),
	)
	return err
}

// AddTokens inserts a tool_event row and updates the in-memory session.
func AddTokens(s *Session, tool string, tokens int) error {
	db, err := DB()
	if err != nil {
		return fmt.Errorf("db unavailable: %w", err)
	}
	_, err = db.Exec(`INSERT INTO tool_events (session_id, tool_name, tokens, called_at) VALUES (?, ?, ?, ?)`,
		s.SessionID, tool, tokens, time.Now().Unix())
	if err != nil {
		return err
	}
	s.EstimatedTokens += tokens
	if s.ToolBreakdown == nil {
		s.ToolBreakdown = make(map[string]int)
	}
	s.ToolBreakdown[tool] += tokens
	return nil
}

// Reset resets session fields in memory for a PreCompact event. Call Save to persist.
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

// SessionForCwd returns the most recently updated session whose Cwd matches the given directory.
// Falls back to globally latest if no cwd match found.
func SessionForCwd(cwd string) string {
	db, err := DB()
	if err != nil {
		return "manual"
	}
	var sessionID string
	err = db.QueryRow(`SELECT session_id FROM sessions WHERE cwd = ? AND session_id != 'manual'
		ORDER BY last_updated DESC LIMIT 1`, cwd).Scan(&sessionID)
	if err == nil && sessionID != "" {
		return sessionID
	}
	err = db.QueryRow(`SELECT session_id FROM sessions WHERE session_id != 'manual'
		ORDER BY last_updated DESC LIMIT 1`).Scan(&sessionID)
	if err == nil && sessionID != "" {
		return sessionID
	}
	return "manual"
}

// AllSessions returns all non-manual sessions ordered by last_updated DESC, with ToolBreakdown populated.
func AllSessions() ([]*Session, error) {
	db, err := DB()
	if err != nil {
		return nil, fmt.Errorf("db unavailable: %w", err)
	}

	rows, err := db.Query(`SELECT session_id, cwd, started_at, last_updated,
		estimated_tokens, baseline_tokens, tool_calls,
		warn_emitted, warnings_emitted, compact_recommended, last_reminder_call, is_subagent
		FROM sessions WHERE session_id != 'manual'
		ORDER BY last_updated DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		var s Session
		var warnEmitted, compactRec, isSubagent int
		if err := rows.Scan(
			&s.SessionID, &s.Cwd, &s.StartedAt, &s.LastUpdated,
			&s.EstimatedTokens, &s.BaselineTokens, &s.ToolCalls,
			&warnEmitted, &s.WarningsEmitted, &compactRec, &s.LastReminderCall, &isSubagent,
		); err != nil {
			continue
		}
		s.WarnEmitted = warnEmitted != 0
		s.CompactRecommended = compactRec != 0
		s.IsSubagent = isSubagent != 0
		sessions = append(sessions, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, s := range sessions {
		s.ToolBreakdown, _ = queryBreakdown(db, s.SessionID)
	}
	return sessions, nil
}

// scanSession reads a single session row from the DB.
func scanSession(db *sql.DB, sessionID string) (*Session, error) {
	var s Session
	var warnEmitted, compactRec, isSubagent int
	err := db.QueryRow(`SELECT session_id, cwd, started_at, last_updated,
		estimated_tokens, baseline_tokens, tool_calls,
		warn_emitted, warnings_emitted, compact_recommended, last_reminder_call, is_subagent
		FROM sessions WHERE session_id = ?`, sessionID).Scan(
		&s.SessionID, &s.Cwd, &s.StartedAt, &s.LastUpdated,
		&s.EstimatedTokens, &s.BaselineTokens, &s.ToolCalls,
		&warnEmitted, &s.WarningsEmitted, &compactRec, &s.LastReminderCall, &isSubagent,
	)
	if err != nil {
		return nil, err
	}
	s.WarnEmitted = warnEmitted != 0
	s.CompactRecommended = compactRec != 0
	s.IsSubagent = isSubagent != 0
	return &s, nil
}

// queryBreakdown aggregates tool_events into a token-per-tool map.
func queryBreakdown(db *sql.DB, sessionID string) (map[string]int, error) {
	rows, err := db.Query(`SELECT tool_name, SUM(tokens) FROM tool_events WHERE session_id = ? GROUP BY tool_name`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[string]int)
	for rows.Next() {
		var tool string
		var tokens int
		if err := rows.Scan(&tool, &tokens); err != nil {
			continue
		}
		m[tool] = tokens
	}
	return m, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
