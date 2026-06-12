package export

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDateHeader(t *testing.T) {
	d := time.Date(2026, 5, 26, 13, 38, 0, 0, time.UTC)
	if got := DateHeader(d); got != "260526" {
		t.Errorf("DateHeader() = %q, want 260526", got)
	}
}

func TestParseDateHeader(t *testing.T) {
	got, ok := ParseDateHeader("260526")
	if !ok {
		t.Fatal("ParseDateHeader() ok = false")
	}
	want := time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("ParseDateHeader() = %v, want %v", got, want)
	}
	if _, ok := ParseDateHeader("bad"); ok {
		t.Error("ParseDateHeader(bad) ok = true, want false")
	}
}

func TestLoadDailySnapshots_dedupSameDay(t *testing.T) {
	dir := t.TempDir()
	writeCSV(t, dir, "202605261338_waitlist.csv", "request_id,dorm,room_type,size_sqm,your_rank\na,D,R,S,10\n")
	writeCSV(t, dir, "202605261400_waitlist.csv", "request_id,dorm,room_type,size_sqm,your_rank\na,D,R,S,5\n")
	writeCSV(t, dir, "202605281102_waitlist.csv", "request_id,dorm,room_type,size_sqm,your_rank\na,D,R,S,3\n")

	snaps, err := LoadDailySnapshots(dir)
	if err != nil {
		t.Fatalf("LoadDailySnapshots() err = %v", err)
	}
	if len(snaps) != 2 {
		t.Fatalf("len(snapshots) = %d, want 2", len(snaps))
	}
	if snaps[0].DateHeader != "260526" || snaps[0].Ranks["a"] != "5" {
		t.Errorf("first snapshot = %+v", snaps[0])
	}
	if snaps[1].DateHeader != "280526" || snaps[1].Ranks["a"] != "3" {
		t.Errorf("second snapshot = %+v", snaps[1])
	}
}

func TestLoadDailySnapshots_metadata(t *testing.T) {
	dir := t.TempDir()
	writeCSV(t, dir, "202605261338_waitlist.csv",
		"request_id,dorm,room_type,size_sqm,your_rank\n10539078,Husumvej 106,Room,11 m2,25\n")

	snaps, err := LoadDailySnapshots(dir)
	if err != nil {
		t.Fatalf("LoadDailySnapshots() err = %v", err)
	}
	row := snaps[0].Rows["10539078"]
	if row.Dorm != "Husumvej 106" || row.RankDisplay != "25" {
		t.Errorf("row = %+v", row)
	}
}

func TestLoadDailySnapshots_url(t *testing.T) {
	dir := t.TempDir()
	writeCSV(t, dir, "202605261338_waitlist.csv",
		"request_id,dorm,url,room_type,size_sqm,your_rank\n10539078,Husumvej 106,https://example.test/x,Room,11 m2,25\n")

	snaps, err := LoadDailySnapshots(dir)
	if err != nil {
		t.Fatalf("LoadDailySnapshots() err = %v", err)
	}
	if row := snaps[0].Rows["10539078"]; row.URL != "https://example.test/x" {
		t.Errorf("row URL = %q, want https://example.test/x", row.URL)
	}
}

func writeCSV(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
