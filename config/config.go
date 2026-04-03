package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Baselines holds per-event token baseline offsets.
type Baselines struct {
	SessionStart int `yaml:"session_start"`
	CtxRestore   int `yaml:"ctx_restore"`
}

// Config is the resolved tally configuration.
type Config struct {
	Enabled               bool               `yaml:"enabled"`
	MaxTallyTokens       int                `yaml:"max_tally_tokens"`
	WarnThresholdPct      float64            `yaml:"warn_threshold_pct"`
	CompactThresholdPct   float64            `yaml:"compact_threshold_pct"`
	ReminderIntervalCalls int                `yaml:"reminder_interval_calls"`
	Baselines             Baselines          `yaml:"baselines"`
	ToolWeights           map[string]float64 `yaml:"tool_weights"`
}

func defaults() *Config {
	return &Config{
		Enabled:               true,
		MaxTallyTokens:       100000,
		WarnThresholdPct:      60,
		CompactThresholdPct:   80,
		ReminderIntervalCalls: 10,
		Baselines: Baselines{
			SessionStart: 10000,
			CtxRestore:   5000,
		},
		ToolWeights: map[string]float64{
			"Read":    1.5,
			"Write":   0.2,
			"Bash":    1.0,
			"Task":    2.0,
			"Edit":    0.3,
			"default": 1.0,
		},
	}
}

// Load reads ~/.config/tally/config.yml, falling back to defaults.
func Load() (*Config, error) {
	cfg := defaults()

	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}

	// Unmarshal on top of defaults so missing keys keep their default values.
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return cfg, err
	}

	// Ensure tool_weights always has a "default" entry.
	if _, ok := cfg.ToolWeights["default"]; !ok {
		cfg.ToolWeights["default"] = 1.0
	}

	return cfg, nil
}

// configPath returns the path to the config file.
func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "tally", "config.yml")
}

// ConfigPath is the exported path accessor.
func ConfigPath() string {
	return configPath()
}
