package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"path/filepath"

	"denmark-housing-waitlist/internal/config"
	"denmark-housing-waitlist/internal/export"
	"denmark-housing-waitlist/internal/mailer"
	"denmark-housing-waitlist/internal/parser"
	"denmark-housing-waitlist/internal/scraper"
	"denmark-housing-waitlist/internal/sheets"
)

func main() {
	os.Exit(run())
}

func run() int {
	dumpHTML := flag.String("dump-html", "", "save fetched HTML to this path after live scrape")
	authSheets := flag.Bool("auth-sheets", false, "run one-time Google OAuth and save token (then exit)")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "err", err)
		return 1
	}

	if err := cfg.ValidateSteps(); err != nil {
		slog.Error("invalid steps config", "err", err)
		return 1
	}

	if !cfg.StepCrawlEnabled() {
		slog.Error("steps.crawl is disabled; live scrape is required")
		return 1
	}
	if err := cfg.ValidateKKIK(); err != nil {
		slog.Error("invalid config", "err", err)
		return 1
	}

	s := scraper.New(cfg)
	html, err := s.FetchHTML(context.Background())
	if err != nil {
		slog.Error("scrape", "err", err)
		return 1
	}
	if *dumpHTML != "" {
		if err := os.WriteFile(*dumpHTML, []byte(html), 0o644); err != nil {
			slog.Error("write dump html", "path", *dumpHTML, "err", err)
			return 1
		}
		slog.Info("saved html", "path", *dumpHTML)
	}

	result, err := parser.ParseHTML(html)
	if err != nil {
		slog.Error("parse", "err", err)
		return 1
	}
	slog.Info("parsed rows", "count", len(result.Rows))

	prevRanks, err := export.LoadPrevRanks(".")
	if err != nil {
		slog.Warn("load prev ranks", "err", err)
		prevRanks = map[string]int{}
	}

	csvPath := cfg.OutputCSVPath()
	if err := export.WriteCSV(csvPath, result.Rows, prevRanks); err != nil {
		slog.Error("write csv", "path", csvPath, "err", err)
		return 1
	}
	abs, _ := filepath.Abs(csvPath)
	slog.Info("wrote csv", "path", abs, "rows", len(result.Rows))

	var sheetURL string
	if cfg.StepSheetEnabled() {
		if *authSheets {
			if err := cfg.ValidateSheetsAuth(); err != nil {
				slog.Error("invalid sheets config", "err", err)
				return 1
			}
			if err := sheets.Authenticate(context.Background(), cfg); err != nil {
				slog.Error("sheets auth", "err", err)
				return 1
			}
			slog.Info("sheets oauth complete", "token", cfg.Sheets.OAuthTokenFile)
			return 0
		}

		if err := cfg.ValidateSheetsUpdate(); err != nil {
			slog.Error("invalid sheets config", "err", err)
			return 1
		}
		sheetURL, err = sheets.Update(context.Background(), cfg, result.Rows, prevRanks)
		if err != nil {
			slog.Error("update sheet", "err", err)
			return 1
		}
		slog.Info("updated sheet", "url", sheetURL)
	}

	if !cfg.StepEmailEnabled() {
		return 0
	}
	if err := cfg.ValidateSMTP(); err != nil {
		slog.Error("invalid smtp config", "err", err)
		return 1
	}
	if err := mailer.SendReport(cfg, result, csvPath, sheetURL); err != nil {
		slog.Error("send email", "err", err)
		return 1
	}
	slog.Info("sent email", "to", cfg.Email.To)
	return 0
}
