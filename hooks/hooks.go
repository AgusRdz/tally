package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func claudeSettingsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "settings.json")
}

func loadSettings() map[string]interface{} {
	data, err := os.ReadFile(claudeSettingsPath())
	if err != nil {
		return map[string]interface{}{}
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return map[string]interface{}{}
	}
	return m
}

func saveSettings(m map[string]interface{}) error {
	path := claudeSettingsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func exePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

// Install registers tally as a Claude Code PostToolUse and PreCompact hook.
func Install(version string) {
	exe, err := exePath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tally: failed to get executable path: %v\n", err)
		os.Exit(1)
	}

	exeFwd := strings.ReplaceAll(exe, "\\", "/")

	settings := loadSettings()
	hooksMap := getOrCreateMap(settings, "hooks")

	// PostToolUse: run tally (no matcher = all tools)
	postToolUse := getOrCreateSlice(hooksMap, "PostToolUse")
	postToolUse = removeOurEntries(postToolUse, "tally")
	postToolUseEntry := map[string]interface{}{
		"hooks": []interface{}{
			map[string]interface{}{
				"type":    "command",
				"command": fmt.Sprintf("%q", exeFwd),
			},
		},
	}
	postToolUse = append(postToolUse, postToolUseEntry)
	hooksMap["PostToolUse"] = postToolUse

	// PreCompact: run tally reset
	preCompact := getOrCreateSlice(hooksMap, "PreCompact")
	preCompact = removeOurEntries(preCompact, "tally")
	preCompactEntry := map[string]interface{}{
		"hooks": []interface{}{
			map[string]interface{}{
				"type":    "command",
				"command": fmt.Sprintf("%q reset", exeFwd),
			},
		},
	}
	preCompact = append(preCompact, preCompactEntry)
	hooksMap["PreCompact"] = preCompact

	settings["hooks"] = hooksMap

	if err := saveSettings(settings); err != nil {
		fmt.Fprintf(os.Stderr, "tally: failed to write settings: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("tally %s hooks installed\n", version)
	fmt.Printf("  binary:  %s\n", exe)
	fmt.Printf("  config:  %s\n", claudeSettingsPath())
	fmt.Printf("  hooks:   PostToolUse (all tools), PreCompact\n")
}

// Uninstall removes the tally hooks.
func Uninstall() {
	settings := loadSettings()

	hooksMap, ok := settings["hooks"].(map[string]interface{})
	if !ok {
		fmt.Println("no hooks found")
		return
	}

	removed := 0

	for _, event := range []string{"PostToolUse", "PreCompact"} {
		entries, ok := hooksMap[event].([]interface{})
		if !ok {
			continue
		}
		before := len(entries)
		entries = removeOurEntries(entries, "tally")
		hooksMap[event] = entries
		removed += before - len(entries)
	}

	settings["hooks"] = hooksMap

	if removed == 0 {
		fmt.Println("no tally hooks found")
		return
	}

	if err := saveSettings(settings); err != nil {
		fmt.Fprintf(os.Stderr, "tally: failed to write settings: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("tally hooks removed from ~/.claude/settings.json")
}

// IsInstalled checks if the PostToolUse hook is installed.
func IsInstalled() bool {
	settings := loadSettings()
	hooksMap, ok := settings["hooks"].(map[string]interface{})
	if !ok {
		return false
	}
	entries, ok := hooksMap["PostToolUse"].([]interface{})
	if !ok {
		return false
	}
	for _, entry := range entries {
		if entryContains(entry, "tally") {
			return true
		}
	}
	return false
}

func entryContains(entry interface{}, needle string) bool {
	m, ok := entry.(map[string]interface{})
	if !ok {
		return false
	}
	hooksList, ok := m["hooks"].([]interface{})
	if !ok {
		return false
	}
	for _, h := range hooksList {
		hm, ok := h.(map[string]interface{})
		if !ok {
			continue
		}
		cmd, _ := hm["command"].(string)
		if strings.Contains(cmd, needle) {
			return true
		}
	}
	return false
}

func getOrCreateMap(m map[string]interface{}, key string) map[string]interface{} {
	v, ok := m[key].(map[string]interface{})
	if !ok {
		v = map[string]interface{}{}
	}
	return v
}

func getOrCreateSlice(m map[string]interface{}, key string) []interface{} {
	v, ok := m[key].([]interface{})
	if !ok {
		v = []interface{}{}
	}
	return v
}

func removeOurEntries(entries []interface{}, needle string) []interface{} {
	var result []interface{}
	for _, entry := range entries {
		if !entryContains(entry, needle) {
			result = append(result, entry)
		}
	}
	return result
}
