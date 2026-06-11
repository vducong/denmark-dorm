package sheets

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"housing-waitlist/internal/export"
	"housing-waitlist/internal/model"
)

var baseHeader = []string{"request_id", "dorm", "room_type", "size_sqm"}

const (
	latestDiffHeader = "latest_diff"
	metaLabel        = "Last updated at"
)

type rowInfo struct {
	requestID string
	dorm      string
	roomType  string
	size      string
	todayRank int
	hasToday  bool
}

// BuildMatrix merges scrape data, CSV snapshots, and existing sheet values into
// sheet rows. rankOrder projects display ranks onto a sortable scale for the
// latest_diff column, so numeric and letter ranks both diff correctly.
func BuildMatrix(
	rows []model.WaitlistRow,
	snapshots []export.DailySnapshot,
	existing [][]any,
	today time.Time,
	rankOrder func(string) (int, bool),
) ([][]any, error) {
	todayHeader := export.DateHeader(today)
	scrapeByID := make(map[string]model.WaitlistRow, len(rows))
	for _, row := range rows {
		scrapeByID[row.RequestID] = row
	}

	snapshotByHeader := make(map[string]export.DailySnapshot, len(snapshots))
	for _, snap := range snapshots {
		snapshotByHeader[snap.DateHeader] = snap
	}

	existingSheet, legacy := parseExistingSheet(existing)

	dateHeaders := mergeDateHeaders(existingSheet.dateHeaders, snapshots, todayHeader, legacy)
	if len(dateHeaders) == 0 {
		dateHeaders = []string{todayHeader}
	}

	ids := collectRequestIDs(scrapeByID, snapshots, existingSheet.rows)
	if len(ids) == 0 {
		return nil, fmt.Errorf("no rows to write")
	}

	infos := make([]rowInfo, 0, len(ids))
	for _, id := range ids {
		info := buildRowInfo(id, scrapeByID, snapshots, existingSheet.rows)
		infos = append(infos, info)
	}
	sort.Slice(infos, func(i, j int) bool {
		if infos[i].hasToday != infos[j].hasToday {
			return infos[i].hasToday
		}
		if !infos[i].hasToday {
			return infos[i].requestID < infos[j].requestID
		}
		if infos[i].todayRank != infos[j].todayRank {
			return infos[i].todayRank < infos[j].todayRank
		}
		return infos[i].requestID < infos[j].requestID
	})

	header := make([]any, 0, len(baseHeader)+1+len(dateHeaders))
	for _, h := range baseHeader {
		header = append(header, h)
	}
	header = append(header, latestDiffHeader)
	for _, h := range dateHeaders {
		header = append(header, h)
	}

	out := make([][]any, 0, len(infos)+2)
	out = append(out, []any{metaLabel, today.Format(time.RFC3339)})
	out = append(out, header)

	for _, info := range infos {
		rankCells := make(map[string]string, len(dateHeaders))
		for _, dh := range dateHeaders {
			rankCells[dh] = rankForDate(dh, info.requestID, todayHeader, scrapeByID, snapshotByHeader, existingSheet.ranks)
		}
		diff := sheetDiff(todayHeader, dateHeaders, rankCells, rankOrder)

		row := make([]any, len(header))
		row[0] = info.requestID
		row[1] = info.dorm
		row[2] = info.roomType
		row[3] = info.size
		row[4] = diff
		for i, dh := range dateHeaders {
			row[5+i] = rankCells[dh]
		}
		out = append(out, row)
	}
	return out, nil
}

type parsedSheet struct {
	dateHeaders []string
	rows        map[string]map[string]string // request_id -> base col name -> value
	ranks       map[string]map[string]string // request_id -> dateHeader -> rank string
}

func parseExistingSheet(existing [][]any) (parsedSheet, bool) {
	out := parsedSheet{
		rows:  map[string]map[string]string{},
		ranks: map[string]map[string]string{},
	}
	if len(existing) < 2 {
		return out, false
	}

	headerRow := existing[1]
	if len(headerRow) == 0 {
		return out, false
	}

	legacy := false
	colIndex := map[string]int{}
	var dateHeaders []string
	for i, cell := range headerRow {
		name := fmt.Sprint(cell)
		colIndex[name] = i
		if name == "your_rank" {
			legacy = true
		}
		if t, ok := export.ParseDateHeader(name); ok {
			dateHeaders = append(dateHeaders, name)
			_ = t
		}
	}
	sortDateHeaders(dateHeaders)

	for _, h := range baseHeader {
		if _, ok := colIndex[h]; !ok {
			return parsedSheet{rows: map[string]map[string]string{}, ranks: map[string]map[string]string{}}, legacy
		}
	}
	if _, ok := colIndex["request_id"]; !ok {
		return parsedSheet{rows: map[string]map[string]string{}, ranks: map[string]map[string]string{}}, legacy
	}

	for r := 2; r < len(existing); r++ {
		row := existing[r]
		if len(row) == 0 {
			continue
		}
		id := fmt.Sprint(row[colIndex["request_id"]])
		if id == "" {
			continue
		}
		base := make(map[string]string, len(baseHeader))
		for _, h := range baseHeader {
			if colIndex[h] < len(row) {
				base[h] = fmt.Sprint(row[colIndex[h]])
			}
		}
		out.rows[id] = base

		if legacy {
			continue
		}
		ranks := make(map[string]string)
		for _, dh := range dateHeaders {
			idx := colIndex[dh]
			if idx < len(row) {
				val := fmt.Sprint(row[idx])
				if val != "" {
					ranks[dh] = val
				}
			}
		}
		if len(ranks) > 0 {
			out.ranks[id] = ranks
		}
	}

	out.dateHeaders = dateHeaders
	return out, legacy
}

