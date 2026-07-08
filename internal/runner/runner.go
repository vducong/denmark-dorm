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

	"housing-waitlist/internal/commute"
	"housing-waitlist/internal/config"
	"housing-waitlist/internal/export"
	"housing-waitlist/internal/mailer"
	"housing-waitlist/internal/model"
	"housing-waitlist/internal/scoring"
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
	// One resolver (and its disk cache) is shared across all sources in the run.
	resolver, commuteCols, destNames := setupCommute(cfg)

	var sections []mailer.Section
	for _, s := range settings {
		section, err := runSource(ctx, cfg, s, resolver, commuteCols, destNames, opts)
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

// crawl runs fetch → parse → commute enrich for one source and returns the
// constructed source plus its parsed result. It is shared by the ranking
// pipeline (runSource) and the standalone scoring run (RunScoring). resolver is
// nil when commute is disabled; enrichment failure is best-effort and leaves the
// commute cells blank rather than aborting.
func crawl(ctx context.Context, s config.SourceSettings, resolver *commute.Resolver, opts Options) (source.Source, *model.Result, error) {
	src, err := source.New(s.Name, s)
	if err != nil {
		return nil, nil, err
	}

	slog.Info("scraping...", "source", s.Name)
	html, err := src.Fetch(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch: %w", err)
	}
	if opts.DumpHTML != "" {
		if err := os.WriteFile(opts.DumpHTML, []byte(html), 0o644); err != nil {
			return nil, nil, fmt.Errorf("dump html: %w", err)
		}
		slog.Info("saved html", "path", opts.DumpHTML)
	}

	slog.Info("parsing...", "source", s.Name)
	result, err := src.Parse(html)
	if err != nil {
		return nil, nil, fmt.Errorf("parse: %w", err)
	}
	slog.Info("parsed rows", "source", s.Name, "count", len(result.Rows))

	if resolver != nil {
		var lookupAddr commute.AddressLookup
		if ar, ok := src.(source.AddressResolver); ok {
			lookupAddr = ar.LookupAddress
		}
		if err := resolver.Enrich(ctx, result.Rows, lookupAddr); err != nil {
			slog.Warn("commute enrichment failed; continuing without commute", "source", s.Name, "err", err)
		}
	}
	return src, result, nil
}

// runSource runs one source through crawl → CSV → sheet and returns its email
// section, or nil when the source's email step is off (or --no-email).
func runSource(ctx context.Context, cfg *config.Config, s config.SourceSettings, resolver *commute.Resolver, commuteCols, destNames []string, opts Options) (*mailer.Section, error) {
	src, result, err := crawl(ctx, s, resolver, opts)
	if err != nil {
		return nil, err
	}
	desc := src.Descriptor()

	prev, err := export.LoadPrevRanks(s.DataDir, src.RankOrder)
	if err != nil {
		slog.Warn("load prev ranks", "source", s.Name, "err", err)
		prev = map[string]int{}
	}

	csvPath := s.CSVPath(time.Now())
	if err := export.WriteCSV(csvPath, result.Rows, prev, commuteCols); err != nil {
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
		sheetURL, err = sheets.Update(ctx, cfg.Google, s.Sheet, result.Rows, s.DataDir, desc.Note, src.RankOrder, commuteCols)
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
		Dests:     destNames,
		Result:    result,
		CSVPath:   csvPath,
		SheetURL:  sheetURL,
	}, nil
}

// RunScoring crawls every selected source fresh and writes the merged
// best-candidates CSV. Sources that implement source.CatalogSource are scraped
// via their full public catalog (all listed buildings, unranked); others fall
// back to the personal-waitlist fetch with rent enrichment. It is fully
// independent of the ranking pipeline: it never writes per-source history CSVs,
// updates sheets, or sends email.
func RunScoring(ctx context.Context, cfg *config.Config, settings []config.SourceSettings, opts Options) error {
	resolver, commuteCols, destNames := setupCommute(cfg)

	var inputs []scoring.Input
	for _, s := range settings {
		src, err := source.New(s.Name, s)
		if err != nil {
			return fmt.Errorf("source %s: %w", s.Name, err)
		}

		var result *model.Result
		if cs, ok := src.(source.CatalogSource); ok {
			// Discovery mode: score every building in the portal's public catalog.
			slog.Info("scraping catalog...", "source", s.Name)
			result, err = cs.FetchCatalog(ctx)
			if err != nil {
				return fmt.Errorf("source %s: catalog: %w", s.Name, err)
			}
			slog.Info("catalog rows", "source", s.Name, "count", len(result.Rows))
			// Commute enrichment: FetchCatalog populates Address but does not
			// query the routing backend — that happens here, same as the ranking
			// path.
			if resolver != nil {
				var lookupAddr commute.AddressLookup
				if ar, ok := src.(source.AddressResolver); ok {
					lookupAddr = ar.LookupAddress
				}
				if err := resolver.Enrich(ctx, result.Rows, lookupAddr); err != nil {
					slog.Warn("commute enrichment failed; continuing without commute", "source", s.Name, "err", err)
				}
			}
		} else {
			// Personal-waitlist fallback: crawl() handles fetch → parse → commute
			// enrich. Rent is then joined from the source's public catalog.
			var crawlSrc source.Source
			crawlSrc, result, err = crawl(ctx, s, resolver, opts)
			if err != nil {
				return fmt.Errorf("source %s: %w", s.Name, err)
			}
			if rr, ok := crawlSrc.(source.RentResolver); ok {
				if err := rr.EnrichRent(ctx, result.Rows); err != nil {
					slog.Warn("rent enrichment failed; continuing without rent", "source", s.Name, "err", err)
				}
			}
		}

		for _, row := range result.Rows {
			inputs = append(inputs, scoring.Input{Source: s.Name, Row: row})
		}
	}

	if len(inputs) == 0 {
		slog.Warn("scoring: no rows to score")
		return nil
	}
	ranked := scoring.Rank(inputs, scoringSettings(cfg, destNames))
	if err := export.WriteCandidates(cfg.Scoring.OutputPath, commuteCols, ranked); err != nil {
		return fmt.Errorf("write candidates: %w", err)
	}
	abs, _ := filepath.Abs(cfg.Scoring.OutputPath)
	slog.Info("wrote candidates", "path", abs, "rows", len(ranked))
	return nil
}

// setupCommute builds the shared commute resolver and ordered commute column list
// once per run. Commute is best-effort: any problem disables it (nil resolver, no
// columns) with a warning rather than failing the scrape.
func setupCommute(cfg *config.Config) (*commute.Resolver, []string, []string) {
	if !cfg.Commute.Enabled {
		return nil, nil, nil
	}
	if err := cfg.ValidateCommute(); err != nil {
		slog.Warn("commute disabled: invalid config", "err", err)
		return nil, nil, nil
	}
	cs := commuteSettings(cfg, time.Now())
	router, err := commute.NewRouter(cs.Provider, cs)
	if err != nil {
		slog.Warn("commute disabled: router init failed", "err", err)
		return nil, nil, nil
	}
	resolver, err := commute.New(cs, router)
	if err != nil {
		slog.Warn("commute disabled: resolver init failed", "err", err)
		return nil, nil, nil
	}
	names := make([]string, 0, len(cs.Destinations))
	for _, d := range cs.Destinations {
		names = append(names, d.Name)
	}
	return resolver, commute.ColumnNames(cs.Destinations), names
}

// commuteSettings projects the shared config block onto the resolver's settings,
// resolving the configured times of day to the next weekday in Copenhagen so the
// Routes API always sees a future instant.
func commuteSettings(cfg *config.Config, now time.Time) commute.Settings {
	loc, err := time.LoadLocation("Europe/Copenhagen")
	if err != nil {
		loc = time.UTC
	}
	dests := make([]commute.Destination, 0, len(cfg.Commute.Destinations))
	for _, d := range cfg.Commute.Destinations {
		dests = append(dests, commute.Destination{Name: d.Name, Address: d.Address})
	}
	return commute.Settings{
		Provider:      cfg.Commute.Provider,
		APIKey:        cfg.Commute.APIKey,
		Destinations:  dests,
		ArriveBy:      nextWeekday(now, cfg.Commute.ArriveBy, loc),
		DepartAt:      nextWeekday(now, cfg.Commute.DepartAt, loc),
		DormAddresses: cfg.Commute.DormAddresses,
		CachePath:     cfg.Commute.CachePath,
	}
}

// scoringSettings projects the shared scoring config onto the scorer's
// settings, threading in the commute destinations to average over.
func scoringSettings(cfg *config.Config, destNames []string) scoring.Settings {
	sc := cfg.Scoring
	return scoring.Settings{
		Enabled:         sc.Enabled,
		MaxRent:         sc.MaxRent,
		RentFloor:       sc.RentFloor,
		CommuteBestMin:  sc.CommuteBestMin,
		CommuteWorstMin: sc.CommuteWorstMin,
		Weights:         scoring.Weights{Commute: sc.Weights.Commute, Size: sc.Weights.Size, Rent: sc.Weights.Rent},
		RankWeight:      sc.RankWeight,
		SortBy:          sc.SortBy,
		DestNames:       destNames,
	}
}

// nextWeekday returns the soonest Mon–Fri instant at the given HH:MM (in loc)
// strictly after now, so transit estimates use a weekday rush-hour baseline and
// the Routes API never rejects a past time.
func nextWeekday(now time.Time, hhmm string, loc *time.Location) time.Time {
	tod, err := time.Parse("15:04", hhmm)
	if err != nil {
		tod, _ = time.Parse("15:04", "08:00")
	}
	d := now.In(loc)
	cand := time.Date(d.Year(), d.Month(), d.Day(), tod.Hour(), tod.Minute(), 0, 0, loc)
	for !cand.After(now) || cand.Weekday() == time.Saturday || cand.Weekday() == time.Sunday {
		cand = cand.AddDate(0, 0, 1)
		cand = time.Date(cand.Year(), cand.Month(), cand.Day(), tod.Hour(), tod.Minute(), 0, 0, loc)
	}
	return cand
}
