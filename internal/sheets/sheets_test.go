package sheets

import "testing"

func TestSheetRange(t *testing.T) {
	tests := []struct {
		name, sheet, cells, want string
	}{
		{"simple", "Sheet1", "A1", "Sheet1!A1"},
		{"spaces", "Wait list", "A:Z", "'Wait list'!A:Z"},
		{"quote", "Bob's", "A1", "'Bob''s'!A1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := sheetRange(tc.sheet, tc.cells); got != tc.want {
				t.Errorf("sheetRange() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestMatrixWidth(t *testing.T) {
	values := [][]any{
		{"Last updated at", "2026-06-10T00:00:00Z"},
		{"request_id", "dorm", "room_type", "size_sqm", "latest_diff", "100626"},
		{"1", "D", "R", "S", "+1", "5"},
	}
	if got := matrixWidth(values); got != 6 {
		t.Errorf("matrixWidth() = %d, want 6", got)
	}
}

func TestColumnLetter(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{1, "A"},
		{6, "F"},
		{26, "Z"},
		{27, "AA"},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			if got := columnLetter(tc.n); got != tc.want {
				t.Errorf("columnLetter(%d) = %q, want %q", tc.n, got, tc.want)
			}
		})
	}
}
