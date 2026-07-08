package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

const (
	defaultSheetName       = "Sheet1"
	defaultOAuthClientFile = "./internal/config/client_secret.json"
	defaultOAuthTokenFile  = "./internal/config/token.json"
	defaultConfigPath      = "internal/config/config.yaml"
	defaultTimeoutSec      = 120
	// sdk crawls one building detail page per signed-up property,
	// so its run needs far longer than a single-page source.
	defaultSDKTimeoutSec = 600

	// Commute defaults: routing backend, then arrive before the first class and
	// leave at the worst case.
	defaultCommuteProvider = "google"
	defaultArriveBy        = "08:00"
	defaultDepartAt        = "17:00"

	// Scoring defaults: commute scored best at ≤20 min / worst at ≥60 min, the
	// opportunity blend leans half on waitlist position, and the merged list
	// leads with the desirability score.
	defaultCommuteBestMin    = 20
	defaultCommuteWorstMin   = 60
	defaultScoringRankWeight = 0.5
	defaultScoringSortBy     = "desirability"
)

// Config is the application configuration (YAML + optional env overrides).
//
// SMTP and Google are shared by every source (one mail sender, one Google identity);
// Sources holds the per-source settings.
type Config struct {
	SMTP    SMTP    `yaml:"smtp"`
	Google  Google  `yaml:"google"`
	Commute Commute `yaml:"commute"`
	Scoring Scoring `yaml:"scoring"`
	Sources Sources `yaml:"sources"`
}

// SMTP holds the shared mail transport, sender, and the single combined recipient.
//
// To is global rather than per-source: every enabled source's section ships in
// one digest to this address, so the recipient is a shared concern like From.
// Cc is optional and may list several comma-separated addresses.
type SMTP struct {
	Host     string `yaml:"host"     env:"SMTP_HOST"`
	Port     int    `yaml:"port"     env:"SMTP_PORT" env-default:"587"`
	User     string `yaml:"user"     env:"SMTP_USER"`
	Password string `yaml:"password" env:"SMTP_PASSWORD"`
	From     string `yaml:"from"     env:"SMTP_FROM"`
	To       string `yaml:"to"       env:"SMTP_TO"`
	Cc       string `yaml:"cc"       env:"SMTP_CC"`
}

// Google holds the shared OAuth identity used for Google Sheets.
type Google struct {
	OAuthClientFile string `yaml:"oauth_client_file" env:"GOOGLE_OAUTH_CLIENT_FILE"`
	OAuthTokenFile  string `yaml:"oauth_token_file"  env:"GOOGLE_OAUTH_TOKEN_FILE"`
}

// Commute holds the shared commute-time settings. It is shared across sources;
// each (origin, destination, time-window) is routed once and cached to disk, so
// the backend is hit only on a cache miss.
//
// Provider selects the routing backend (pluggable; "google" by default — see
// internal/commute). ArriveBy/DepartAt are local times of day ("HH:MM",
// Europe/Copenhagen); the runner resolves them to the next weekday so transit
// estimates use a stable rush-hour baseline. Destinations is a configurable list
// of campuses, each producing its own <name>_* columns.
type Commute struct {
	Enabled       bool                 `yaml:"enabled"`
	Provider      string               `yaml:"provider" env:"COMMUTE_PROVIDER"`
	APIKey        string               `yaml:"api_key" env:"COMMUTE_API_KEY"`
	ArriveBy      string               `yaml:"arrive_by"`
	DepartAt      string               `yaml:"depart_at"`
	CachePath     string               `yaml:"cache_path"`
	Destinations  []CommuteDestination `yaml:"destinations"`
	DormAddresses map[string]string    `yaml:"dorm_addresses"`
}

// CommuteDestination is one campus to route to. Name prefixes its columns.
type CommuteDestination struct {
	Name    string `yaml:"name"`
	Address string `yaml:"address"`
}

// Scoring holds the shared dorm-scoring settings. It ranks every crawled room
// by desirability (rent + commute + size) and opportunity (desirability blended
// with waitlist position) into one merged CSV at OutputPath.
//
// MaxRent is a hard budget gate (DKK/mo): rooms whose cheapest rent exceeds it
// are dropped, and it also anchors the rent score band [RentFloor, MaxRent]; a
// zero MaxRent disables the gate and scores rent by relative min-max instead.
// Commute is scored on the fixed band [CommuteBestMin, CommuteWorstMin] minutes.
// SortBy picks the lead score ("desirability" or "opportunity").
type Scoring struct {
	Enabled         bool           `yaml:"enabled"`
	OutputPath      string         `yaml:"output_path"`
	MaxRent         int            `yaml:"max_rent"`
	RentFloor       int            `yaml:"rent_floor"`
	CommuteBestMin  int            `yaml:"commute_best_min"`
	CommuteWorstMin int            `yaml:"commute_worst_min"`
	RankWeight      float64        `yaml:"rank_weight"`
	SortBy          string         `yaml:"sort_by"`
	Weights         ScoringWeights `yaml:"weights"`
}

