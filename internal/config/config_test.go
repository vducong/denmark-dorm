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
}
