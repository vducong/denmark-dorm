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
type WaitlistRow struct {
	RequestID   string
	Dorm        string
	URL         string
	RoomType    string
	Size        string
	RankDisplay string
	RankOrder   int
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
