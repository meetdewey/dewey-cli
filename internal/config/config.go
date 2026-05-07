// Package config handles ~/.dewey/config.toml and ~/.dewey/state.toml plus
// env-var resolution.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const (
	EnvAPIKey    = "DEWEY_API_KEY"
	EnvBaseURL   = "DEWEY_BASE_URL"
	EnvTelemetry = "DEWEY_TELEMETRY"
	EnvNoColor   = "NO_COLOR"
)

type Config struct {
	DefaultCollection string `toml:"default_collection,omitempty"`
	ProjectID         string `toml:"project_id,omitempty"`
	OrgID             string `toml:"org_id,omitempty"`
	Output            string `toml:"output,omitempty"` // "human" | "json"
	Color             string `toml:"color,omitempty"`  // "auto" | "always" | "never"
	BaseURL           string `toml:"base_url,omitempty"`
}

type State struct {
	LastCollection      string   `toml:"last_collection,omitempty"`
	RecentDocumentIDs   []string `toml:"recent_document_ids,omitempty"`
	TelemetryNoticeShown bool    `toml:"telemetry_notice_shown,omitempty"`
}

func dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".dewey"), nil
}

func ConfigPath() (string, error) {
	d, err := dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "config.toml"), nil
}

func StatePath() (string, error) {
	d, err := dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "state.toml"), nil
}

// Load reads ~/.dewey/config.toml, returning an empty Config if missing.
func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}
	c := &Config{}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, err
	}
	if err := toml.Unmarshal(raw, c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return c, nil
}

// Save writes the config back to disk, creating ~/.dewey if needed.
func (c *Config) Save() error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := toml.NewEncoder(f)
	return enc.Encode(c)
}

// LoadState reads ~/.dewey/state.toml, returning an empty State if missing.
func LoadState() (*State, error) {
	path, err := StatePath()
	if err != nil {
		return nil, err
	}
	s := &State{}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	if err := toml.Unmarshal(raw, s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return s, nil
}

// SaveState persists machine-managed state.
func (s *State) Save() error {
	path, err := StatePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(s)
}

// ResolveAPIKey returns the API key from --flag, env var, or "".
func ResolveAPIKey(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	return os.Getenv(EnvAPIKey)
}

// ResolveBaseURL applies the layered base-url resolution:
//  1. --flag
//  2. DEWEY_BASE_URL
//  3. config.toml base_url
//  4. default
func ResolveBaseURL(flagValue string, cfg *Config) string {
	if flagValue != "" {
		return flagValue
	}
	if v := os.Getenv(EnvBaseURL); v != "" {
		return v
	}
	if cfg != nil && cfg.BaseURL != "" {
		return cfg.BaseURL
	}
	return "" // signal default
}

// TelemetryEnabled returns false if DEWEY_TELEMETRY=0|false|no.
func TelemetryEnabled() bool {
	v := os.Getenv(EnvTelemetry)
	switch v {
	case "0", "false", "no", "off":
		return false
	}
	return true
}

// MaskAPIKey returns a redacted form of the key for log/help output.
//
//	dwy_live_AbCdEfGhIjKlMnOpQrStUvWxYz  →  dwy_live_…WxYz
func MaskAPIKey(k string) string {
	if k == "" {
		return ""
	}
	if len(k) <= 12 {
		return "…"
	}
	prefix := k[:9]
	suffix := k[len(k)-4:]
	return prefix + "…" + suffix
}
