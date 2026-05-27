package export

import (
	"encoding/csv"
	"fmt"
	"os"
	"sort"

	"denmark-housing-waitlist/internal/parser"
)

// SortRows returns rows sorted by your_rank, then dorm, then room_type.
func SortRows(rows []parser.WaitlistRow) []parser.WaitlistRow {
	out := make([]parser.WaitlistRow, len(rows))
	copy(out, rows)
	sort.Slice(out, func(i, j int) bool {
		if out[i].YourRank != out[j].YourRank {
			return out[i].YourRank < out[j].YourRank
		}
		if out[i].Dorm != out[j].Dorm {
			return out[i].Dorm < out[j].Dorm
		}
		return out[i].RoomType < out[j].RoomType
	})
	return out
}

// WriteCSV writes sorted rows to path with a stable header.
func WriteCSV(path string, rows []parser.WaitlistRow) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create csv: %w", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	for _, rec := range Records(rows) {
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
