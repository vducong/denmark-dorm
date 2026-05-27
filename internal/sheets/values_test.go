package sheets

import (
	"reflect"
	"testing"

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