// ScoringWeights is each factor's share of the desirability score. When all
// three are zero (the block is omitted) applyDefaults sets 0.4/0.3/0.3.
type ScoringWeights struct {
	Commute float64 `yaml:"commute"`
	Size    float64 `yaml:"size"`
	Rent    float64 `yaml:"rent"`
}

// Sources holds one block per registered source.
// Adding a source adds a field here plus a case in Source().
type Sources struct {
	KKIK KKIKConfig `yaml:"kkik"`
	SDK  SDKConfig  `yaml:"sdk"`
}

// KKIKConfig holds the KKIK (kollegierneskontor.dk) source settings.
type KKIKConfig struct {
	Enabled bool        `yaml:"enabled"`
	Steps   Steps       `yaml:"steps"`
	Login   Credentials `yaml:"login"`
	// Headless is a pointer so a YAML false is honored; nil (omitted) defaults to true.
	// A plain bool with env-default:"true" would override a real false,
	// since false is indistinguishable from the zero value.
	Headless   *bool       `yaml:"headless"`
	TimeoutSec int         `yaml:"timeout_sec" env:"KKIK_TIMEOUT_SEC" env-default:"120"`
	DebugDir   string      `yaml:"debug_dir"   env:"KKIK_DEBUG_DIR"`
	Sheet      SheetTarget `yaml:"sheet"`
	DataDir    string      `yaml:"data_dir"`
}

// Credentials holds a source's login. Env tags are source-specific,
// so a new source declares its own credentials type.
type Credentials struct {
	Email    string `yaml:"email"    env:"KKIK_EMAIL"`
	Password string `yaml:"password" env:"KKIK_PASSWORD"`
}

// SDKConfig holds the s.dk (mit.s.dk/studiebolig) source settings.
type SDKConfig struct {
	Enabled bool           `yaml:"enabled"`
	Steps   Steps          `yaml:"steps"`
	Login   SDKCredentials `yaml:"login"`
	// Headless is a pointer so a YAML false is honored; nil (omitted) defaults to true.
	// A plain bool with env-default:"true" would override a real false,
	// since false is indistinguishable from the zero value.
	Headless   *bool       `yaml:"headless"`
	TimeoutSec int         `yaml:"timeout_sec" env:"SDK_TIMEOUT_SEC" env-default:"120"`
	DebugDir   string      `yaml:"debug_dir"   env:"SDK_DEBUG_DIR"`
	Sheet      SheetTarget `yaml:"sheet"`
	DataDir    string      `yaml:"data_dir"`
}

// SDKCredentials holds s.dk's login. It declares its own env tags
// so they don't collide with KKIK's.
type SDKCredentials struct {
	Email    string `yaml:"email"    env:"SDK_EMAIL"`
	Password string `yaml:"password" env:"SDK_PASSWORD"`
}

// Steps toggles a source's optional pipeline phases. Crawl (fetch + CSV) always runs;
// email and sheet default to true when the block is omitted.
type Steps struct {
	Email *bool `yaml:"email"`
	Sheet *bool `yaml:"sheet"`
}

func (s Steps) email() bool { return s.Email == nil || *s.Email }
func (s Steps) sheet() bool { return s.Sheet == nil || *s.Sheet }

// SheetTarget identifies one source's Google Sheet destination.
type SheetTarget struct {
	SpreadsheetID string `yaml:"spreadsheet_id"`
	SheetName     string `yaml:"sheet_name"`
}

