package export

import (
	"testing"

	"denmark-housing-waitlist/internal/parser"
)

func TestRecords_sortedByRank(t *testing.T) {
	rows := []parser.WaitlistRow{
		{RequestID: "b", Dorm: "B", YourRank: 20},
		{RequestID: "a", Dorm: "A", YourRank: 5},
	}
	recs := Records(rows, nil)
	if len(recs) != 3 {
		t.Fatalf("len = %d, want 3", len(recs))
	}
	if recs[0][0] != "request_id" || recs[0][5] != "diff" {
		t.Errorf("header = %v", recs[0])
	}
	if recs[1][0] != "a" || recs[1][4] != "5" || recs[1][5] != "" {
		t.Errorf("first data row = %v", recs[1])
	}
	if recs[2][0] != "b" {
		t.Errorf("second data row = %v", recs[2])
	}
}

func TestRecords_diff(t *testing.T) {
	rows := []parser.WaitlistRow{
		{RequestID: "a", Dorm: "A", YourRank: 10},
		{RequestID: "b", Dorm: "B", YourRank: 30},
	}
	prev := map[string]int{"a": 15, "b": 25}
	recs := Records(rows, prev)
	if recs[1][5] != "5" {
		t.Errorf("improved row diff = %q, want 5", recs[1][5])
	}
	if recs[2][5] != "-5" {
		t.Errorf("worsened row diff = %q, want -5", recs[2][5])
	}
}

func TestRecords_diff_newListing(t *testing.T) {
	rows := []parser.WaitlistRow{{RequestID: "new", Dorm: "D", YourRank: 1}}
	prev := map[string]int{"old": 99}
	recs := Records(rows, prev)
	if recs[1][5] != "" {
		t.Errorf("new listing diff = %q, want empty", recs[1][5])
	}
}
