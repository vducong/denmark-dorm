// Package runner orchestrates the per-source pipeline: fetch → parse → CSV →
// sheet → email. It is source-agnostic; sources are resolved from the registry.
package runner

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"housing-waitlist/internal/config"
	"housing-waitlist/internal/export"
	"housing-waitlist/internal/mailer"
	"housing-waitlist/internal/sheets"
	"housing-waitlist/internal/source"
)

// Options controls a single run.
type Options struct {
	// DumpHTML, when set, writes the fetched HTML to this path. Only valid with
	// a single selected source.
	DumpHTML string
	// NoEmail / NoSheet force those steps off for this run, overriding config.
	NoEmail bool
	NoSheet bool
}

// Run executes the pipeline for each source's settings, stopping at the first
// error.
func Run(ctx context.Context, cfg *config.Config, settings []config.SourceSettings, opts Options) error {
	for _, s := range settings {
		if err := runSource(ctx, cfg, s, opts); err != nil {
			return fmt.Errorf("source %s: %w", s.Name, err)
		}
	}
	return nil
}

func runSource(ctx context.Context, cfg *config.Config, s config.SourceSettings, opts Options) error {
	src, err := source.New(s.Name, s)
	if err != nil {
		return err
	}
	desc := src.Descriptor()

	slog.Info("scraping...", "source", s.Name)
	html, err := src.Fetch(ctx)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}
	if opts.DumpHTML != "" {
		if err := os.WriteFile(opts.DumpHTML, []byte(html), 0o644); err != nil {
			return fmt.Errorf("dump html: %w", err)
		}
		slog.Info("saved html", "path", opts.DumpHTML)
	}

	slog.Info("parsing...", "source", s.Name)
	result, err := src.Parse(html)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	slog.Info("parsed rows", "source", s.Name, "count", len(result.Rows))

	prev, err := export.LoadPrevRanks(s.DataDir, src.RankOrder)
	if err != nil {
		slog.Warn("load prev ranks", "source", s.Name, "err", err)
		prev = map[string]int{}
	}

	csvPath := s.CSVPath(time.Now())
	if err := export.WriteCSV(csvPath, result.Rows, prev); err != nil {
		return fmt.Errorf("write csv: %w", err)
	}
	abs, _ := filepath.Abs(csvPath)
	slog.Info("wrote csv", "source", s.Name, "path", abs, "rows", len(result.Rows))

	var sheetURL string
	if s.SheetEnabled() && !opts.NoSheet {
		if err := cfg.ValidateGoogleToken(); err != nil {
			return fmt.Errorf("sheets: %w", err)
		}
		if err := s.ValidateSheet(); err != nil {
			return err
		}
		slog.Info("updating sheet...", "source", s.Name)
		sheetURL, err = sheets.Update(ctx, cfg.Google, s.Sheet, result.Rows, s.DataDir, src.RankOrder)
		if err != nil {
			return fmt.Errorf("update sheet: %w", err)
		}
		slog.Info("updated sheet", "source", s.Name, "url", sheetURL)
	}

	if s.EmailEnabled() && !opts.NoEmail {
		if err := cfg.ValidateSMTP(); err != nil {
			return fmt.Errorf("smtp: %w", err)
		}
		slog.Info("sending email...", "source", s.Name)
		report := mailer.Report{Title: desc.Title, PortalURL: desc.PortalURL, To: s.EmailTo}
		if err := mailer.SendReport(cfg.SMTP, report, result, csvPath, sheetURL); err != nil {
			return fmt.Errorf("send email: %w", err)
		}
		slog.Info("sent email", "source", s.Name, "to", s.EmailTo)
	}

	return nil
}