// SourceSettings is the normalized, source-agnostic view the runner and source factories consume.
// Per-source structs are projected onto it by Source().
type SourceSettings struct {
	Name       string
	Enabled    bool
	EmailStep  bool
	SheetStep  bool
	Login      Credentials
	Headless   bool
	TimeoutSec int
	DebugDir   string
	Sheet      SheetTarget
	DataDir    string
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
	if c.Google.OAuthClientFile == "" {
		c.Google.OAuthClientFile = defaultOAuthClientFile
	}
	if c.Google.OAuthTokenFile == "" {
		c.Google.OAuthTokenFile = defaultOAuthTokenFile
	}
	if c.Sources.KKIK.TimeoutSec <= 0 {
		c.Sources.KKIK.TimeoutSec = defaultTimeoutSec
	}
	if c.Sources.KKIK.Sheet.SheetName == "" {
		c.Sources.KKIK.Sheet.SheetName = defaultSheetName
	}
	if c.Sources.SDK.TimeoutSec <= 0 {
		c.Sources.SDK.TimeoutSec = defaultSDKTimeoutSec
	}
	if c.Sources.SDK.Sheet.SheetName == "" {
		c.Sources.SDK.Sheet.SheetName = defaultSheetName
	}
	if c.Commute.Provider == "" {
		c.Commute.Provider = defaultCommuteProvider
	}
	if c.Commute.ArriveBy == "" {
		c.Commute.ArriveBy = defaultArriveBy
	}
	if c.Commute.DepartAt == "" {
		c.Commute.DepartAt = defaultDepartAt
	}
	if c.Commute.CachePath == "" {
		c.Commute.CachePath = filepath.Join("data", "commute_cache.json")
	}
	c.applyScoringDefaults()
}

func (c *Config) applyScoringDefaults() {
	s := &c.Scoring
	if s.OutputPath == "" {
		s.OutputPath = filepath.Join("data", "candidates.csv")
	}
	if s.CommuteBestMin <= 0 {
		s.CommuteBestMin = defaultCommuteBestMin
	}
	if s.CommuteWorstMin <= 0 {
		s.CommuteWorstMin = defaultCommuteWorstMin
	}
	if s.SortBy == "" {
		s.SortBy = defaultScoringSortBy
	}
	// A zero rank_weight makes opportunity identical to desirability, so treat
	// it as unset and apply the blend default.
	if s.RankWeight <= 0 {
		s.RankWeight = defaultScoringRankWeight
	}
	// Default the weights only when the whole block is omitted (all zero), so a
	// deliberate single-factor weighting is preserved.
	if s.Weights == (ScoringWeights{}) {
		s.Weights = ScoringWeights{Commute: 0.4, Size: 0.3, Rent: 0.3}
	}
}

// Source returns the normalized settings for a registered source name.
func (c *Config) Source(name string) (SourceSettings, bool) {
	switch name {
	case "kkik":
		return c.Sources.KKIK.settings("kkik"), true
	case "sdk":
		return c.Sources.SDK.settings("sdk"), true
	default:
		return SourceSettings{}, false
	}
}

// EnabledSources returns the settings of every source with enabled: true.
func (c *Config) EnabledSources() []SourceSettings {
	var out []SourceSettings
	for _, name := range SourceNames() {
		if s, ok := c.Source(name); ok && s.Enabled {
			out = append(out, s)
		}
	}
	return out
}

// SourceNames lists every source known to config (independent of the registry).
func SourceNames() []string {
	return []string{"kkik", "sdk"}
}

func (k KKIKConfig) settings(name string) SourceSettings {
	dataDir := k.DataDir
	if dataDir == "" {
		dataDir = filepath.Join("data", name)
	}
	timeout := k.TimeoutSec
	if timeout <= 0 {
		timeout = defaultTimeoutSec
	}
	sheet := k.Sheet
	if sheet.SheetName == "" {
		sheet.SheetName = defaultSheetName
	}
	headless := k.Headless == nil || *k.Headless
	return SourceSettings{
		Name:       name,
		Enabled:    k.Enabled,
		EmailStep:  k.Steps.email(),
		SheetStep:  k.Steps.sheet(),
		Login:      k.Login,
		Headless:   headless,
		TimeoutSec: timeout,
		DebugDir:   k.DebugDir,
		Sheet:      sheet,
		DataDir:    dataDir,
	}
}

func (s SDKConfig) settings(name string) SourceSettings {
	dataDir := s.DataDir
	if dataDir == "" {
		dataDir = filepath.Join("data", name)
	}
	timeout := s.TimeoutSec
	if timeout <= 0 {
		timeout = defaultTimeoutSec
	}
	sheet := s.Sheet
	if sheet.SheetName == "" {
		sheet.SheetName = defaultSheetName
	}
	headless := s.Headless == nil || *s.Headless
	return SourceSettings{
		Name:       name,
		Enabled:    s.Enabled,
		EmailStep:  s.Steps.email(),
		SheetStep:  s.Steps.sheet(),
		Login:      Credentials{Email: s.Login.Email, Password: s.Login.Password},
		Headless:   headless,
		TimeoutSec: timeout,
		DebugDir:   s.DebugDir,
		Sheet:      sheet,
		DataDir:    dataDir,
	}
}

