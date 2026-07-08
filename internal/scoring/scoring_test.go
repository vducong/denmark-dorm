package scoring

import (
	"testing"

	"housing-waitlist/internal/commute"
	"housing-waitlist/internal/model"
)

func baseSettings() Settings {
	return Settings{
		Enabled:         true,
		CommuteBestMin:  20,
		CommuteWorstMin: 60,
		Weights:         Weights{Commute: 0.4, Size: 0.3, Rent: 0.3},
		RankWeight:      0.5,
		SortBy:          "desirability",
		DestNames:       []string{"cbs"},
	}
}

func withCommute(r model.WaitlistRow, dest, mins string) model.WaitlistRow {
	if r.Commute == nil {
		r.Commute = map[string]string{}
	}
	r.Commute[commute.TransitMorningCol(dest)] = mins
	return r
}

func find(rows []ScoredRow, reqID string) (ScoredRow, bool) {
	for _, r := range rows {
		if r.Row.RequestID == reqID {
			return r, true
		}
	}
	return ScoredRow{}, false
}

func TestRank_budgetGateExcludesUnaffordable(t *testing.T) {
	s := baseSettings()
	s.MaxRent = 6000
	in := []Input{
		{Source: "kkik", Row: model.WaitlistRow{RequestID: "A", RankDisplay: "1", RankOrder: 1, Size: "30", RentMin: 4000, RentMax: 4000}},
		{Source: "kkik", Row: model.WaitlistRow{RequestID: "B", RankDisplay: "2", RankOrder: 2, Size: "30", RentMin: 7000, RentMax: 8000}},
	}
	got := Rank(in, s)
	if len(got) != 1 || got[0].Row.RequestID != "A" {
		t.Fatalf("budget gate: got %d rows %v, want only A", len(got), ids(got))
	}
}

func TestRank_cheaperRentScoresHigher(t *testing.T) {
	s := baseSettings()
	s.MaxRent = 8000 // rent band 0..8000
	s.Weights = Weights{Rent: 1}
	in := []Input{
		{Source: "kkik", Row: model.WaitlistRow{RequestID: "A", RankDisplay: "1", RankOrder: 1, RentMin: 2000, RentMax: 2000}},
		{Source: "kkik", Row: model.WaitlistRow{RequestID: "B", RankDisplay: "2", RankOrder: 2, RentMin: 6000, RentMax: 6000}},
	}
	got := Rank(in, s)
	a, _ := find(got, "A")
	b, _ := find(got, "B")
	if !almost(a.Desirability, 75) {
		t.Errorf("A desirability = %v, want 75", a.Desirability)
	}
	if !almost(b.Desirability, 25) {
		t.Errorf("B desirability = %v, want 25", b.Desirability)
	}
	if got[0].Row.RequestID != "A" {
		t.Errorf("sort: got %v, want A first", ids(got))
	}
}

func TestRank_missingRentRenormalizesWeights(t *testing.T) {
	s := baseSettings() // weights 0.4/0.3/0.3, no rent present -> renormalize over 0.7
	a := withCommute(model.WaitlistRow{RequestID: "A", RankDisplay: "1", RankOrder: 1, Size: "40"}, "cbs", "20")
	b := withCommute(model.WaitlistRow{RequestID: "B", RankDisplay: "2", RankOrder: 2, Size: "20"}, "cbs", "20")
	got := Rank([]Input{{Source: "kkik", Row: a}, {Source: "kkik", Row: b}}, s)
	ra, _ := find(got, "A")
	rb, _ := find(got, "B")
	// commute norm = 1.0 for both (20 <= best). size relative: A=1.0, B=0.0.
	// A: (0.4*1 + 0.3*1)/0.7 = 1.0 -> 100. B: (0.4*1 + 0.3*0)/0.7 -> 57.142857.
	if !almost(ra.Desirability, 100) {
		t.Errorf("A desirability = %v, want 100", ra.Desirability)
	}
	if !almost(rb.Desirability, 4.0/7.0*100) {
		t.Errorf("B desirability = %v, want %v", rb.Desirability, 4.0/7.0*100)
	}
}

