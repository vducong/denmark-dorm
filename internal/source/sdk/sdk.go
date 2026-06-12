// Package sdk implements the Source for mit.s.dk/studiebolig (s.dk).
package sdk

import (
	"context"

	"housing-waitlist/internal/config"
	"housing-waitlist/internal/model"
	"housing-waitlist/internal/source"
)

func init() {
	source.Register("sdk", func(cfg config.SourceSettings) (source.Source, error) {
		return New(cfg)
	})
}

// SDK is the mit.s.dk/studiebolig crawl source.
type SDK struct {
	cfg config.SourceSettings
}

// New returns a configured SDK source, validating its login credentials.
func New(cfg config.SourceSettings) (*SDK, error) {
	if err := cfg.ValidateLogin(); err != nil {
		return nil, err
	}
	return &SDK{cfg: cfg}, nil
}

// Descriptor identifies s.dk for output naming and email links.
func (s *SDK) Descriptor() source.Descriptor {
	return source.Descriptor{
		Name:      "sdk",
		Title:     "s.dk",
		PortalURL: listURL,
	}
}

// Fetch logs into s.dk and returns the applicant's per-building ranking tables
// stitched into one document.
func (s *SDK) Fetch(ctx context.Context) (string, error) {
	return (&scraper{cfg: s.cfg}).fetchHTML(ctx)
}

// Parse extracts waitlist rows from s.dk list HTML.
func (s *SDK) Parse(html string) (*model.Result, error) {
	return parseHTML(html)
}

// RankOrder maps s.dk's rank string to its sortable order.
func (s *SDK) RankOrder(display string) (int, bool) {
	return rankOrder(display)
}
