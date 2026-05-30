package export

import (
	"fmt"
	"strconv"

	"denmark-housing-waitlist/internal/parser"
)

// CSVHeader is the column header row written to CSV and Google Sheets.
var CSVHeader = []string{"request_id", "dorm", "room_type", "size_sqm", "your_rank", "diff"}

// Records returns CSV header plus sorted data rows as strings.
// prevRanks maps request_id to your_rank from the latest previous export; diff is prev − new (positive = improved).
func Records(rows []parser.WaitlistRow, prevRanks map[string]int) [][]string {
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
			diffCell(row.RequestID, row.YourRank, prevRanks),
		})
	}
	return out
}

func diffCell(requestID string, rank int, prevRanks map[string]int) string {
	if prevRanks == nil {
		return ""
	}
	prev, ok := prevRanks[requestID]
	if !ok {
		return ""
	}

	if prev > rank {
		return "+" + strconv.Itoa(prev-rank)
	} else if prev < rank {
		return "-" + strconv.Itoa(rank-prev)
	}
	return "-"
}
