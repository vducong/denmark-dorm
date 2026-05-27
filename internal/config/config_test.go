package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `
kkik:
  email: test@example.com
  password: secret
  headless: false
  timeout_sec: 60
email:
  to: a@b.com
  from: a@b.com
  smtp_host: smtp.example.com
  smtp_port: 587
  smtp_user: u
  smtp_password: p
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("CONFIG_PATH", path)
	_ = os.Unsetenv("KKIK_EMAIL")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.KKIK.Email != "test@example.com" {
		t.Errorf("email = %q", cfg.KKIK.Email)
	}
	if cfg.KKIK.TimeoutSec != 60 {
		t.Errorf("timeout_sec = %d", cfg.KKIK.TimeoutSec)
	}
	if cfg.Sheets.SheetName != defaultSheetName {
		t.Errorf("sheet_name = %q", cfg.Sheets.SheetName)
	}
	if !cfg.StepCrawlEnabled() || !cfg.StepEmailEnabled() {
		t.Errorf("steps defaults: crawl=%v email=%v", cfg.Steps.Crawl, cfg.Steps.Email)
	}
}

func TestStepSheetEnabled(t *testing.T) {
	cfg := &Config{Steps: Steps{Sheet: true}}
	if cfg.StepSheetEnabled() {
		t.Fatal("expected false without spreadsheet_id")
	}
	cfg.Sheets.SpreadsheetID = "abc"
	if !cfg.StepSheetEnabled() {
		t.Fatal("expected true with sheet step and spreadsheet_id")
	}
	cfg.Steps.Sheet = false
	if cfg.StepSheetEnabled() {
		t.Fatal("expected false when steps.sheet is false")
	}
}

func TestValidateSteps(t *testing.T) {
	cfg := &Config{}
	if err := cfg.ValidateSteps(); err == nil {
		t.Fatal("expected error when all steps disabled")
	}
	cfg.Steps.Email = true
	if err := cfg.ValidateSteps(); err != nil {
		t.Fatalf("ValidateSteps: %v", err)
	}
}

func TestSheetsEnabled(t *testing.T) {
	cfg := &Config{Sheets: Sheets{SpreadsheetID: "abc"}}
	if !cfg.SheetsEnabled() {
		t.Fatal("expected sheets enabled")
	}
	if cfg.SheetURL() != "https://docs.google.com/spreadsheets/d/abc/edit" {
		t.Errorf("url = %q", cfg.SheetURL())
	}
}

func TestValidateSheetsUpdate_missingToken(t *testing.T) {
	dir := t.TempDir()
	client := filepath.Join(dir, "client_secret.json")
	if err := os.WriteFile(client, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	token := filepath.Join(dir, "missing-token.json")

	cfg := &Config{
		Sheets: Sheets{
			SpreadsheetID:   "sheet-id",
			OAuthClientFile: client,
			OAuthTokenFile:  token,
			SheetName:       "Waitlist",
		},
	}
	if err := cfg.ValidateSheetsUpdate(); err == nil {
		t.Fatal("expected error for missing token")
	}
}
