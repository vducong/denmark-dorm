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
