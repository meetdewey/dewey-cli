package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withFakeHome redirects the user's home directory at HOME for the lifetime
// of a test. Cleanup is automatic via t.TempDir + t.Setenv.
func withFakeHome(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	// On Linux runners os.UserHomeDir falls back to $HOME; on macOS too. No
	// further setup needed.
	return tmp
}

func TestConfig_RoundtripSaveLoad(t *testing.T) {
	withFakeHome(t)
	cfg := &Config{
		DefaultCollection: "papers",
		Output:            "json",
		Color:             "never",
		BaseURL:           "http://localhost:3000/v1",
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if *loaded != *cfg {
		t.Errorf("roundtrip mismatch:\n got %+v\nwant %+v", loaded, cfg)
	}
}

func TestConfig_LoadMissingReturnsEmpty(t *testing.T) {
	withFakeHome(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultCollection != "" || cfg.Output != "" {
		t.Errorf("expected empty config, got %+v", cfg)
	}
}

func TestConfig_LoadCorruptReturnsError(t *testing.T) {
	home := withFakeHome(t)
	deweyDir := filepath.Join(home, ".dewey")
	if err := os.MkdirAll(deweyDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(deweyDir, "config.toml"), []byte("not = valid = toml"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Error("expected parse error")
	}
}

func TestConfigPath_PointsAtHome(t *testing.T) {
	home := withFakeHome(t)
	path, err := ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".dewey", "config.toml")
	if path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
}

func TestState_Roundtrip(t *testing.T) {
	withFakeHome(t)
	s := &State{
		LastCollection:       "abc",
		RecentDocumentIDs:    []string{"d1", "d2"},
		TelemetryNoticeShown: true,
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.LastCollection != "abc" || len(loaded.RecentDocumentIDs) != 2 || !loaded.TelemetryNoticeShown {
		t.Errorf("state roundtrip: %+v", loaded)
	}
}

func TestState_FilePermissions(t *testing.T) {
	home := withFakeHome(t)
	s := &State{LastCollection: "abc"}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(filepath.Join(home, ".dewey", "state.toml"))
	if err != nil {
		t.Fatal(err)
	}
	// File mode should be readable only by owner. Skip check on Windows where
	// 0o600 is approximated.
	if st.Mode().Perm()&0o077 != 0 {
		t.Errorf("expected 0o600-ish, got %o", st.Mode().Perm())
	}
}

func TestResolveAPIKey_FlagWins(t *testing.T) {
	t.Setenv(EnvAPIKey, "from-env")
	got := ResolveAPIKey("from-flag")
	if got != "from-flag" {
		t.Errorf("got %q", got)
	}
}

func TestResolveAPIKey_FallsBackToEnv(t *testing.T) {
	t.Setenv(EnvAPIKey, "from-env")
	got := ResolveAPIKey("")
	if got != "from-env" {
		t.Errorf("got %q", got)
	}
}

func TestResolveAPIKey_EmptyWhenUnset(t *testing.T) {
	t.Setenv(EnvAPIKey, "")
	got := ResolveAPIKey("")
	if got != "" {
		t.Errorf("got %q", got)
	}
}

func TestResolveBaseURL_Precedence(t *testing.T) {
	t.Setenv(EnvBaseURL, "from-env")
	cfg := &Config{BaseURL: "from-config"}

	// 1. flag wins
	if got := ResolveBaseURL("from-flag", cfg); got != "from-flag" {
		t.Errorf("flag should win: %q", got)
	}
	// 2. env wins over config
	if got := ResolveBaseURL("", cfg); got != "from-env" {
		t.Errorf("env should win: %q", got)
	}
	// 3. config used when env unset
	t.Setenv(EnvBaseURL, "")
	if got := ResolveBaseURL("", cfg); got != "from-config" {
		t.Errorf("config should win: %q", got)
	}
	// 4. default (empty signal) when nothing set
	if got := ResolveBaseURL("", &Config{}); got != "" {
		t.Errorf("expected empty (= default), got: %q", got)
	}
}

func TestTelemetryEnabled(t *testing.T) {
	cases := map[string]bool{
		"":      true,
		"0":     false,
		"false": false,
		"no":    false,
		"off":   false,
		"1":     true,
		"on":    true,
	}
	for v, want := range cases {
		t.Setenv(EnvTelemetry, v)
		if got := TelemetryEnabled(); got != want {
			t.Errorf("TelemetryEnabled(%q) = %v, want %v", v, got, want)
		}
	}
}

func TestMaskAPIKey(t *testing.T) {
	cases := map[string]string{
		"":                                    "",
		"short":                                "…",
		"dwy_live_AbCdEfGhIjKlMnOpQrStUvWxYz":  "dwy_live_…WxYz",
	}
	for in, want := range cases {
		got := MaskAPIKey(in)
		if got != want {
			t.Errorf("MaskAPIKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMaskAPIKey_LeaksOnlyTrailingFour(t *testing.T) {
	in := "dwy_live_secretsecretsecret_FACE"
	got := MaskAPIKey(in)
	// The interior of the secret should not appear in the masked output.
	if strings.Contains(got, "secretsecret") {
		t.Errorf("interior leaked: %q", got)
	}
	if !strings.HasSuffix(got, "FACE") {
		t.Errorf("expected suffix FACE, got %q", got)
	}
}
