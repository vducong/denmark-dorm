package export

import (
	"strconv"

	"housing-waitlist/internal/model"
)

// CSVHeader is the column header row written to CSV and Google Sheets.
var CSVHeader = []string{"request_id", "dorm", "room_type", "size_sqm", "your_rank", "diff"}

// Records returns CSV header plus sorted data rows as strings.
// prevOrders maps request_id to RankOrder from the latest previous export;
// diff is prevOrder − newOrder (positive = improved toward a better rank).
func Records(rows []model.WaitlistRow, prevOrders map[string]int) [][]string {
	sorted := SortRows(rows)
	out := make([][]string, 0, len(sorted)+1)
	out = append(out, CSVHeader)
	for _, row := range sorted {
		out = append(out, []string{
			row.RequestID,
			row.Dorm,
			row.RoomType,
			row.Size,
			row.RankDisplay,
			diffCell(row.RequestID, row.RankOrder, prevOrders),
		})
	}
	return out
}

func diffCell(requestID string, order int, prevOrders map[string]int) string {
	if prevOrders == nil {
		return ""
	}
	prev, ok := prevOrders[requestID]
	if !ok {
		return ""
	}

	if prev > order {
		return "+" + strconv.Itoa(prev-order)
	} else if prev < order {
		return "-" + strconv.Itoa(order-prev)
	}
	return "-"
}