func TestRank_rankNormalizedPerSource(t *testing.T) {
	s := baseSettings()
	s.RankWeight = 1 // opportunity = 100 * norm_rank, isolates rank
	in := []Input{
		{Source: "kkik", Row: model.WaitlistRow{RequestID: "K1", RankDisplay: "10", RankOrder: 10}},
		{Source: "kkik", Row: model.WaitlistRow{RequestID: "K2", RankDisplay: "50", RankOrder: 50}},
		{Source: "sdk", Row: model.WaitlistRow{RequestID: "S1", RankDisplay: "A", RankOrder: 1}},
		{Source: "sdk", Row: model.WaitlistRow{RequestID: "S2", RankDisplay: "G", RankOrder: 7}},
		{Source: "sdk", Row: model.WaitlistRow{RequestID: "S3", RankDisplay: "Not set", RankOrder: 99}},
	}
	got := Rank(in, s)
	want := map[string]float64{"K1": 100, "K2": 0, "S1": 100, "S2": 0, "S3": 0}
	for id, w := range want {
		r, ok := find(got, id)
		if !ok {
			t.Fatalf("missing row %s", id)
		}
		if !almost(r.Opportunity, w) {
			t.Errorf("%s opportunity = %v, want %v", id, r.Opportunity, w)
		}
	}
}

func TestRank_commuteFixedBand(t *testing.T) {
	s := baseSettings()
	s.Weights = Weights{Commute: 1}
	mk := func(id, mins string) Input {
		return Input{Source: "kkik", Row: withCommute(model.WaitlistRow{RequestID: id, RankDisplay: "1", RankOrder: 1}, "cbs", mins)}
	}
	got := Rank([]Input{mk("A", "20"), mk("B", "60"), mk("C", "40"), mk("D", "10")}, s)
	want := map[string]float64{"A": 100, "B": 0, "C": 50, "D": 100}
	for id, w := range want {
		r, _ := find(got, id)
		if !almost(r.Desirability, w) {
			t.Errorf("%s desirability = %v, want %v", id, r.Desirability, w)
		}
	}
}

func TestRank_commuteAveragedAcrossDestinations(t *testing.T) {
	s := baseSettings()
	s.Weights = Weights{Commute: 1}
	s.DestNames = []string{"cbs", "ku"}
	r := withCommute(model.WaitlistRow{RequestID: "A", RankDisplay: "1", RankOrder: 1}, "cbs", "20")
	r = withCommute(r, "ku", "40")
	got := Rank([]Input{{Source: "kkik", Row: r}}, s)
	a, _ := find(got, "A")
	// avg commute 30 -> norm (60-30)/40 = 0.75 -> 75.
	if !almost(a.Desirability, 75) {
		t.Errorf("A desirability = %v, want 75", a.Desirability)
	}
}

func TestRank_sortByOpportunity(t *testing.T) {
	s := baseSettings()
	s.SortBy = "opportunity"
	s.Weights = Weights{Size: 1}
	s.RankWeight = 0.7
	// A wins desirability (best size) but worst rank; B wins rank. With
	// rank_weight 0.7 and sort_by=opportunity, B's rank edge wins -> B leads,
	// even though by desirability A would lead.
	in := []Input{
		{Source: "kkik", Row: model.WaitlistRow{RequestID: "A", RankDisplay: "100", RankOrder: 100, Size: "45"}},
		{Source: "kkik", Row: model.WaitlistRow{RequestID: "B", RankDisplay: "1", RankOrder: 1, Size: "44"}},
	}
	got := Rank(in, s)
	if got[0].Row.RequestID != "B" {
		t.Errorf("sort_by opportunity: got %v, want B first", ids(got))
	}
}

func TestRank_unrankedOpportunityEqualsDesirability(t *testing.T) {
	s := baseSettings()
	s.MaxRent = 6000
	// Catalog row: no rank (empty RankDisplay, sentinel RankOrder=99).
	// Opportunity must equal Desirability regardless of rank_weight.
	in := []Input{
		{Source: "sdk", Row: model.WaitlistRow{RequestID: "C1", RankDisplay: "", RankOrder: 99, Size: "37", RentMin: 3000, RentMax: 3000}},
	}
	got := Rank(in, s)
	if len(got) != 1 {
		t.Fatalf("expected 1 row, got %d", len(got))
	}
	r := got[0]
	if !almost(r.Opportunity, r.Desirability) {
		t.Errorf("unranked row: opportunity = %v, desirability = %v, want equal", r.Opportunity, r.Desirability)
	}
}

func ids(rows []ScoredRow) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Row.RequestID
	}
	return out
}
