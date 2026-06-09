package sheets

import (
	"testing"
	"time"

	"denmark-housing-waitlist/internal/export"
	"denmark-housing-waitlist/internal/parser"
)

func TestBuildMatrix_backfillAndAppend(t *testing.T) {
	snapshots := []export.DailySnapshot{
		{
			DateHeader: "260526",
			Date:       time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC),
			Ranks:      map[string]int{"a": 25},
			Rows: map[string]parser.WaitlistRow{
				"a": {RequestID: "a", Dorm: "D1", RoomType: "R1", Size: "S1", YourRank: 25},
			},
		},
	}
	today := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	rows := []parser.WaitlistRow{
		{RequestID: "a", Dorm: "D1", RoomType: "R1", Size: "S1", YourRank: 22},
	}

	matrix, err := BuildMatrix(rows, snapshots, nil, today)
	if err != nil {
		t.Fatalf("BuildMatrix() err = %v", err)
	}
	if len(matrix) != 3 {
		t.Fatalf("len(matrix) = %d, want 3", len(matrix))
	}

	header := matrix[1]
	if header[4] != latestDiffHeader || header[5] != "260526" || header[6] != "090626" {
		t.Errorf("header = %v", header)
	}

	data := matrix[2]
	if data[0] != "a" || data[4] != "+3" || data[5] != "25" || data[6] != "22" {
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
	rows := []parser.WaitlistRow{
		{RequestID: "a", Dorm: "D1", RoomType: "R1", Size: "S1", YourRank: 22},
	}

	matrix, err := BuildMatrix(rows, nil, existing, today)
	if err != nil {
		t.Fatalf("BuildMatrix() err = %v", err)
	}
	if matrix[2][4] != "" {
		t.Errorf("latest_diff with single day = %v, want empty", matrix[2][4])
	}
	if matrix[2][5] != "22" {
		t.Errorf("today rank = %v, want 22", matrix[2][5])
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
			Ranks:      map[string]int{"a": 25},
		},
	}
	rows := []parser.WaitlistRow{
		{RequestID: "a", Dorm: "D1", RoomType: "R1", Size: "S1", YourRank: 22},
	}

	matrix, err := BuildMatrix(rows, snapshots, existing, today)
	if err != nil {
		t.Fatalf("BuildMatrix() err = %v", err)
	}
	header := matrix[1]
	if header[4] != latestDiffHeader || header[5] != "260526" || header[6] != "090626" {
		t.Errorf("header = %v", header)
	}
	if matrix[2][4] != "+3" || matrix[2][5] != "25" || matrix[2][6] != "22" {
		t.Errorf("data row = %v", matrix[2])
	}
}

func TestBuildMatrix_sortedByTodayRank(t *testing.T) {
	today := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	rows := []parser.WaitlistRow{
		{RequestID: "b", Dorm: "D2", YourRank: 20},
		{RequestID: "a", Dorm: "D1", YourRank: 5},
	}

	matrix, err := BuildMatrix(rows, nil, nil, today)
	if err != nil {
		t.Fatalf("BuildMatrix() err = %v", err)
	}
	if matrix[2][0] != "a" || matrix[3][0] != "b" {
		t.Errorf("row order = %v then %v", matrix[2][0], matrix[3][0])
	}
}

func TestSheetDiff(t *testing.T) {
	rankCells := map[string]string{
		"260526": "25",
		"090626": "22",
	}
	got := sheetDiff("a", "090626", []string{"260526", "090626"}, rankCells)
	if got != "+3" {
		t.Errorf("sheetDiff() = %q, want +3", got)
	}
}
