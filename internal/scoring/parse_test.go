package scoring

import "testing"

const eps = 1e-9

func almost(a, b float64) bool {
	d := a - b
	return d < eps && d > -eps
}

func TestParseSize(t *testing.T) {
	tests := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"25,2 m²", 25.2, true},  // Danish decimal comma
		{"43-45 m²", 44, true},   // range -> midpoint
		{"39-42 m²", 40.5, true}, // range -> midpoint
		{"39 (27) m²", 39, true}, // parenthetical -> first number
		{"33", 33, true},         // SDK bare digits
		{"11 m2", 11, true},
		{"", 0, false},
		{"n/a", 0, false},
	}
	for _, tt := range tests {
		got, ok := parseSize(tt.in)
		if ok != tt.ok {
			t.Errorf("parseSize(%q) ok = %v, want %v", tt.in, ok, tt.ok)
			continue
		}
		if ok && !almost(got, tt.want) {
			t.Errorf("parseSize(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestRentValue(t *testing.T) {
	tests := []struct {
		min, max int
		want     float64
		ok       bool
	}{
		{4000, 6000, 5000, true}, // range -> midpoint
		{4000, 4000, 4000, true}, // single value
		{0, 6000, 6000, true},    // only max known
		{4000, 0, 4000, true},    // only min known
		{0, 0, 0, false},         // unknown
	}
	for _, tt := range tests {
		got, ok := rentValue(tt.min, tt.max)
		if ok != tt.ok {
			t.Errorf("rentValue(%d,%d) ok = %v, want %v", tt.min, tt.max, ok, tt.ok)
			continue
		}
		if ok && !almost(got, tt.want) {
			t.Errorf("rentValue(%d,%d) = %v, want %v", tt.min, tt.max, got, tt.want)
		}
	}
}

func TestRentExceeds(t *testing.T) {
	tests := []struct {
		min, max, maxRent int
		want              bool
	}{
		{7000, 9000, 6000, true},  // cheapest still over budget -> excluded
		{4000, 8000, 6000, false}, // range straddles budget -> kept (min under)
		{0, 0, 6000, false},       // unknown rent -> never excluded
		{4000, 6000, 0, false},    // no budget gate
	}
	for _, tt := range tests {
		if got := rentExceeds(tt.min, tt.max, tt.maxRent); got != tt.want {
			t.Errorf("rentExceeds(%d,%d,%d) = %v, want %v", tt.min, tt.max, tt.maxRent, got, tt.want)
		}
	}
}
