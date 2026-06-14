package commute

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"housing-waitlist/internal/model"
)

// failDoer fails the test if any network call is made.
type failDoer struct{ t *testing.T }

func (f failDoer) Do(*http.Request) (*http.Response, error) {
	f.t.Helper()
	f.t.Fatalf("unexpected network call")
	return nil, nil
}

func TestColumnNames_order(t *testing.T) {
	cols := ColumnNames([]Destination{{Name: "cbs"}, {Name: "rda"}})
	want := []string{
		"cbs_transit_morning_min", "cbs_transit_evening_min", "cbs_walk_min",
		"rda_transit_morning_min", "rda_transit_evening_min", "rda_walk_min",
	}
	if len(cols) != len(want) {
		t.Fatalf("len = %d, want %d", len(cols), len(want))
	}
	for i := range want {
		if cols[i] != want[i] {
			t.Errorf("col[%d] = %q, want %q", i, cols[i], want[i])
		}
	}
}

func newResolver(t *testing.T, doer HTTPDoer) *Resolver {
	t.Helper()
	s := Settings{
		APIKey:        "k",
		Destinations:  []Destination{{Name: "cbs", Address: "Solbjerg Plads 3, 2000 Frederiksberg, Denmark"}},
		ArriveBy:      time.Date(2026, 6, 15, 8, 0, 0, 0, time.UTC),
		DepartAt:      time.Date(2026, 6, 15, 17, 0, 0, 0, time.UTC),
		DormAddresses: map[string]string{"KKIK Dorm": "Tagensvej 15, 2200 København N"},
		CachePath:     "", // no disk in tests
	}
	r, err := New(s, newGoogleRouter(s, doer))
	if err != nil {
		t.Fatalf("New() err = %v", err)
	}
	return r
}

// recordRouter is a stub Router that records the legs it receives, proving the
// resolver runs against any backend — not just Google.
type recordRouter struct {
	legs []Leg
	ret  string
}

func (r *recordRouter) Minutes(_ context.Context, leg Leg) (string, error) {
	r.legs = append(r.legs, leg)
	return r.ret, nil
}

func TestEnrich_usesInjectedRouter(t *testing.T) {
	rr := &recordRouter{ret: "12"}
	s := Settings{
		Destinations: []Destination{{Name: "cbs", Address: "Solbjerg Plads 3, 2000 Frederiksberg, Denmark"}},
		ArriveBy:     time.Date(2026, 6, 15, 8, 0, 0, 0, time.UTC),
		DepartAt:     time.Date(2026, 6, 15, 17, 0, 0, 0, time.UTC),
	}
	r, err := New(s, rr)
	if err != nil {
		t.Fatalf("New() err = %v", err)
	}
	rows := []model.WaitlistRow{{RequestID: "1", Dorm: "X", Address: "Nørrebrogade 9E, 2200 København N"}}
	if err := r.Enrich(context.Background(), rows, nil); err != nil {
		t.Fatalf("Enrich err = %v", err)
	}
	if len(rr.legs) != 3 {
		t.Fatalf("legs = %d, want 3", len(rr.legs))
	}
	campus := geoAddr("Solbjerg Plads 3, 2000 Frederiksberg, Denmark")
	home := geoAddr("Nørrebrogade 9E, 2200 København N")
	// Morning transit: home -> campus, arrive-by.
	if rr.legs[0].Mode != ModeTransit || rr.legs[0].When.Kind != WhenArrive ||
		rr.legs[0].Origin != home || rr.legs[0].Dest != campus {
		t.Errorf("morning leg = %+v", rr.legs[0])
	}
	// Evening transit: campus -> home (direction swapped), depart-at.
	if rr.legs[1].Mode != ModeTransit || rr.legs[1].When.Kind != WhenDepart ||
		rr.legs[1].Origin != campus || rr.legs[1].Dest != home {
		t.Errorf("evening leg = %+v", rr.legs[1])
	}
	// Walk: no time constraint.
	if rr.legs[2].Mode != ModeWalk || rr.legs[2].When.Kind != WhenNone {
		t.Errorf("walk leg = %+v", rr.legs[2])
	}
	if rows[0].Commute["cbs_transit_morning_min"] != "12" {
		t.Errorf("commute = %v", rows[0].Commute)
	}
}

func TestEnrich_cacheHitNoNetwork(t *testing.T) {
	r := newResolver(t, failDoer{t})
	origin := geoAddr("Nørrebrogade 9E, 2200 København N")
	dest := geoAddr("Solbjerg Plads 3, 2000 Frederiksberg, Denmark")
	r.cache.put(cacheKey("transit", "arr0800", origin, dest), "28")
	r.cache.put(cacheKey("transit", "dep1700", dest, origin), "31")
	r.cache.put(cacheKey("walk", "", origin, dest), "44")

	rows := []model.WaitlistRow{{RequestID: "1", Dorm: "X", Address: "Nørrebrogade 9E, 2200 København N"}}
	if err := r.Enrich(context.Background(), rows, nil); err != nil {
		t.Fatalf("Enrich err = %v", err)
	}
	got := rows[0].Commute
	if got["cbs_transit_morning_min"] != "28" || got["cbs_transit_evening_min"] != "31" || got["cbs_walk_min"] != "44" {
		t.Errorf("commute = %v", got)
	}
}

