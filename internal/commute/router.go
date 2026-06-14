package commute

import (
	"context"
	"fmt"
	"time"
)

// Router computes the travel time of a single leg in whole minutes. It is the
// pluggable seam for commute backends: the Google Maps adapter (google.go) is
// the only implementation today, but Rejseplanen, OpenTripPlanner, or any other
// provider can be added by implementing this interface and wiring a case into
// NewRouter — nothing else in the package or the pipeline changes.
//
// An empty string with a nil error means "no route by this mode/time": a real,
// cacheable answer, distinct from a transport failure (which returns an error).
type Router interface {
	Minutes(ctx context.Context, leg Leg) (string, error)
}

type RouterProvider string

const (
	RouterProviderGoogle RouterProvider = "google"
)

// Mode is the travel mode for a leg. The values are the canonical Routes API
// mode names; adapters translate them to their own vocabulary as needed.
type Mode string

const (
	ModeTransit Mode = "TRANSIT"
	ModeWalk    Mode = "WALK"
)

// WhenKind selects which time constraint (if any) a leg carries.
type WhenKind int

const (
	WhenNone   WhenKind = iota // walking: no time constraint
	WhenArrive                 // transit morning: be at the destination by At
	WhenDepart                 // transit evening: leave the origin at At
)

// When pins a transit leg to an arrive-by or depart-at instant.
type When struct {
	Kind WhenKind
	At   time.Time
}

// Leg is one origin→destination routing request, expressed in provider-agnostic
// terms so any Router can fulfill it.
type Leg struct {
	Origin string
	Dest   string
	Mode   Mode
	When   When
}

// NewRouter builds the routing backend named by provider. An empty name
// defaults to Google. To add a backend, implement Router and add a case here.
func NewRouter(provider string, s Settings) (Router, error) {
	switch provider {
	case "", string(RouterProviderGoogle):
		return newGoogleRouter(s, nil), nil
	default:
		return nil, fmt.Errorf("unknown commute provider %q (known: %s)", provider, RouterProviderGoogle)
	}
}
