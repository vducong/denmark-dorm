package sheets

import (
	"strconv"
	"testing"
	"time"

	"housing-waitlist/internal/export"
	"housing-waitlist/internal/model"
)

// numericOrder is a stand-in for a source's RankOrder over numeric ranks.
func numericOrder(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	return n, err == nil
}

func TestBuildMatrix_backfillAndAppend(t *testing.T) {
	snapshots := []export.DailySnapshot{
		{
			DateHeader: "260526",
			Date:       time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC),
			Ranks:      map[string]string{"a": "25"},
			Rows: map[string]model.WaitlistRow{
				"a": {RequestID: "a", Dorm: "D1", RoomType: "R1", Size: "S1", RankDisplay: "25", RankOrder: 25},
			},
		},
	}
	today := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	rows := []model.WaitlistRow{
		{RequestID: "a", Dorm: "D1", URL: "U1", RoomType: "R1", Size: "S1", RankDisplay: "22", RankOrder: 22},
	}

	matrix, err := BuildMatrix(rows, snapshots, nil, today, "", numericOrder, nil)
	if err != nil {
		t.Fatalf("BuildMatrix() err = %v", err)
	}
	if len(matrix) != 4 {
		t.Fatalf("len(matrix) = %d, want 4", len(matrix))
	}

	header := matrix[2]
	if header[2] != "url" || header[5] != latestDiffHeader || header[6] != "260526" || header[7] != "090626" {
		t.Errorf("header = %v", header)
	}

	data := matrix[3]
	if data[0] != "a" || data[2] != "U1" || data[5] != "+3" || data[6] != "25" || data[7] != "22" {
		t.Errorf("data row = %v", data)
	}
}

func TestBuildMatrix_sameDayUpdate(t *testing.T) {
	today := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	existing := [][]any{
		{"Last updated at", "2026-06-09T10:00:00Z"},
		{"request_id", "dorm", "room_type", "size_sqm", latestDiffHeader, "090626"},
		{"a", "D1", "R1", "S1", "", "30"},
	}
	rows := []model.WaitlistRow{
		{RequestID: "a", Dorm: "D1", RoomType: "R1", Size: "S1", RankDisplay: "22", RankOrder: 22},
	}

	matrix, err := BuildMatrix(rows, nil, existing, today, "", numericOrder, nil)
	if err != nil {
		t.Fatalf("BuildMatrix() err = %v", err)
	}
	if matrix[3][5] != "" {
		t.Errorf("latest_diff with single day = %v, want empty", matrix[3][5])
	}
	if matrix[3][6] != "22" {
		t.Errorf("today rank = %v, want 22", matrix[3][6])
	}
}

func TestBuildMatrix_legacyMigration(t *testing.T) {
	today := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	existing := [][]any{
		{"Last updated at", "2026-06-09T10:00:00Z"},
		{"request_id", "dorm", "room_type", "size_sqm", "your_rank", "diff"},
		{"a", "D1", "R1", "S1", "30", ""},
	}
	snapshots := []export.DailySnapshot{
		{
			DateHeader: "260526",
			Date:       time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC),
			Ranks:      map[string]string{"a": "25"},
		},
	}
	rows := []model.WaitlistRow{
		{RequestID: "a", Dorm: "D1", RoomType: "R1", Size: "S1", RankDisplay: "22", RankOrder: 22},
	}

	matrix, err := BuildMatrix(rows, snapshots, existing, today, "", numericOrder, nil)
	if err != nil {
		t.Fatalf("BuildMatrix() err = %v", err)
	}
	header := matrix[2]
	if header[5] != latestDiffHeader || header[6] != "260526" || header[7] != "090626" {
		t.Errorf("header = %v", header)
	}
	if matrix[3][5] != "+3" || matrix[3][6] != "25" || matrix[3][7] != "22" {
		t.Errorf("data row = %v", matrix[3])
	}
}

