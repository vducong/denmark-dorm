package config

import (
	"fmt"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

const (
	defaultSheetName       = "Sheet1"
	defaultOAuthClientFile = "./internal/config/client_secret.json"
	defaultOAuthTokenFile  = "./internal/config/token.json"
)

const (
	LoginURL          = "https://www.kollegierneskontor.dk/default.aspx?func=kkikportal.login&lang=GB"
	HousingURL        = "https://www.kollegierneskontor.dk/default.aspx?func=kkikportal.housingrequests&mid=10&topmenuid=5&lang=GB"
	defaultConfigPath = "internal/config/config.yaml"
)

// Config is the application configuration (YAML + optional env overrides).
type Config struct {
	KKIK   KKIK   `yaml:"kkik"`
	Email  Email  `yaml:"email"`
	Sheets Sheets `yaml:"sheets"`
}

// Sheets holds Google Sheets export settings (OAuth).
type Sheets struct {
	SpreadsheetID   string `yaml:"spreadsheet_id" env:"SHEETS_SPREADSHEET_ID"`
	SheetName       string `yaml:"sheet_name" env:"SHEETS_SHEET_NAME"`
	OAuthClientFile string `yaml:"oauth_client_file" env:"SHEETS_OAUTH_CLIENT_FILE"`
	OAuthTokenFile  string `yaml:"oauth_token_file" env:"SHEETS_OAUTH_TOKEN_FILE"`
}

// KKIK holds portal login and scraper settings.
type KKIK struct {
	Email      string `yaml:"email" env:"KKIK_EMAIL"`
	Password   string `yaml:"password" env:"KKIK_PASSWORD"`
	Headless   bool   `yaml:"headless" env:"KKIK_HEADLESS" env-default:"true"`
	TimeoutSec int    `yaml:"timeout_sec" env:"KKIK_TIMEOUT_SEC" env-default:"120"`
	DebugDir   string `yaml:"debug_dir" env:"KKIK_DEBUG_DIR"`
}

// Email holds SMTP report delivery settings.
type Email struct {
	To           string `yaml:"to" env:"EMAIL_TO"`
	From         string `yaml:"from" env:"EMAIL_FROM"`
	SMTPHost     string `yaml:"smtp_host" env:"SMTP_HOST"`
	SMTPPort     int    `yaml:"smtp_port" env:"SMTP_PORT" env-default:"587"`
	SMTPUser     string `yaml:"smtp_user" env:"SMTP_USER"`
	SMTPPassword string `yaml:"smtp_password" env:"SMTP_PASSWORD"`
}

// Load reads config.yaml (or CONFIG_PATH) via cleanenv.
func Load() (*Config, error) {
	path := os.Getenv("CONFIG_PATH")
	if path == "" {
		path = defaultConfigPath
	}

	var cfg Config
	if err := cleanenv.ReadConfig(path, &cfg); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	cfg.applyDefaults()
	return &cfg, nil
}

func (c *Config) applyDefaults() {
	if c.KKIK.TimeoutSec <= 0 {
		c.KKIK.TimeoutSec = 120
	}
	if c.Sheets.SheetName == "" {
		c.Sheets.SheetName = defaultSheetName
	}
	if c.Sheets.OAuthClientFile == "" {
		c.Sheets.OAuthClientFile = defaultOAuthClientFile
	}
	if c.Sheets.OAuthTokenFile == "" {
		c.Sheets.OAuthTokenFile = defaultOAuthTokenFile
	}
}

// SheetsEnabled reports whether Google Sheets upload should run.
func (c *Config) SheetsEnabled() bool {
	return c.Sheets.SpreadsheetID != ""
}

// SheetURL returns a browser link to the configured spreadsheet.
func (c *Config) SheetURL() string {
	return fmt.Sprintf("https://docs.google.com/spreadsheets/d/%s/edit", c.Sheets.SpreadsheetID)
}

// OutputCSVPath returns the timestamped CSV output file path for this run.
func (c *Config) OutputCSVPath() string {
	return fmt.Sprintf("./%s_kkik_waitlist.csv", time.Now().Format("200601021504"))
}

// Timeout returns the browser timeout.
func (c *Config) Timeout() time.Duration {
	return time.Duration(c.KKIK.TimeoutSec) * time.Second
}

// ValidateKKIK checks credentials required for live scraping.
func (c *Config) ValidateKKIK() error {
	if c.KKIK.Email == "" {
		return fmt.Errorf("kkik.email is required")
	}
	if c.KKIK.Password == "" {
		return fmt.Errorf("kkik.password is required")
	}
	return nil
}

// ValidateSheetsAuth checks settings required for --auth-sheets.
func (c *Config) ValidateSheetsAuth() error {
	if c.Sheets.OAuthClientFile == "" {
		return fmt.Errorf("sheets.oauth_client_file is required")
	}
	if _, err := os.Stat(c.Sheets.OAuthClientFile); err != nil {
		return fmt.Errorf("sheets oauth client file %s: %w", c.Sheets.OAuthClientFile, err)
	}
	return nil
}

// ValidateSheetsUpdate checks settings required for uploading rows.
func (c *Config) ValidateSheetsUpdate() error {
	if c.Sheets.SpreadsheetID == "" {
		return fmt.Errorf("sheets.spreadsheet_id is required")
	}
	if err := c.ValidateSheetsAuth(); err != nil {
		return err
	}
	if c.Sheets.OAuthTokenFile == "" {
		return fmt.Errorf("sheets.oauth_token_file is required")
	}
	if _, err := os.Stat(c.Sheets.OAuthTokenFile); err != nil {
		return fmt.Errorf("sheets oauth token missing at %s (run with --auth-sheets)", c.Sheets.OAuthTokenFile)
	}
	return nil
}

// ValidateSMTP checks settings required for sending email.
func (c *Config) ValidateSMTP() error {
	if c.Email.To == "" {
		return fmt.Errorf("email.to is required")
	}
	if c.Email.From == "" {
		return fmt.Errorf("email.from is required")
	}
	if c.Email.SMTPHost == "" {
		return fmt.Errorf("email.smtp_host is required")
	}
	if c.Email.SMTPUser == "" {
		return fmt.Errorf("email.smtp_user is required")
	}
	if c.Email.SMTPPassword == "" {
		return fmt.Errorf("email.smtp_password is required")
	}
	return nil
}
