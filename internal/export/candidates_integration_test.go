package export

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"housing-waitlist/internal/commute"
	"housing-waitlist/internal/model"
	"housing-waitlist/internal/scoring"
)

// TestCandidates_endToEnd exercises the real scoring.Rank -> WriteCandidates
// path on rows shaped like the KKIK and SDK parsers produce (numeric vs letter
// ranks, "Not set", Danish-comma sizes), with a budget gate, and logs the CSV.
func TestCandidates_endToEnd(t *testing.T) {
	dest := "cbs"
	col := commute.TransitMorningCol(dest)
	row := func(dorm, size, rankDisplay string, rankOrder, min, max, mins int) model.WaitlistRow {
		return model.WaitlistRow{
			Dorm: dorm, RoomType: dorm + " room", Size: size,
			RankDisplay: rankDisplay, RankOrder: rankOrder,
			RentMin: min, RentMax: max,
			URL:     "http://x/" + dorm,
			Commute: map[string]string{col: strconv.Itoa(mins)},
		}
	}

	in := []scoring.Input{
		{Source: "kkik", Row: row("Aksehuset", "25,2 m²", "3", 3, 3500, 4000, 22)},
		{Source: "kkik", Row: row("Bispebjerg", "43-45 m²", "120", 120, 4500, 5000, 45)},
		{Source: "kkik", Row: row("Pricey", "30 m²", "5", 5, 8000, 9000, 25)}, // gated: 8000 > 6000
		{Source: "sdk", Row: row("Nørrebrogade", "33", "A", 1, 5000, 7000, 30)},
		{Source: "sdk", Row: row("Enghavevej", "58", "Not set", 99, 0, 0, 18)}, // rent unknown
	}

	s := scoring.Settings{
		Enabled: true, MaxRent: 6000, RentFloor: 0,
		CommuteBestMin: 20, CommuteWorstMin: 60,
		Weights:    scoring.Weights{Commute: 0.4, Size: 0.3, Rent: 0.3},
		RankWeight: 0.5, SortBy: "desirability", DestNames: []string{dest},
	}

	ranked := scoring.Rank(in, s)

	if len(ranked) != 4 {
		t.Fatalf("want 4 rows after budget gate, got %d", len(ranked))
	}
	for _, r := range ranked {
		if r.Row.Dorm == "Pricey" {
			t.Errorf("Pricey (rent 8000 > budget 6000) should be gated out")
		}
	}
	// desirability sort: each row's score is >= the next.
	for i := 1; i < len(ranked); i++ {
		if ranked[i-1].Desirability < ranked[i].Desirability {
			t.Errorf("rows not sorted by desirability: %v < %v at %d",
				ranked[i-1].Desirability, ranked[i].Desirability, i)
		}
	}

	path := filepath.Join(t.TempDir(), "candidates.csv")
	if err := WriteCandidates(path, []string{col}, ranked); err != nil {
		t.Fatalf("WriteCandidates: %v", err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	t.Logf("candidates.csv:\n%s", strings.TrimRight(string(out), "\n"))
}