func mergeDateHeaders(existingDates []string, snapshots []export.DailySnapshot, todayHeader string, legacy bool) []string {
	set := map[string]time.Time{}
	if !legacy {
		for _, h := range existingDates {
			if t, ok := export.ParseDateHeader(h); ok {
				set[h] = t
			}
		}
	}
	for _, snap := range snapshots {
		set[snap.DateHeader] = snap.Date
	}
	if t, ok := export.ParseDateHeader(todayHeader); ok {
		set[todayHeader] = t
	}

	out := make([]string, 0, len(set))
	for h := range set {
		out = append(out, h)
	}
	sortDateHeaders(out)
	return out
}

func sortDateHeaders(headers []string) {
	sort.Slice(headers, func(i, j int) bool {
		ti, okI := export.ParseDateHeader(headers[i])
		tj, okJ := export.ParseDateHeader(headers[j])
		if okI && okJ {
			return ti.Before(tj)
		}
		return headers[i] < headers[j]
	})
}

func collectRequestIDs(
	scrape map[string]model.WaitlistRow,
	snapshots []export.DailySnapshot,
	existingRows map[string]map[string]string,
) []string {
	set := map[string]struct{}{}
	for id := range scrape {
		set[id] = struct{}{}
	}
	for _, snap := range snapshots {
		for id := range snap.Ranks {
			set[id] = struct{}{}
		}
	}
	for id := range existingRows {
		set[id] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func buildRowInfo(
	id string,
	scrape map[string]model.WaitlistRow,
	snapshots []export.DailySnapshot,
	existingRows map[string]map[string]string,
) rowInfo {
	if row, ok := scrape[id]; ok {
		return rowInfo{
			requestID: id,
			dorm:      row.Dorm,
			roomType:  row.RoomType,
			size:      row.Size,
			todayRank: row.RankOrder,
			hasToday:  true,
		}
	}

	for i := len(snapshots) - 1; i >= 0; i-- {
		if row, ok := snapshots[i].Rows[id]; ok {
			return rowInfo{
				requestID: id,
				dorm:      row.Dorm,
				roomType:  row.RoomType,
				size:      row.Size,
			}
		}
	}

	if base, ok := existingRows[id]; ok {
		return rowInfo{
			requestID: id,
			dorm:      base["dorm"],
			roomType:  base["room_type"],
			size:      base["size_sqm"],
		}
	}

	return rowInfo{requestID: id}
}

func rankForDate(
	dateHeader, requestID, todayHeader string,
	scrape map[string]model.WaitlistRow,
	snapshots map[string]export.DailySnapshot,
	existingRanks map[string]map[string]string,
) string {
	if dateHeader == todayHeader {
		if row, ok := scrape[requestID]; ok {
			return row.RankDisplay
		}
	}
	if snap, ok := snapshots[dateHeader]; ok {
		if rank, ok := snap.Ranks[requestID]; ok {
			return rank
		}
	}
	if byID, ok := existingRanks[requestID]; ok {
		if rank, ok := byID[dateHeader]; ok {
			return rank
		}
	}
	return ""
}

func sheetDiff(todayHeader string, dateHeaders []string, rankCells map[string]string, rankOrder func(string) (int, bool)) string {
	todayStr, ok := rankCells[todayHeader]
	if !ok || todayStr == "" {
		return ""
	}
	todayOrder, ok := rankOrder(todayStr)
	if !ok {
		return ""
	}

	todayIdx := -1
	for i, h := range dateHeaders {
		if h == todayHeader {
			todayIdx = i
			break
		}
	}
	if todayIdx <= 0 {
		return ""
	}

	var prevOrder int
	found := false
	for i := todayIdx - 1; i >= 0; i-- {
		prevStr := rankCells[dateHeaders[i]]
		if prevStr == "" {
			continue
		}
		o, ok := rankOrder(prevStr)
		if !ok {
			continue
		}
		prevOrder = o
		found = true
		break
	}
	if !found {
		return ""
	}

	if prevOrder > todayOrder {
		return "+" + strconv.Itoa(prevOrder-todayOrder)
	}
	if prevOrder < todayOrder {
		return "-" + strconv.Itoa(todayOrder-prevOrder)
	}
	return "-"
}
