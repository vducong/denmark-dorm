package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	// Embed the timezone database so commute time-of-day resolution to
	// Europe/Copenhagen works even on systems without system tzdata.
	_ "time/tzdata"

	"housing-waitlist/internal/config"
	"housing-waitlist/internal/runner"
	"housing-waitlist/internal/sheets"
	"housing-waitlist/internal/source"
	_ "housing-waitlist/internal/source/all" // register every source
)

func main() {
	os.Exit(run())
}

func run() int {
	dumpHTML := flag.String("dump-html", "", "save fetched HTML to this path (requires a single --source)")
	authSheets := flag.Bool("auth-sheets", false, "run one-time Google OAuth and save token (then exit)")
	sourceList := flag.String("source", "", "comma-separated sources to run (default: all enabled)")
	listSources := flag.Bool("list-sources", false, "print registered sources and exit")
	noEmail := flag.Bool("no-email", false, "skip the email step for this run")
	noSheet := flag.Bool("no-sheet", false, "skip the Google Sheets step for this run")
	scoreOnly := flag.Bool("score-only", false, "crawl fresh and write only the scored candidates CSV (no ranking CSV, sheet, or email)")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	if *listSources {
		for _, n := range source.Names() {
			fmt.Println(n)
		}
		return 0
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "err", err)
		return 1
	}

	ctx := context.Background()

	if *authSheets {
		if err := cfg.ValidateGoogleAuth(); err != nil {
			slog.Error("invalid google config", "err", err)
			return 1
		}
		if err := sheets.Authenticate(ctx, cfg.Google); err != nil {
			slog.Error("sheets auth", "err", err)
			return 1
		}
		slog.Info("sheets oauth complete", "token", cfg.Google.OAuthTokenFile)
		return 0
	}

	settings, err := selectSources(cfg, *sourceList)
	if err != nil {
		slog.Error("select sources", "err", err)
		return 1
	}
	if len(settings) == 0 {
		slog.Error("no sources selected; enable a source in config or pass --source")
		return 1
	}
	if *dumpHTML != "" && len(settings) != 1 {
		slog.Error("--dump-html requires exactly one --source")
		return 1
	}

	opts := runner.Options{DumpHTML: *dumpHTML, NoEmail: *noEmail, NoSheet: *noSheet}
	if *scoreOnly {
		if err := runner.RunScoring(ctx, cfg, settings, opts); err != nil {
			slog.Error("score", "err", err)
			return 1
		}
		return 0
	}
	if err := runner.Run(ctx, cfg, settings, opts); err != nil {
		slog.Error("run", "err", err)
		return 1
	}
	return 0
}

// selectSources resolves --source (or all enabled sources when empty). An
// explicitly named source runs even if its config has enabled: false.
func selectSources(cfg *config.Config, list string) ([]config.SourceSettings, error) {
	if strings.TrimSpace(list) == "" {
		return cfg.EnabledSources(), nil
	}
	var out []config.SourceSettings
	for _, name := range strings.Split(list, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		s, ok := cfg.Source(name)
		if !ok {
			return nil, fmt.Errorf("unknown source %q (known: %v)", name, config.SourceNames())
		}
		out = append(out, s)
	}
	return out, nil
}