func TestEnrich_kkikUsesDormMap(t *testing.T) {
	f := &fakeDoer{}
	r := newResolver(t, f)
	rows := []model.WaitlistRow{{RequestID: "2", Dorm: "KKIK Dorm"}} // no Address -> dorm map
	if err := r.Enrich(context.Background(), rows, nil); err != nil {
		t.Fatalf("Enrich err = %v", err)
	}
	if len(f.requests) != 3 {
		t.Fatalf("calls = %d, want 3", len(f.requests))
	}
	if !strings.Contains(f.bodies[0], "Tagensvej 15") {
		t.Errorf("origin not taken from dorm map: %s", f.bodies[0])
	}
	if rows[0].Commute["cbs_walk_min"] != "34" {
		t.Errorf("walk = %q, want 34", rows[0].Commute["cbs_walk_min"])
	}
}

func TestEnrich_unmappedDormStaysBlank(t *testing.T) {
	r := newResolver(t, failDoer{t}) // must not hit the network for an unmapped dorm
	rows := []model.WaitlistRow{{RequestID: "3", Dorm: "Unknown"}}
	if err := r.Enrich(context.Background(), rows, nil); err != nil {
		t.Fatalf("Enrich err = %v", err)
	}
	if len(rows[0].Commute) != 0 {
		t.Errorf("expected no commute values, got %v", rows[0].Commute)
	}
}

func TestEnrich_addressLookedUpOncePerBuilding(t *testing.T) {
	f := &fakeDoer{}
	r := newResolver(t, f)
	calls := 0
	lookup := func(_ context.Context, _ string) (string, error) {
		calls++
		return "Strandlodsvej 13K, København S", nil
	}
	// Two rooms in the SAME building, each with its OWN detail URL. The address
	// is the building's, so exactly one detail page should be fetched.
	rows := []model.WaitlistRow{
		{RequestID: "1", Dorm: "Aksehuset", URL: "https://kkik.example/detail?arid=1"},
		{RequestID: "2", Dorm: "Aksehuset", URL: "https://kkik.example/detail?arid=2"},
	}
	if err := r.Enrich(context.Background(), rows, lookup); err != nil {
		t.Fatalf("Enrich err = %v", err)
	}
	if calls != 1 {
		t.Fatalf("address lookups = %d, want 1 (one per building, not per room)", calls)
	}
	// Both rooms receive the building's commute values.
	for i := range rows {
		if rows[i].Commute["cbs_walk_min"] != "34" {
			t.Errorf("row %d walk = %q, want 34", i, rows[i].Commute["cbs_walk_min"])
		}
	}
	if !strings.Contains(f.bodies[0], "Strandlodsvej 13K") {
		t.Errorf("origin not from looked-up address: %s", f.bodies[0])
	}
	// A later run with a brand-new room URL in the same building still does not
	// re-fetch — the building's address is cached.
	rows2 := []model.WaitlistRow{{RequestID: "3", Dorm: "Aksehuset", URL: "https://kkik.example/detail?arid=99"}}
	if err := r.Enrich(context.Background(), rows2, lookup); err != nil {
		t.Fatalf("second Enrich err = %v", err)
	}
	if calls != 1 {
		t.Errorf("lookup called again for a new room in a known building; calls = %d, want 1", calls)
	}
}

func TestEnrich_routesOncePerBuildingAcrossRoomAddresses(t *testing.T) {
	f := &fakeDoer{}
	r := newResolver(t, f) // one destination (cbs)
	// Three rooms in ONE building, each with its own full per-unit address — the
	// s.dk shape. Commute must be computed once for the building, not per room.
	rows := []model.WaitlistRow{
		{RequestID: "1", Dorm: "Tranehavegård", Address: "Tranehavevej 1 2 206, 2450 København SV"},
		{RequestID: "2", Dorm: "Tranehavegård", Address: "Tranehavevej 1 4 101, 2450 København SV"},
		{RequestID: "3", Dorm: "Tranehavegård", Address: "Tranehavevej 1 1 003, 2450 København SV"},
	}
	if err := r.Enrich(context.Background(), rows, nil); err != nil {
		t.Fatalf("Enrich err = %v", err)
	}
	// One building × one destination × three legs = exactly 3 route calls,
	// not 9 (the per-room count that caused the API blow-up).
	if len(f.requests) != 3 {
		t.Fatalf("route calls = %d, want 3 (one building × 3 legs)", len(f.requests))
	}
	// Every room receives the building's commute values.
	for i := range rows {
		if rows[i].Commute["cbs_walk_min"] != "34" {
			t.Errorf("room %d walk = %q, want 34", i, rows[i].Commute["cbs_walk_min"])
		}
	}
	// The first room's address is the building representative.
	if !strings.Contains(f.bodies[0], "Tranehavevej 1 2 206") {
		t.Errorf("origin not the building representative: %s", f.bodies[0])
	}
}

func TestEnrich_legDirectionAndTime(t *testing.T) {
	f := &fakeDoer{}
	r := newResolver(t, f)
	rows := []model.WaitlistRow{{RequestID: "4", Dorm: "X", Address: "Some Street 1, 2200 København N"}}
	if err := r.Enrich(context.Background(), rows, nil); err != nil {
		t.Fatalf("Enrich err = %v", err)
	}
	if len(f.bodies) != 3 {
		t.Fatalf("bodies = %d, want 3", len(f.bodies))
	}
	// Morning: arrive-by, origin is the dorm.
	if !strings.Contains(f.bodies[0], "arrivalTime") || !strings.Contains(f.bodies[0], "Some Street 1") {
		t.Errorf("morning body = %s", f.bodies[0])
	}
	// Evening: depart-at, origin is the campus (direction swapped).
	if !strings.Contains(f.bodies[1], "departureTime") || !strings.Contains(f.bodies[1], "Solbjerg Plads") {
		t.Errorf("evening body = %s", f.bodies[1])
	}
	// Walk: no time constraint.
	if strings.Contains(f.bodies[2], "Time") {
		t.Errorf("walk body should omit time: %s", f.bodies[2])
	}
}
