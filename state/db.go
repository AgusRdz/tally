package state

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	_ "modernc.org/sqlite"
)

var (
	dbOnce     sync.Once
	dbInstance *sql.DB
	dbErr      error
)

const schema = `
CREATE TABLE IF NOT EXISTS sessions (
    session_id TEXT PRIMARY KEY,
    cwd TEXT,
    started_at INTEGER NOT NULL,
    last_updated INTEGER NOT NULL,
    estimated_tokens INTEGER NOT NULL DEFAULT 0,
    baseline_tokens INTEGER NOT NULL DEFAULT 0,
    tool_calls INTEGER NOT NULL DEFAULT 0,
    warn_emitted INTEGER NOT NULL DEFAULT 0,
    warnings_emitted INTEGER NOT NULL DEFAULT 0,
    compact_recommended INTEGER NOT NULL DEFAULT 0,
    last_reminder_call INTEGER NOT NULL DEFAULT 0,
    is_subagent INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS tool_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    tool_name TEXT NOT NULL,
    tokens INTEGER NOT NULL,
    called_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_tool_events_session ON tool_events(session_id);
CREATE INDEX IF NOT EXISTS idx_sessions_cwd ON sessions(cwd);
CREATE INDEX IF NOT EXISTS idx_sessions_updated ON sessions(last_updated DESC);
`

// DB returns the package-level singleton *sql.DB, opening and initializing it on first call.
func DB() (*sql.DB, error) {
	dbOnce.Do(func() {
		cacheDir, err := os.UserCacheDir()
		if err != nil {
			dbErr = fmt.Errorf("cannot locate cache dir: %w", err)
			return
		}
		dir := filepath.Join(cacheDir, "tally")
		if err := os.MkdirAll(dir, 0755); err != nil {
			dbErr = fmt.Errorf("cannot create cache dir: %w", err)
			return
		}
		dbPath := filepath.Join(dir, "tally.db")
		dsn := "file:" + dbPath + "?_busy_timeout=5000&_journal_mode=WAL"
		db, err := sql.Open("sqlite", dsn)
		if err != nil {
			dbErr = fmt.Errorf("cannot open sqlite: %w", err)
			return
		}
		// Allow concurrent reads from multiple hook invocations.
		db.SetMaxOpenConns(1)
		if _, err := db.Exec(schema); err != nil {
			dbErr = fmt.Errorf("schema init failed: %w", err)
			db.Close()
			return
		}
		dbInstance = db
		// Migrate any legacy JSON session files.
		migrateJSON(db, dir)
	})
	return dbInstance, dbErr
}

// migrateJSON reads legacy session_*.json files and inserts them into the DB.
// On success each file is renamed to .json.migrated (never deleted).
func migrateJSON(db *sql.DB, dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, "session_") || !strings.HasSuffix(name, ".json") {
			continue
		}
		fullPath := filepath.Join(dir, name)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}
		var s struct {
			SessionID          string         `json:"session_id"`
			Cwd                string         `json:"cwd"`
			StartedAt          int64          `json:"started_at"`
			LastUpdated        int64          `json:"last_updated"`
			EstimatedTokens    int            `json:"estimated_tokens"`
			BaselineTokens     int            `json:"baseline_tokens"`
			ToolCalls          int            `json:"tool_calls"`
			ToolBreakdown      map[string]int `json:"tool_breakdown"`
			WarnEmitted        bool           `json:"warn_emitted"`
			WarningsEmitted    int            `json:"warnings_emitted"`
			CompactRecommended bool           `json:"compact_recommended"`
			LastReminderCall   int            `json:"last_reminder_call"`
			IsSubagent         bool           `json:"is_subagent"`
		}
		if err := json.Unmarshal(data, &s); err != nil {
			continue
		}
		if s.SessionID == "" {
			continue
		}
		// Skip if already in DB.
		var count int
		_ = db.QueryRow("SELECT COUNT(*) FROM sessions WHERE session_id = ?", s.SessionID).Scan(&count)
		if count > 0 {
			// Already migrated — still rename if possible.
			_ = os.Rename(fullPath, fullPath+".migrated")
			continue
		}
		tx, err := db.Begin()
		if err != nil {
			continue
		}
		warnEmitted := 0
		if s.WarnEmitted {
			warnEmitted = 1
		}
		compactRec := 0
		if s.CompactRecommended {
			compactRec = 1
		}
		isSubagent := 0
		if s.IsSubagent {
			isSubagent = 1
		}
		_, err = tx.Exec(`INSERT INTO sessions
			(session_id, cwd, started_at, last_updated, estimated_tokens, baseline_tokens,
			 tool_calls, warn_emitted, warnings_emitted, compact_recommended, last_reminder_call, is_subagent)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			s.SessionID, s.Cwd, s.StartedAt, s.LastUpdated,
			s.EstimatedTokens, s.BaselineTokens, s.ToolCalls,
			warnEmitted, s.WarningsEmitted, compactRec, s.LastReminderCall, isSubagent,
		)
		if err != nil {
			_ = tx.Rollback()
			continue
		}
		// Insert synthetic tool_events for each breakdown entry.
		for tool, tokens := range s.ToolBreakdown {
			_, _ = tx.Exec(`INSERT INTO tool_events (session_id, tool_name, tokens, called_at) VALUES (?, ?, ?, ?)`,
				s.SessionID, tool, tokens, s.LastUpdated)
		}
		if err := tx.Commit(); err != nil {
			continue
		}
		_ = os.Rename(fullPath, fullPath+".migrated")
	}
}
