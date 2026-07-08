package export

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"housing-waitlist/internal/scoring"
)

// candidateLeadCols precede the commute columns; candidateTailCols follow them.
// Commute columns are inserted between, mirroring the per-source CSV layout.
var (
	candidateLeadCols = []string{"source", "dorm", "room_type", "size_sqm", "rent"}
	candidateTailCols = []string{"your_rank", "desirability", "opportunity", "url"}
)

// WriteCandidates writes the merged, already-sorted scored rows to path as a
// CSV, creating the parent directory if needed. commuteCols names the commute
// columns to insert after rent (nil to omit); each value comes from the row's
// Commute map, blank when unknown. Rows are written in the order given — sorting
// is the scorer's job.
func WriteCandidates(path string, commuteCols []string, rows []scoring.ScoredRow) error {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create candidates dir: %w", err)
		}
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create candidates csv: %w", err)
	}
	defer f.Close()

	header := make([]string, 0, len(candidateLeadCols)+len(commuteCols)+len(candidateTailCols))
	header = append(header, candidateLeadCols...)
	header = append(header, commuteCols...)
	header = append(header, candidateTailCols...)

	w := csv.NewWriter(f)
	if err := w.Write(header); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	for _, sr := range rows {
		rec := make([]string, 0, len(header))
		rec = append(rec, sr.Source, sr.Row.Dorm, sr.Row.RoomType, sr.Row.Size, rentCell(sr.Row.RentMin, sr.Row.RentMax))
		for _, col := range commuteCols {
			rec = append(rec, sr.Row.Commute[col])
		}
		rec = append(rec, sr.Row.RankDisplay, score(sr.Desirability), score(sr.Opportunity), sr.Row.URL)
		if err := w.Write(rec); err != nil {
			return fmt.Errorf("write row: %w", err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return fmt.Errorf("flush candidates csv: %w", err)
	}
	return nil
}

// rentCell renders the rent bounds: a single value when equal, a "min-max"
// range otherwise, whichever bound is known when only one is, and blank when
// rent is unknown.
func rentCell(min, max int) string {
	switch {
	case min > 0 && max > 0 && min != max:
		return strconv.Itoa(min) + "-" + strconv.Itoa(max)
	case min > 0:
		return strconv.Itoa(min)
	case max > 0:
		return strconv.Itoa(max)
	default:
		return ""
	}
}

func score(x float64) string { return strconv.FormatFloat(x, 'f', 1, 64) }
