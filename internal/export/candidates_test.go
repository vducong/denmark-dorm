package export

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"housing-waitlist/internal/model"
	"housing-waitlist/internal/scoring"
)

func TestWriteCandidates(t *testing.T) {
	commuteCols := []string{"cbs_transit_morning_min", "cbs_walk_min"}
	rows := []scoring.ScoredRow{
		{
			Input: scoring.Input{Source: "kkik", Row: model.WaitlistRow{
				Dorm: "Aksehuset", RoomType: "Single", Size: "25,2 m²", RankDisplay: "10",
				URL: "http://x/1", RentMin: 4000, RentMax: 6000,
				Commute: map[string]string{"cbs_transit_morning_min": "25", "cbs_walk_min": "60"},
			}},
			Desirability: 72.5, Opportunity: 81.0,
		},
		{
			Input: scoring.Input{Source: "sdk", Row: model.WaitlistRow{
				Dorm: "Nørrebro", RoomType: "Addr", Size: "33", RankDisplay: "Not set",
				URL: "http://y/2", // rent unknown, no commute
			}},
			Desirability: 50.0, Opportunity: 25.0,
		},
	}

	path := filepath.Join(t.TempDir(), "nested", "candidates.csv")
	if err := WriteCandidates(path, commuteCols, rows); err != nil {
		t.Fatalf("WriteCandidates: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	got, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	want := [][]string{
		{"source", "dorm", "room_type", "size_sqm", "rent", "cbs_transit_morning_min", "cbs_walk_min", "your_rank", "desirability", "opportunity", "url"},
		{"kkik", "Aksehuset", "Single", "25,2 m²", "4000-6000", "25", "60", "10", "72.5", "81.0", "http://x/1"},
		{"sdk", "Nørrebro", "Addr", "33", "", "", "", "Not set", "50.0", "25.0", "http://y/2"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("candidates CSV mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestRentCell(t *testing.T) {
	tests := []struct {
		min, max int
		want     string
	}{
		{4000, 6000, "4000-6000"},
		{4000, 4000, "4000"},
		{0, 0, ""},
		{0, 5000, "5000"},
		{5000, 0, "5000"},
	}
	for _, tt := range tests {
		if got := rentCell(tt.min, tt.max); got != tt.want {
			t.Errorf("rentCell(%d,%d) = %q, want %q", tt.min, tt.max, got, tt.want)
		}
	}
}
