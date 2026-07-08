package runner

import (
	"reflect"
	"testing"

	"housing-waitlist/internal/config"
	"housing-waitlist/internal/scoring"
)

func TestScoringSettings_projectsConfig(t *testing.T) {
	cfg := &config.Config{Scoring: config.Scoring{
		Enabled:         true,
		MaxRent:         6000,
		RentFloor:       1000,
		CommuteBestMin:  15,
		CommuteWorstMin: 50,
		RankWeight:      0.3,
		SortBy:          "opportunity",
		Weights:         config.ScoringWeights{Commute: 0.5, Size: 0.2, Rent: 0.3},
	}}

	got := scoringSettings(cfg, []string{"cbs", "rda"})

	want := scoring.Settings{
		Enabled:         true,
		MaxRent:         6000,
		RentFloor:       1000,
		CommuteBestMin:  15,
		CommuteWorstMin: 50,
		Weights:         scoring.Weights{Commute: 0.5, Size: 0.2, Rent: 0.3},
		RankWeight:      0.3,
		SortBy:          "opportunity",
		DestNames:       []string{"cbs", "rda"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("scoringSettings mismatch:\n got %+v\nwant %+v", got, want)
	}
}
