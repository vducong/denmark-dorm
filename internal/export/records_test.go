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
	recs := Records(rows)
	if len(recs) != 3 {
		t.Fatalf("len = %d, want 3", len(recs))
	}
	if recs[0][0] != "request_id" {
		t.Errorf("header[0] = %q", recs[0][0])
	}
	if recs[1][0] != "a" || recs[1][4] != "5" {
		t.Errorf("first data row = %v", recs[1])
	}
	if recs[2][0] != "b" {
		t.Errorf("second data row = %v", recs[2])
	}
}
