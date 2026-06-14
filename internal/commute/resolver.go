// Package commute estimates travel time from each dorm to one or more campuses
// and writes the results onto each row. The routing backend is pluggable (see
// Router / NewRouter); the Google Maps adapter is the default. It is
// source-agnostic: the runner builds one Resolver per run and calls Enrich
// between parsing and exporting. Values are cached to disk, so the backend is
// hit only on a cache miss.
package commute

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"housing-waitlist/internal/model"
)

// Destination is one campus to route to. Name prefixes its output columns.
type Destination struct {
	Name    string
	Address string
}

// Settings is the resolver's source-agnostic configuration. ArriveBy and
// DepartAt are concrete future weekday instants (resolved by the runner from
// the configured times of day) so the backend never sees a past time. Provider
// selects the routing backend (see NewRouter); APIKey is consumed by whichever
// adapter needs one.
type Settings struct {
	Provider      string
	APIKey        string
	Destinations  []Destination
	ArriveBy      time.Time
	DepartAt      time.Time
	DormAddresses map[string]string
	CachePath     string
}

// AddressLookup derives a street address from a row's detail-page URL, for
// sources whose rows carry only a name + URL (KKIK). The resolver caches the
// result by building, so one detail page per building is fetched — not one per
// room, since every room in a building shares the building's address.
type AddressLookup func(ctx context.Context, detailURL string) (string, error)

// Resolver enriches rows with commute estimates.
type Resolver struct {
	settings Settings
	router   Router
	cache    *cache
}

// New builds a Resolver around a routing backend and loads its disk cache once.
// The router is injected (built via NewRouter in production, or a stub in
// tests), so the resolver is fully decoupled from any specific provider.
func New(s Settings, router Router) (*Resolver, error) {
	c, err := loadCache(s.CachePath)
	if err != nil {
		return nil, err
	}
	return &Resolver{
		settings: s,
		router:   router,
		cache:    c,
	}, nil
}

// Column-name helpers are the single source of truth for commute column names,
// shared by the CSV, sheet, and email writers.
func TransitMorningCol(dest string) string { return dest + "_transit_morning_min" }
func TransitEveningCol(dest string) string { return dest + "_transit_evening_min" }
func WalkCol(dest string) string           { return dest + "_walk_min" }

// ColumnNames returns the ordered commute columns for the given destinations:
// per destination, morning transit, evening transit, then walk.
func ColumnNames(dests []Destination) []string {
	cols := make([]string, 0, len(dests)*3)
	for _, d := range dests {
		cols = append(cols, TransitMorningCol(d.Name), TransitEveningCol(d.Name), WalkCol(d.Name))
	}
	return cols
}

// Enrich fills each row's Commute map for every configured destination.
//
// Commute is a property of the building, not the room: every room in a building
// shares one origin, so Enrich resolves and routes once per building (keyed on
// the row's Dorm) and fans the result out to all of that building's rooms. This
// caps API calls at one building's worth — #buildings × destinations × legs —
// no matter how many rooms a building has, or whether a source hands us a
// distinct per-unit address per room (s.dk does).
//
// lookupAddr (may be nil) discovers an address for rows that carry only a name +
// URL. Enrich never returns an error that should abort a scrape: a per-building
// or per-leg failure logs a warning and leaves those cells blank. The cache is
// flushed once at the end.
func (r *Resolver) Enrich(ctx context.Context, rows []model.WaitlistRow, lookupAddr AddressLookup) error {
	arrKey := "arr" + r.settings.ArriveBy.Format("1504")
	depKey := "dep" + r.settings.DepartAt.Format("1504")

	// Group row indices by building, preserving first-appearance order so the
	// representative row (and thus the cache key) is stable across runs.
	byBuilding := map[string][]int{}
	var order []string
	for i := range rows {
		d := rows[i].Dorm
		if _, ok := byBuilding[d]; !ok {
			order = append(order, d)
		}
		byBuilding[d] = append(byBuilding[d], i)
	}

	var unmapped []string
	for _, dorm := range order {
		idxs := byBuilding[dorm]
		origin := r.addressFor(ctx, rows[idxs[0]], lookupAddr)
		if origin == "" {
			unmapped = append(unmapped, dorm)
			continue
		}
		building := map[string]string{}
		for _, d := range r.settings.Destinations {
			dest := geoAddr(d.Address)
			// Morning: home -> campus, arrive before the first class.
			r.fill(ctx, building, TransitMorningCol(d.Name),
				cacheKey("transit", arrKey, origin, dest),
				Leg{Origin: origin, Dest: dest, Mode: ModeTransit, When: When{Kind: WhenArrive, At: r.settings.ArriveBy}})
			// Evening: campus -> home, leave at the worst-case time.
			r.fill(ctx, building, TransitEveningCol(d.Name),
				cacheKey("transit", depKey, dest, origin),
				Leg{Origin: dest, Dest: origin, Mode: ModeTransit, When: When{Kind: WhenDepart, At: r.settings.DepartAt}})
			// Walk: symmetric and time-independent, so one value per campus.
			r.fill(ctx, building, WalkCol(d.Name),
				cacheKey("walk", "", origin, dest),
				Leg{Origin: origin, Dest: dest, Mode: ModeWalk})
		}
		for _, i := range idxs {
			if rows[i].Commute == nil {
				rows[i].Commute = map[string]string{}
			}
			for k, v := range building {
				rows[i].Commute[k] = v
			}
		}
	}

	if len(unmapped) > 0 {
		slog.Warn("commute: no origin address for some buildings", "buildings", unmapped)
	}
	if err := r.cache.flush(r.settings.CachePath); err != nil {
		slog.Warn("commute: cache flush failed", "err", err)
	}
	return nil
}

