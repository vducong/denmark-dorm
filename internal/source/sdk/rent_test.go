package sdk

import (
	"os"
	"testing"
)

func TestParseHTML_attachesRentBySize(t *testing.T) {
	html, err := os.ReadFile("testdata/building_rent.html")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	res, err := parseHTML(string(html))
	if err != nil {
		t.Fatalf("parseHTML: %v", err)
	}

	want := map[string][2]int{
		"16775": {2588, 2875}, // 37 m² -> 1-room group
		"3513":  {3612, 3612}, // 53 m² -> 2-room group
		"9999":  {0, 0},       // 99 m² -> no group, rent stays unknown
	}
	for _, row := range res.Rows {
		w, ok := want[row.RequestID]
		if !ok {
			t.Errorf("unexpected row %s", row.RequestID)
			continue
		}
		if row.RentMin != w[0] || row.RentMax != w[1] {
			t.Errorf("tenancy %s rent = %d-%d, want %d-%d", row.RequestID, row.RentMin, row.RentMax, w[0], w[1])
		}
	}
}

func TestRentForSize(t *testing.T) {
	groups := []rentGroup{
		{sizeMin: 37, sizeMax: 37, rentMin: 2588, rentMax: 2875},
		{sizeMin: 50, sizeMax: 55, rentMin: 3612, rentMax: 3612},
	}
	cases := []struct {
		size     string
		min, max int
		ok       bool
	}{
		{"37", 2588, 2875, true},
		{"53", 3612, 3612, true}, // within 50-55 band
		{"99", 0, 0, false},
		{"", 0, 0, false},
	}
	for _, c := range cases {
		min, max, ok := rentForSize(groups, c.size)
		if ok != c.ok || (ok && (min != c.min || max != c.max)) {
			t.Errorf("rentForSize(%q) = (%d,%d,%v), want (%d,%d,%v)", c.size, min, max, ok, c.min, c.max, c.ok)
		}
	}
}
