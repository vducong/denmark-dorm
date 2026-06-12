package sdk

import (
	"os"
	"testing"

	"housing-waitlist/internal/model"
)

func TestParseHTML(t *testing.T) {
	html, err := os.ReadFile("testdata/list.html")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	res, err := parseHTML(string(html))
	if err != nil {
		t.Fatalf("parseHTML: %v", err)
	}

	// Three signed-up tenancies across two buildings; the not-signed-up
	// ("Sign up for this tenancy") row carries no rank label and is skipped.
	want := []model.WaitlistRow{
		{RequestID: "3735", Dorm: "Nørrebrogade", RoomType: "Nørrebrogade 9E 2 206, 2200 København N", Size: "33", RankDisplay: "G", RankOrder: 7},
		{RequestID: "3736", Dorm: "Nørrebrogade", RoomType: "Nørrebrogade 9D 3 302, 2200 København N", Size: "31", RankDisplay: "F", RankOrder: 6},
		{RequestID: "23043", Dorm: "Enghavevej", RoomType: "Enghavevej 70 3 tv., 1503 København V", Size: "58", RankDisplay: "Not set", RankOrder: 99},
	}

	if len(res.Rows) != len(want) {
		t.Fatalf("got %d rows, want %d: %+v", len(res.Rows), len(want), res.Rows)
	}
	for i, w := range want {
		if res.Rows[i] != w {
			t.Errorf("row %d:\n got %+v\nwant %+v", i, res.Rows[i], w)
		}
	}
}

func TestRankOrder(t *testing.T) {
	cases := []struct {
		display   string
		wantOrder int
		wantOK    bool
	}{
		{"A", 1, true},
		{"G", 7, true},
		{"Z", 26, true},
		{"Not set", 99, true},
		{" C ", 3, true}, // trimmed
		{"", 0, false},
		{"AA", 0, false},
		{"3", 0, false},
	}
	for _, c := range cases {
		order, ok := rankOrder(c.display)
		if order != c.wantOrder || ok != c.wantOK {
			t.Errorf("rankOrder(%q) = (%d, %t), want (%d, %t)", c.display, order, ok, c.wantOrder, c.wantOK)
		}
	}
}

func TestParseHTMLEmpty(t *testing.T) {
	if _, err := parseHTML("<html><body></body></html>"); err == nil {
		t.Fatal("expected error for HTML with no tenancies")
	}
}
