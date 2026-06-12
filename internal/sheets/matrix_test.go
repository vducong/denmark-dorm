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

	matrix, err := BuildMatrix(rows, snapshots, nil, today, "", numericOrder)
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

	matrix, err := BuildMatrix(rows, nil, existing, today, "", numericOrder)
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

	matrix, err := BuildMatrix(rows, snapshots, existing, today, "", numericOrder)
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

	matrix, err := BuildMatrix(rows, nil, nil, today, "", numericOrder)
	if err != nil {
		t.Fatalf("BuildMatrix() err = %v", err)
	}
	if matrix[3][0] != "a" || matrix[4][0] != "b" {
		t.Errorf("row order = %v then %v", matrix[3][0], matrix[4][0])
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
