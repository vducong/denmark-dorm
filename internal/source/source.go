// Package source defines the pluggable crawl-source abstraction and a registry
// of available sources. Each source fetches its own HTML and parses it into the
// shared model; the downstream pipeline is source-agnostic.
package source

import (
	"context"

	"housing-waitlist/internal/model"
)

// Descriptor identifies a source for output naming and email links.
type Descriptor struct {
	Name      string // machine key, e.g. "kkik"
	Title     string // human title, e.g. "Kollegiernes Kontor"
	PortalURL string // link embedded in the email body
}

// Source crawls one waitlist provider and parses it into the common model.
type Source interface {
	Descriptor() Descriptor
	// Fetch logs in / requests and returns raw HTML.
	Fetch(ctx context.Context) (string, error)
	// Parse turns raw HTML into normalized rows.
	Parse(html string) (*model.Result, error)
	// RankOrder projects a displayed rank onto a sortable scale (lower = better).
	// It is reused to convert previous CSV ranks back to an order for diffing.
	RankOrder(display string) (int, bool)
}
