// Package kkik implements the Source for kollegierneskontor.dk (KKIK).
package kkik

import (
	"context"

	"housing-waitlist/internal/config"
	"housing-waitlist/internal/model"
	"housing-waitlist/internal/source"
)

func init() {
	source.Register("kkik", func(cfg config.SourceSettings) (source.Source, error) {
		return New(cfg)
	})
}

// KKIK is the kollegierneskontor.dk crawl source.
type KKIK struct {
	cfg config.SourceSettings
}

// New returns a configured KKIK source, validating its login credentials.
func New(cfg config.SourceSettings) (*KKIK, error) {
	if err := cfg.ValidateLogin(); err != nil {
		return nil, err
	}
	return &KKIK{cfg: cfg}, nil
}

// Descriptor identifies KKIK for output naming and email links.
func (k *KKIK) Descriptor() source.Descriptor {
	return source.Descriptor{
		Name:      "kkik",
		Title:     "KKIK",
		PortalURL: housingURL,
	}
}

// Fetch logs into KKIK and returns the housing requests page HTML.
func (k *KKIK) Fetch(ctx context.Context) (string, error) {
	return (&scraper{cfg: k.cfg}).fetchHTML(ctx)
}

// Parse extracts waitlist rows from KKIK housing requests HTML.
func (k *KKIK) Parse(html string) (*model.Result, error) {
	return parseHTML(html)
}

// RankOrder maps KKIK's numeric rank string to its sortable order.
func (k *KKIK) RankOrder(display string) (int, bool) {
	return rankOrder(display)
}
