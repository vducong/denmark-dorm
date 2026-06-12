package export

import (
	"testing"

	"housing-waitlist/internal/model"
)

func TestRecords_sortedByRank(t *testing.T) {
	rows := []model.WaitlistRow{
		{RequestID: "b", Dorm: "B", RankDisplay: "20", RankOrder: 20},
		{RequestID: "a", Dorm: "A", URL: "https://example.test/a", RankDisplay: "5", RankOrder: 5},
	}
	recs := Records(rows, nil)
	if len(recs) != 3 {
		t.Fatalf("len = %d, want 3", len(recs))
	}
	if recs[0][0] != "request_id" || recs[0][2] != "url" || recs[0][6] != "diff" {
		t.Errorf("header = %v", recs[0])
	}
	if recs[1][0] != "a" || recs[1][2] != "https://example.test/a" || recs[1][5] != "5" || recs[1][6] != "" {
		t.Errorf("first data row = %v", recs[1])
	}
	if recs[2][0] != "b" {
		t.Errorf("second data row = %v", recs[2])
	}
}

func TestRecords_diff(t *testing.T) {
	rows := []model.WaitlistRow{
		{RequestID: "a", Dorm: "A", RankDisplay: "10", RankOrder: 10},
		{RequestID: "b", Dorm: "B", RankDisplay: "30", RankOrder: 30},
	}
	prev := map[string]int{"a": 15, "b": 25}
	recs := Records(rows, prev)
	if recs[1][6] != "+5" {
		t.Errorf("improved row diff = %q, want +5", recs[1][6])
	}
	if recs[2][6] != "-5" {
		t.Errorf("worsened row diff = %q, want -5", recs[2][6])
	}
}

func TestRecords_diff_newListing(t *testing.T) {
	rows := []model.WaitlistRow{{RequestID: "new", Dorm: "D", RankDisplay: "1", RankOrder: 1}}
	prev := map[string]int{"old": 99}
	recs := Records(rows, prev)
	if recs[1][6] != "" {
		t.Errorf("new listing diff = %q, want empty", recs[1][6])
	}
}
