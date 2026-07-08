package config

import (
	"path/filepath"
	"testing"
)

func TestApplyDefaults_scoring(t *testing.T) {
	var c Config
	c.applyDefaults()

	s := c.Scoring
	if want := filepath.Join("data", "candidates.csv"); s.OutputPath != want {
		t.Errorf("OutputPath = %q, want %q", s.OutputPath, want)
	}
	if s.CommuteBestMin != 20 {
		t.Errorf("CommuteBestMin = %d, want 20", s.CommuteBestMin)
	}
	if s.CommuteWorstMin != 60 {
		t.Errorf("CommuteWorstMin = %d, want 60", s.CommuteWorstMin)
	}
	if s.SortBy != "desirability" {
		t.Errorf("SortBy = %q, want desirability", s.SortBy)
	}
	if s.RankWeight != 0.5 {
		t.Errorf("RankWeight = %v, want 0.5", s.RankWeight)
	}
	if s.Weights.Commute != 0.4 || s.Weights.Size != 0.3 || s.Weights.Rent != 0.3 {
		t.Errorf("default Weights = %+v, want {0.4 0.3 0.3}", s.Weights)
	}
}

func TestApplyDefaults_scoringRespectsExplicitWeights(t *testing.T) {
	var c Config
	c.Scoring.Weights = ScoringWeights{Commute: 0.5} // partially set => not all-zero
	c.applyDefaults()
	if got := c.Scoring.Weights; got.Commute != 0.5 || got.Size != 0 || got.Rent != 0 {
		t.Errorf("explicit weights overwritten: %+v", got)
	}
}
