package export

import (
	"fmt"

	"denmark-housing-waitlist/internal/parser"
)

// CSVHeader is the column header row written to CSV and Google Sheets.
var CSVHeader = []string{"request_id", "dorm", "room_type", "size_sqm", "your_rank"}

// Records returns CSV header plus sorted data rows as strings.
func Records(rows []parser.WaitlistRow) [][]string {
	sorted := SortRows(rows)
	out := make([][]string, 0, len(sorted)+1)
	out = append(out, CSVHeader)
	for _, row := range sorted {
		out = append(out, []string{
			row.RequestID,
			row.Dorm,
			row.RoomType,
			row.Size,
			fmt.Sprintf("%d", row.YourRank),
		})
	}
	return out
}
