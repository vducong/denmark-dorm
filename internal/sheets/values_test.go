package sheets

import (
	"reflect"
	"testing"
	"time"

	"denmark-housing-waitlist/internal/export"
	"denmark-housing-waitlist/internal/parser"
)

func TestRecordsToValues_includesHeaderRow(t *testing.T) {
	records := export.Records([]parser.WaitlistRow{
		{RequestID: "1", Dorm: "D", YourRank: 3},
	})
	values := recordsToValues(records)
	if len(values) != 2 {
		t.Fatalf("len = %d, want 2 (header + 1 row)", len(values))
	}
	if !reflect.DeepEqual(values[0], []interface{}{"request_id", "dorm", "room_type", "size_sqm", "your_rank"}) {
		t.Errorf("header row = %v", values[0])
	}
}

func TestSheetValues_lastUpdatedMetadata(t *testing.T) {
	records := export.Records([]parser.WaitlistRow{
		{RequestID: "1", Dorm: "D", YourRank: 3},
	})
	updatedAt := time.Date(2026, 5, 26, 15, 4, 5, 0, time.UTC)
	values := sheetValues(records, updatedAt)

	if len(values) != 3 {
		t.Fatalf("len = %d, want 3 (meta + header + 1 row)", len(values))
	}
	if !reflect.DeepEqual(values[0], []interface{}{"Last updated at", "2026-05-26T15:04:05Z"}) {
		t.Errorf("meta row = %v", values[0])
	}
	if values[1][0] != "request_id" {
		t.Errorf("header row = %v", values[1])
	}
}