// SheetEnabled reports whether the source should upload to Google Sheets.
func (s SourceSettings) SheetEnabled() bool {
	return s.SheetStep && s.Sheet.SpreadsheetID != ""
}

// EmailEnabled reports whether this source contributes a section to the combined
// email. The recipient is global (smtp.to), so only the per-source step gates here.
func (s SourceSettings) EmailEnabled() bool {
	return s.EmailStep
}

// Timeout returns the browser timeout for the source.
func (s SourceSettings) Timeout() time.Duration {
	return time.Duration(s.TimeoutSec) * time.Second
}

// CSVPath returns the timestamped CSV output path under the source's data dir.
func (s SourceSettings) CSVPath(now time.Time) string {
	return filepath.Join(s.DataDir, fmt.Sprintf("%s_waitlist.csv", now.Format("200601021504")))
}

// SheetURL returns a browser link to a spreadsheet.
func SheetURL(spreadsheetID string) string {
	return fmt.Sprintf("https://docs.google.com/spreadsheets/d/%s/edit", spreadsheetID)
}

// ValidateLogin checks credentials required for a live scrape.
func (s SourceSettings) ValidateLogin() error {
	if s.Login.Email == "" {
		return fmt.Errorf("%s: login.email is required", s.Name)
	}
	if s.Login.Password == "" {
		return fmt.Errorf("%s: login.password is required", s.Name)
	}
	return nil
}

// ValidateSheet checks settings required for uploading a source's rows.
func (s SourceSettings) ValidateSheet() error {
	if s.Sheet.SpreadsheetID == "" {
		return fmt.Errorf("%s: sheet.spreadsheet_id is required", s.Name)
	}
	if s.Sheet.SheetName == "" {
		return fmt.Errorf("%s: sheet.sheet_name is required", s.Name)
	}
	return nil
}

// ValidateGoogleAuth checks the shared OAuth client file required for --auth-sheets.
func (c *Config) ValidateGoogleAuth() error {
	if c.Google.OAuthClientFile == "" {
		return fmt.Errorf("google.oauth_client_file is required")
	}
	if _, err := os.Stat(c.Google.OAuthClientFile); err != nil {
		return fmt.Errorf("google oauth client file %s: %w", c.Google.OAuthClientFile, err)
	}
	return nil
}

// ValidateGoogleToken checks the shared OAuth token required to upload rows.
func (c *Config) ValidateGoogleToken() error {
	if err := c.ValidateGoogleAuth(); err != nil {
		return err
	}
	if c.Google.OAuthTokenFile == "" {
		return fmt.Errorf("google.oauth_token_file is required")
	}
	if _, err := os.Stat(c.Google.OAuthTokenFile); err != nil {
		return fmt.Errorf("google oauth token missing at %s (run with --auth-sheets)", c.Google.OAuthTokenFile)
	}
	return nil
}

// ValidateCommute checks the settings required to call the Routes API. It is
// only enforced when commute is enabled; a disabled block is always valid.
func (c *Config) ValidateCommute() error {
	if c.Commute.APIKey == "" {
		return fmt.Errorf("commute.api_key is required (or COMMUTE_API_KEY)")
	}
	if len(c.Commute.Destinations) == 0 {
		return fmt.Errorf("commute.destinations must list at least one campus")
	}
	for i, d := range c.Commute.Destinations {
		if d.Name == "" || d.Address == "" {
			return fmt.Errorf("commute.destinations[%d]: name and address are required", i)
		}
	}
	if _, err := time.Parse("15:04", c.Commute.ArriveBy); err != nil {
		return fmt.Errorf("commute.arrive_by %q must be HH:MM", c.Commute.ArriveBy)
	}
	if _, err := time.Parse("15:04", c.Commute.DepartAt); err != nil {
		return fmt.Errorf("commute.depart_at %q must be HH:MM", c.Commute.DepartAt)
	}
	return nil
}

// ValidateSMTP checks the shared settings required to send email.
func (c *Config) ValidateSMTP() error {
	if c.SMTP.From == "" {
		return fmt.Errorf("smtp.from is required")
	}
	if c.SMTP.Host == "" {
		return fmt.Errorf("smtp.host is required")
	}
	if c.SMTP.User == "" {
		return fmt.Errorf("smtp.user is required")
	}
	if c.SMTP.Password == "" {
		return fmt.Errorf("smtp.password is required")
	}
	if c.SMTP.To == "" {
		return fmt.Errorf("smtp.to is required")
	}
	return nil
}
