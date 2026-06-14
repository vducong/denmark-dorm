package export

import (
	"strconv"

	"housing-waitlist/internal/model"
)

// csvLeadCols are the fixed columns before any commute columns; csvTailCols are
// the fixed columns after. Commute columns (when enabled) are inserted between
// them — right after size_sqm — so they sit beside the room attributes rather
// than after the rank/diff. The snapshot and prev-rank loaders read by column
// name, so this ordering is free to change without breaking them.
var (
	csvLeadCols = []string{"request_id", "dorm", "url", "room_type", "size_sqm"}
	csvTailCols = []string{"your_rank", "diff"}
)

// Records returns the CSV header plus sorted data rows as strings.
// prevOrders maps request_id to RankOrder from the latest previous export;
// diff is prevOrder − newOrder (positive = improved toward a better rank).
// commuteCols lists the commute column names to insert after size_sqm (nil when
// disabled); each row's value comes from its Commute map, blank when unknown.
func Records(rows []model.WaitlistRow, prevOrders map[string]int, commuteCols []string) [][]string {
	sorted := SortRows(rows)
	header := make([]string, 0, len(csvLeadCols)+len(commuteCols)+len(csvTailCols))
	header = append(header, csvLeadCols...)
	header = append(header, commuteCols...)
	header = append(header, csvTailCols...)

	out := make([][]string, 0, len(sorted)+1)
	out = append(out, header)
	for _, row := range sorted {
		rec := make([]string, 0, len(header))
		rec = append(rec, row.RequestID, row.Dorm, row.URL, row.RoomType, row.Size)
		for _, col := range commuteCols {
			rec = append(rec, row.Commute[col])
		}
		rec = append(rec, row.RankDisplay, diffCell(row.RequestID, row.RankOrder, prevOrders))
		out = append(out, rec)
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
