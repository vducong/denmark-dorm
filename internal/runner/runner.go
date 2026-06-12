// Package runner orchestrates the per-source pipeline:
// fetch → parse → CSV → sheet → email.
// It is source-agnostic; sources are resolved from the registry.
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
	// DumpHTML, when set, writes the fetched HTML to this path.
	// Only valid with a single selected source.
	DumpHTML string
	// NoEmail / NoSheet force those steps off for this run, overriding config.
	NoEmail bool
	NoSheet bool
}

// Run executes the per-source pipeline (fetch → parse → CSV → sheet) for each
// source, stopping at the first error. Email is aggregated: every email-enabled
// source contributes a section, and one combined digest is sent after all
// sources succeed (so a single source failing means no email at all).
func Run(ctx context.Context, cfg *config.Config, settings []config.SourceSettings, opts Options) error {
	var sections []mailer.Section
	for _, s := range settings {
		section, err := runSource(ctx, cfg, s, opts)
		if err != nil {
			return fmt.Errorf("source %s: %w", s.Name, err)
		}
		if section != nil {
			sections = append(sections, *section)
		}
	}

	if len(sections) == 0 {
		return nil
	}
	if err := cfg.ValidateSMTP(); err != nil {
		return fmt.Errorf("smtp: %w", err)
	}
	slog.Info("sending combined email...", "sources", len(sections), "to", cfg.SMTP.To)
	if err := mailer.SendDigest(cfg.SMTP, cfg.SMTP.To, sections); err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	slog.Info("sent combined email", "sources", len(sections), "to", cfg.SMTP.To)
	return nil
}

// runSource runs one source through fetch → parse → CSV → sheet and returns its
// email section, or nil when the source's email step is off (or --no-email).
func runSource(ctx context.Context, cfg *config.Config, s config.SourceSettings, opts Options) (*mailer.Section, error) {
	src, err := source.New(s.Name, s)
	if err != nil {
		return nil, err
	}
	desc := src.Descriptor()

	slog.Info("scraping...", "source", s.Name)
	html, err := src.Fetch(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	if opts.DumpHTML != "" {
		if err := os.WriteFile(opts.DumpHTML, []byte(html), 0o644); err != nil {
			return nil, fmt.Errorf("dump html: %w", err)
		}
		slog.Info("saved html", "path", opts.DumpHTML)
	}

	slog.Info("parsing...", "source", s.Name)
	result, err := src.Parse(html)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	slog.Info("parsed rows", "source", s.Name, "count", len(result.Rows))

	prev, err := export.LoadPrevRanks(s.DataDir, src.RankOrder)
	if err != nil {
		slog.Warn("load prev ranks", "source", s.Name, "err", err)
		prev = map[string]int{}
	}

	csvPath := s.CSVPath(time.Now())
	if err := export.WriteCSV(csvPath, result.Rows, prev); err != nil {
		return nil, fmt.Errorf("write csv: %w", err)
	}
	abs, _ := filepath.Abs(csvPath)
	slog.Info("wrote csv", "source", s.Name, "path", abs, "rows", len(result.Rows))

	var sheetURL string
	if s.SheetEnabled() && !opts.NoSheet {
		if err := cfg.ValidateGoogleToken(); err != nil {
			return nil, fmt.Errorf("sheets: %w", err)
		}
		if err := s.ValidateSheet(); err != nil {
			return nil, err
		}
		slog.Info("updating sheet...", "source", s.Name)
		sheetURL, err = sheets.Update(ctx, cfg.Google, s.Sheet, result.Rows, s.DataDir, desc.Note, src.RankOrder)
		if err != nil {
			return nil, fmt.Errorf("update sheet: %w", err)
		}
		slog.Info("updated sheet", "source", s.Name, "url", sheetURL)
	}

	if !s.EmailEnabled() || opts.NoEmail {
		return nil, nil
	}
	return &mailer.Section{
		Name:      s.Name,
		Title:     desc.Title,
		PortalURL: desc.PortalURL,
		Result:    result,
		CSVPath:   csvPath,
		SheetURL:  sheetURL,
	}, nil
}
