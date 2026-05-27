package parser_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"denmark-housing-waitlist/internal/parser"
)

func testdataPath(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", name)
}

func TestParse_ListHTML(t *testing.T) {
	html, err := os.ReadFile(testdataPath(t, "list.html"))
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}

	result, err := parser.ParseHTML(string(html))
	if err != nil {
		t.Fatalf("ParseHTML: %v", err)
	}

	if got := len(result.Rows); got != 42 {
		t.Fatalf("row count = %d, want 42", got)
	}

	first := result.Rows[0]
	if first.YourRank != 149 {
		t.Errorf("first YourRank = %d, want 149", first.YourRank)
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