// fill resolves one leg: cache hit short-circuits the network; a successful
// lookup (including a real "no route" blank) is cached; a failure is logged and
// left blank without caching, so it retries next run.
func (r *Resolver) fill(ctx context.Context, dst map[string]string, col, key string, leg Leg) {
	if v, ok := r.cache.get(key); ok {
		dst[col] = v
		return
	}
	v, err := r.router.Minutes(ctx, leg)
	if err != nil {
		slog.Warn("commute: route lookup failed", "col", col, "origin", leg.Origin, "dest", leg.Dest, "err", err)
		return
	}
	r.cache.put(key, v)
	dst[col] = v
}

// addressFor resolves a row's origin address in precedence order:
//  1. the address the source parsed (s.dk),
//  2. a manual override from config (commute.dorm_addresses),
//  3. auto-discovery from the row's detail page via lookupAddr.
//
// Discovered addresses are cached by *building* (row.Dorm), not by detail URL:
// every room in a building shares the building's address, so one detail-page
// fetch per building per run suffices — and is reused across runs even for room
// URLs never seen before. Returns "" when none yields an address.
func (r *Resolver) addressFor(ctx context.Context, row model.WaitlistRow, lookupAddr AddressLookup) string {
	if a := geoAddr(row.Address); a != "" {
		return a
	}
	if a := geoAddr(r.settings.DormAddresses[row.Dorm]); a != "" {
		return a
	}
	if lookupAddr == nil {
		return ""
	}
	key := addrCacheKey(row)
	if v, ok := r.cache.get(key); ok {
		return v
	}
	if row.URL == "" {
		return ""
	}
	addr, err := lookupAddr(ctx, row.URL)
	if err != nil {
		slog.Warn("commute: address lookup failed", "dorm", row.Dorm, "url", row.URL, "err", err)
		return ""
	}
	geo := geoAddr(addr)
	r.cache.put(key, geo)
	return geo
}

// addrCacheKey keys a discovered address by building so every room in the same
// building reuses one lookup. It falls back to the detail URL only when a row
// carries no building name.
func addrCacheKey(row model.WaitlistRow) string {
	if row.Dorm != "" {
		return "addr|dorm|" + row.Dorm
	}
	return "addr|url|" + row.URL
}

// geoAddr normalizes whitespace and appends ", Denmark" when absent, to steer
// the Routes API's geocoding to the right country. Used for both the API call
// and the cache key so the two always agree.
func geoAddr(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return ""
	}
	if !strings.Contains(strings.ToLower(s), "denmark") {
		s += ", Denmark"
	}
	return s
}

func cacheKey(mode, timePart, origin, dest string) string {
	return strings.Join([]string{mode, timePart, origin, dest}, "|")
}
