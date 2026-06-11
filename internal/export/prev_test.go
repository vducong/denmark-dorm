package export

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// numericOrder is a stand-in for a source's RankOrder over numeric ranks.
func numericOrder(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	return n, err == nil
}

func TestLoadPrevRanks_latestFile(t *testing.T) {
	dir := t.TempDir()
	older := filepath.Join(dir, "202605261338_waitlist.csv")
	newer := filepath.Join(dir, "202605300134_waitlist.csv")
	if err := os.WriteFile(older, []byte("request_id,dorm,room_type,size_sqm,your_rank\n1,A,,,100\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newer, []byte("request_id,dorm,room_type,size_sqm,your_rank\n2,B,,,50\n3,C,,,75\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ranks, err := LoadPrevRanks(dir, numericOrder)
	if err != nil {
		t.Fatal(err)
	}
	if len(ranks) != 2 {
		t.Fatalf("len = %d, want 2", len(ranks))
	}
	if ranks["2"] != 50 || ranks["3"] != 75 {
		t.Errorf("ranks = %v", ranks)
	}
	if _, ok := ranks["1"]; ok {
		t.Errorf("should not load from older file: %v", ranks)
	}
}

func TestLoadPrevRanks_noFiles(t *testing.T) {
	ranks, err := LoadPrevRanks(t.TempDir(), numericOrder)
	if err != nil {
		t.Fatal(err)
	}
	if len(ranks) != 0 {
		t.Errorf("ranks = %v, want empty", ranks)
	}
}
