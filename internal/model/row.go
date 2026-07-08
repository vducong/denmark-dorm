// Package model holds the source-agnostic data types shared between sources
// (which produce them) and the pipeline (which exports them).
package model

// WaitlistRow is one housing request, normalized across sources.
//
// RankDisplay is the rank exactly as shown by the source
// ("3" for KKIK, "B" for a letter-ranked source)
// and is what lands in the your_rank column.
// RankOrder is the same rank projected onto a sortable scale where lower is better;
// sorting and diffing run on it so both numeric and letter ranks work.
//
// Address is the street address used for commute estimation.
// Sources that expose one set it directly (s.dk);
// for sources that only have a dorm name (KKIK) the commute resolver fills it from config.
// Commute maps a commute column name (e.g. "cbs_transit_morning_min") to its
// value in whole minutes, or "" when unknown; it is always read through the
// pipeline's ordered column list, so iteration order never depends on the map.
//
// RentMin and RentMax are the monthly rent bounds in DKK (equal for a single
// value, both zero when unknown); a source that exposes only a range fills
// both, and scoring uses the midpoint while the budget gate uses the minimum.
type WaitlistRow struct {
	RequestID   string
	Dorm        string
	URL         string
	RoomType    string
	Size        string
	RankDisplay string
	RankOrder   int
	Address     string
	Commute     map[string]string
	RentMin     int
	RentMax     int
}

// Result is the parsed page content for one source.
type Result struct {
	Rows []WaitlistRow
	Meta Meta
}

// Meta holds optional header information from a source's list page.
type Meta struct {
	ApplicantName   string
	RenewalDeadline string
}
