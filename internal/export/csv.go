package export

import (
	"encoding/csv"
	"fmt"
	"os"
	"sort"

	"github.com/koan/kkik-waitlist/internal/parser"
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
	sorted := SortRows(rows)

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create csv: %w", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write([]string{"request_id", "dorm", "room_type", "size_sqm", "your_rank"}); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	for _, row := range sorted {
		if err := w.Write([]string{
			row.RequestID,
			row.Dorm,
			row.RoomType,
			row.Size,
			fmt.Sprintf("%d", row.YourRank),
		}); err != nil {
			return fmt.Errorf("write row: %w", err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return fmt.Errorf("flush csv: %w", err)
	}
	return nil
}
