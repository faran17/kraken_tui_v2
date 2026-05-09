package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config represents the v2 configuration settings.
type Config struct {
	GeminiAPIKey string    `json:"gemini_api_key"`
	Theme        string    `json:"theme"`
	DarkMode     bool      `json:"dark_mode"`
	PanelWidths  []float64 `json:"panel_widths"`
	TermHeight   int       `json:"term_height"`
}

// Default returns the default configuration.
func Default() *Config {
	return &Config{
		Theme:       "Ocean",
		DarkMode:    true,
		PanelWidths: []float64{0.334, 0.333, 0.333},
		TermHeight:  12,
	}
}

// GetConfigPath returns the absolute path to the v2 config file.
func GetConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	dir := filepath.Join(home, ".kraken")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("create config dir: %w", err)
		}
	}
	return filepath.Join(dir, "v2_config.json"), nil
}

// Load reads the configuration from disk.
func Load() (*Config, error) {
	path, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		cfg := Default()
		// Try to migrate API key from env if available
		if envKey := os.Getenv("GEMINI_API_KEY"); envKey != "" {
			cfg.GeminiAPIKey = envKey
		}
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := Default()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return cfg, nil
}

// Save writes the configuration to disk.
func (c *Config) Save() error {
	path, err := GetConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}
