package export

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"housing-waitlist/internal/model"
)

// SortRows returns rows sorted by rank order, then dorm, then room_type.
func SortRows(rows []model.WaitlistRow) []model.WaitlistRow {
	out := make([]model.WaitlistRow, len(rows))
	copy(out, rows)
	sort.Slice(out, func(i, j int) bool {
		if out[i].RankOrder != out[j].RankOrder {
			return out[i].RankOrder < out[j].RankOrder
		}
		if out[i].Dorm != out[j].Dorm {
			return out[i].Dorm < out[j].Dorm
		}
		return out[i].RoomType < out[j].RoomType
	})
	return out
}

// WriteCSV writes sorted rows to path with a stable header, creating the parent
// directory if needed.
func WriteCSV(path string, rows []model.WaitlistRow, prevOrders map[string]int) error {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create csv dir: %w", err)
		}
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create csv: %w", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	for _, rec := range Records(rows, prevOrders) {
		if err := w.Write(rec); err != nil {
			return fmt.Errorf("write row: %w", err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return fmt.Errorf("flush csv: %w", err)
	}
	return nil
}
