package kkik

import (
	"os"
	"testing"
)

func TestParseRentTable(t *testing.T) {
	html, err := os.ReadFile(testdataPath(t, "kollegiumlist.html"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	got := parseRentTable(string(html))

	want := map[string]int{"1788": 3701, "1789": 3701, "1790": 5957, "1791": 7076}
	for arid, rent := range want {
		if got[arid] != rent {
			t.Errorf("arid %s rent = %d, want %d", arid, got[arid], rent)
		}
	}
	// arid 1332709 has a roomtypedetails link but no price cell -> skipped.
	if r, ok := got["1332709"]; ok {
		t.Errorf("arid 1332709 has no price cell; should be absent, got %d", r)
	}
	if len(got) != len(want) {
		t.Errorf("got %d entries %v, want %d", len(got), got, len(want))
	}
}

func TestAridFromURL(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://www.kollegierneskontor.dk/default.aspx?func=kkikportal.roomtypedetails&arid=1788&kid=1783&lang=GB", "1788"},
		{"default.aspx?func=kkikportal.roomtypedetails&arid=1332709&lang=GB", "1332709"},
		{"https://x/?func=kkikportal.roomtypelist&kid=1330699&lang=GB", ""}, // roomtypelist has kid, no arid
		{"", ""},
	}
	for _, c := range cases {
		if got := aridFromURL(c.in); got != c.want {
			t.Errorf("aridFromURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseKr(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"kr. 3,701.00", 3701, true},
		{"kr. 7,076.00", 7076, true},
		{"kr. 738.00", 738, true},
		{"kr.", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		got, ok := parseKr(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("parseKr(%q) = (%d, %v), want (%d, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}
