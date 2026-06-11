package config

import (
	"os"
	"path/filepath"
	"testing"
)

const sampleYAML = `
smtp:
  from: a@b.com
  host: smtp.example.com
  port: 587
  user: u
  password: p
sources:
  kkik:
    enabled: true
    login:
      email: test@example.com
      password: secret
    headless: false
    timeout_sec: 60
    sheet:
      spreadsheet_id: abc
    email:
      to: a@b.com
`

func loadSample(t *testing.T) *Config {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(sampleYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_PATH", path)
	for _, k := range []string{"KKIK_EMAIL", "KKIK_PASSWORD", "KKIK_TIMEOUT_SEC"} {
		_ = os.Unsetenv(k)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cfg
}

func TestLoadYAML(t *testing.T) {
	cfg := loadSample(t)

	k := cfg.Sources.KKIK
	if k.Login.Email != "test@example.com" {
		t.Errorf("login.email = %q", k.Login.Email)
	}
	if k.TimeoutSec != 60 {
		t.Errorf("timeout_sec = %d", k.TimeoutSec)
	}
	if k.Headless == nil || *k.Headless {
		t.Errorf("headless should honor YAML false, got %v", k.Headless)
	}
	if cfg.Sources.KKIK.Sheet.SheetName != defaultSheetName {
		t.Errorf("sheet_name = %q, want default", cfg.Sources.KKIK.Sheet.SheetName)
	}
}

func TestSourceSettings_defaults(t *testing.T) {
	cfg := loadSample(t)
	s, ok := cfg.Source("kkik")
	if !ok {
		t.Fatal("Source(kkik) not found")
	}
	// Steps omitted → email and sheet default to enabled.
	if !s.EmailStep || !s.SheetStep {
		t.Errorf("steps default: email=%v sheet=%v", s.EmailStep, s.SheetStep)
	}
	if s.Sheet.SheetName != defaultSheetName {
		t.Errorf("sheet name = %q, want default", s.Sheet.SheetName)
	}
	if s.DataDir != filepath.Join("data", "kkik") {
		t.Errorf("data dir = %q", s.DataDir)
	}
	if s.Headless {
		t.Error("headless should be false from YAML")
	}
}

func TestEnabledSources(t *testing.T) {
	cfg := loadSample(t)
	enabled := cfg.EnabledSources()
	if len(enabled) != 1 || enabled[0].Name != "kkik" {
		t.Fatalf("enabled = %+v", enabled)
	}
}

func TestSourceSettings_stepGates(t *testing.T) {
	s := SourceSettings{
		SheetStep: true,
		EmailStep: true,
	}
	if s.SheetEnabled() {
		t.Error("sheet should be disabled without spreadsheet_id")
	}
	if s.EmailEnabled() {
		t.Error("email should be disabled without recipient")
	}
	s.Sheet.SpreadsheetID = "abc"
	s.EmailTo = "x@y.com"
	if !s.SheetEnabled() || !s.EmailEnabled() {
		t.Error("expected both enabled once targets set")
	}
	s.SheetStep = false
	if s.SheetEnabled() {
		t.Error("sheet should be disabled when step off")
	}
}

func TestSheetURL(t *testing.T) {
	if got := SheetURL("abc"); got != "https://docs.google.com/spreadsheets/d/abc/edit" {
		t.Errorf("url = %q", got)
	}
}

func TestUnknownSource(t *testing.T) {
	cfg := loadSample(t)
	if _, ok := cfg.Source("nope"); ok {
		t.Error("expected unknown source to be missing")
	}
}

func TestValidateGoogleToken_missing(t *testing.T) {
	dir := t.TempDir()
	client := filepath.Join(dir, "client_secret.json")
	if err := os.WriteFile(client, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{
		Google: Google{
			OAuthClientFile: client,
			OAuthTokenFile:  filepath.Join(dir, "missing-token.json"),
		},
	}
	if err := cfg.ValidateGoogleToken(); err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestValidateSMTP(t *testing.T) {
	cfg := &Config{}
	if err := cfg.ValidateSMTP(); err == nil {
		t.Fatal("expected error for empty smtp")
	}
	cfg.SMTP = SMTP{From: "a@b.com", Host: "h", User: "u", Password: "p"}
	if err := cfg.ValidateSMTP(); err != nil {
		t.Fatalf("ValidateSMTP: %v", err)
	}
}
