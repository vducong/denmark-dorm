package kkik

import (
	"os"
	"testing"
)

func TestParseCatalog(t *testing.T) {
	html, err := os.ReadFile(testdataPath(t, "kollegiumlist.html"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	rows, err := parseCatalog(string(html))
	if err != nil {
		t.Fatalf("parseCatalog: %v", err)
	}

	byID := map[string]struct {
		dorm string
		size string
		rent int
	}{
		"1788": {"Testkollegiet A", "25,2 m2", 3701},
		"1789": {"Testkollegiet A", "25,2 m2", 3701},
		"1790": {"Testkollegiet A", "41,5 m2", 5957},
		"1791": {"Testkollegiet A", "49,8 m2", 7076},
	}
	if len(rows) != len(byID) {
		t.Fatalf("got %d rows, want %d: %+v", len(rows), len(byID), rows)
	}
	for _, r := range rows {
		want, ok := byID[r.RequestID]
		if !ok {
			t.Errorf("unexpected row for arid %q: %+v", r.RequestID, r)
			continue
		}
		if r.Dorm != want.dorm {
			t.Errorf("arid %s: Dorm = %q, want %q", r.RequestID, r.Dorm, want.dorm)
		}
		if r.Size != want.size {
			t.Errorf("arid %s: Size = %q, want %q", r.RequestID, r.Size, want.size)
		}
		if r.RentMin != want.rent || r.RentMax != want.rent {
			t.Errorf("arid %s: RentMin/RentMax = %d/%d, want %d", r.RequestID, r.RentMin, r.RentMax, want.rent)
		}
		if r.RankDisplay != "" || r.RankOrder != 99 {
			t.Errorf("arid %s: RankDisplay/RankOrder = %q/%d, want \"\"/99", r.RequestID, r.RankDisplay, r.RankOrder)
		}
		if r.Address != "" {
			t.Errorf("arid %s: Address = %q, want empty (left for the commute resolver)", r.RequestID, r.Address)
		}
		if r.URL == "" {
			t.Errorf("arid %s: URL is empty", r.RequestID)
		}
	}
}

func TestParseCatalog_skipsIncompleteRow(t *testing.T) {
	html, err := os.ReadFile(testdataPath(t, "kollegiumlist.html"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	rows, err := parseCatalog(string(html))
	if err != nil {
		t.Fatalf("parseCatalog: %v", err)
	}
	for _, r := range rows {
		if r.RequestID == "1332709" {
			t.Errorf("arid 1332709 has no price cell; should be skipped, got %+v", r)
		}
	}
}

func TestParseCatalog_skipsDormWithNoRoomTypeDetails(t *testing.T) {
	rows, err := parseCatalog(`<div class="kollegium row"><div class="head"><h3><a>Empty Dorm</a></h3></div></div>`)
	if err != nil {
		t.Fatalf("parseCatalog: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("got %d rows for a dorm with no room types, want 0: %+v", len(rows), rows)
	}
}
