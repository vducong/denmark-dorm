package kkik

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func testdataPath(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "testdata", name)
}

func TestParse_ListHTML(t *testing.T) {
	html, err := os.ReadFile(testdataPath(t, "list.html"))
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}

	result, err := parseHTML(string(html))
	if err != nil {
		t.Fatalf("parseHTML: %v", err)
	}

	if got := len(result.Rows); got != 42 {
		t.Fatalf("row count = %d, want 42", got)
	}

	first := result.Rows[0]
	if first.RankOrder != 149 {
		t.Errorf("first RankOrder = %d, want 149", first.RankOrder)
	}
	if first.RankDisplay != "149" {
		t.Errorf("first RankDisplay = %q, want 149", first.RankDisplay)
	}
	if first.RequestID != "10539074" {
		t.Errorf("first RequestID = %q, want 10539074", first.RequestID)
	}
	if first.Dorm == "" {
		t.Error("first Dorm is empty")
	}
	if first.RoomType == "" {
		t.Error("first RoomType is empty")
	}
}

func TestRankOrder(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"3", 3, true},
		{"3.", 3, true},
		{"1.234", 1234, true},
		{"", 0, false},
		{"abc", 0, false},
	}
	for _, c := range cases {
		got, ok := rankOrder(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("rankOrder(%q) = (%d,%v), want (%d,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}