func TestBuildMatrix_sortedByTodayRank(t *testing.T) {
	today := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	rows := []model.WaitlistRow{
		{RequestID: "b", Dorm: "D2", RankDisplay: "20", RankOrder: 20},
		{RequestID: "a", Dorm: "D1", RankDisplay: "5", RankOrder: 5},
	}

	matrix, err := BuildMatrix(rows, nil, nil, today, "", numericOrder, nil)
	if err != nil {
		t.Fatalf("BuildMatrix() err = %v", err)
	}
	if matrix[3][0] != "a" || matrix[4][0] != "b" {
		t.Errorf("row order = %v then %v", matrix[3][0], matrix[4][0])
	}
}

// TestBuildMatrix_preCommuteSheetPreservesHistory is the migration-safety test:
// an existing sheet that predates the commute columns must keep its dated rank
// history (NOT be wiped) when commute columns are newly added.
func TestBuildMatrix_preCommuteSheetPreservesHistory(t *testing.T) {
	commuteCols := []string{"cbs_transit_morning_min", "cbs_transit_evening_min", "cbs_walk_min"}
	today := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	existing := [][]any{
		{"Last updated at", "2026-06-09T10:00:00Z"},
		{"note legend"},
		{"request_id", "dorm", "url", "room_type", "size_sqm", latestDiffHeader, "260526"},
		{"a", "D1", "U1", "R1", "S1", "", "25"},
	}
	rows := []model.WaitlistRow{
		{RequestID: "a", Dorm: "D1", URL: "U1", RoomType: "R1", Size: "S1", RankDisplay: "22", RankOrder: 22,
			Commute: map[string]string{"cbs_transit_morning_min": "28", "cbs_walk_min": "44"}},
	}

	matrix, err := BuildMatrix(rows, nil, existing, today, "note", numericOrder, commuteCols)
	if err != nil {
		t.Fatalf("BuildMatrix err = %v", err)
	}
	header := matrix[2]
	// base(5) + commute(3): latest_diff at 8, dated columns at 9+.
	if header[5] != "cbs_transit_morning_min" || header[8] != latestDiffHeader {
		t.Fatalf("header = %v", header)
	}
	if header[9] != "260526" || header[10] != "090626" {
		t.Errorf("date headers = %v", header[9:])
	}
	data := matrix[3]
	if data[9] != "25" || data[10] != "22" {
		t.Errorf("dated history not preserved: %v", data[9:])
	}
	if data[8] != "+3" {
		t.Errorf("latest_diff = %v, want +3", data[8])
	}
	if data[5] != "28" || data[6] != "" || data[7] != "44" {
		t.Errorf("commute cells = %v", data[5:8])
	}
}

func TestBuildMatrix_commuteColumnsFromScrape(t *testing.T) {
	commuteCols := []string{"cbs_transit_morning_min", "cbs_transit_evening_min", "cbs_walk_min"}
	today := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	rows := []model.WaitlistRow{
		{RequestID: "a", Dorm: "D1", RankDisplay: "5", RankOrder: 5,
			Commute: map[string]string{"cbs_transit_morning_min": "28", "cbs_transit_evening_min": "33", "cbs_walk_min": "44"}},
	}
	matrix, err := BuildMatrix(rows, nil, nil, today, "", numericOrder, commuteCols)
	if err != nil {
		t.Fatalf("BuildMatrix err = %v", err)
	}
	if h := matrix[2]; h[5] != "cbs_transit_morning_min" || h[7] != "cbs_walk_min" {
		t.Fatalf("header commute = %v", h)
	}
	if d := matrix[3]; d[5] != "28" || d[6] != "33" || d[7] != "44" {
		t.Errorf("commute data = %v", d[5:8])
	}
}

func TestSheetDiff(t *testing.T) {
	rankCells := map[string]string{
		"260526": "25",
		"090626": "22",
	}
	got := sheetDiff("090626", []string{"260526", "090626"}, rankCells, numericOrder)
	if got != "+3" {
		t.Errorf("sheetDiff() = %q, want +3", got)
	}
}
