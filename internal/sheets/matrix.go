package sheets

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"housing-waitlist/internal/export"
	"housing-waitlist/internal/model"
)

// coreBase are the always-present base columns. The config-driven commute
// columns are appended after these to form a run's full base header.
var coreBase = []string{"request_id", "dorm", "url", "room_type", "size_sqm"}

// requiredBaseCols must be present in an existing sheet for its dated history to
// be trusted. url and commute columns are newer/optional: a sheet written before
// they existed is still valid and gets them backfilled, so their absence must
// NOT void the parse (this guard is what keeps a column-add migration safe).
var requiredBaseCols = map[string]bool{
	"request_id": true, "dorm": true, "room_type": true, "size_sqm": true,
}

const (
	latestDiffHeader = "latest_diff"
	metaLabel        = "Last updated at"
)

type rowInfo struct {
	requestID string
	dorm      string
	url       string
	roomType  string
	size      string
	commute   map[string]string // commute column name -> value
	todayRank int
	hasToday  bool
}

// BuildMatrix merges scrape data, CSV snapshots, and existing sheet values into sheet rows.
func BuildMatrix(
	rows []model.WaitlistRow,
	snapshots []export.DailySnapshot,
	existing [][]any,
	today time.Time,
	note string,
	rankOrder func(string) (int, bool),
	commuteCols []string,
) ([][]any, error) {
	// baseCols is the run's full base header: the always-present core columns
	// followed by the config-driven commute columns.
	baseCols := append(append([]string{}, coreBase...), commuteCols...)
	todayHeader := export.DateHeader(today)
	scrapeByID := make(map[string]model.WaitlistRow, len(rows))
	for _, row := range rows {
		scrapeByID[row.RequestID] = row
	}

	snapshotByHeader := make(map[string]export.DailySnapshot, len(snapshots))
	for _, snap := range snapshots {
		snapshotByHeader[snap.DateHeader] = snap
	}

	existingSheet, legacy := parseExistingSheet(existing, baseCols)

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
		info := buildRowInfo(id, scrapeByID, snapshots, existingSheet.rows, commuteCols)
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

	header := make([]any, 0, len(baseCols)+1+len(dateHeaders))
	for _, h := range baseCols {
		header = append(header, h)
	}
	header = append(header, latestDiffHeader)
	for _, h := range dateHeaders {
		header = append(header, h)
	}

	out := make([][]any, 0, len(infos)+3)
	out = append(out, []any{metaLabel, today.Format(time.RFC3339)})
	out = append(out, []any{note})
	out = append(out, header)

	for _, info := range infos {
		rankCells := make(map[string]string, len(dateHeaders))
		for _, dh := range dateHeaders {
			rankCells[dh] = rankForDate(dh, info.requestID, todayHeader, scrapeByID, snapshotByHeader, existingSheet.ranks)
		}
		diff := sheetDiff(todayHeader, dateHeaders, rankCells, rankOrder)

		// latest_diff sits immediately after the base columns, then the dated rank columns;
		// deriving the offsets from len(baseCols) keeps these in step whenever a base column is added.
		diffCol := len(baseCols)
		row := make([]any, len(header))
		row[0] = info.requestID
		row[1] = info.dorm
		row[2] = info.url
		row[3] = info.roomType
		row[4] = info.size
		for i, col := range commuteCols {
			row[len(coreBase)+i] = info.commute[col]
		}
		row[diffCol] = diff
		for i, dh := range dateHeaders {
			row[diffCol+1+i] = rankCells[dh]
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

func parseExistingSheet(existing [][]any, baseCols []string) (parsedSheet, bool) {
	out := parsedSheet{
		rows:  map[string]map[string]string{},
		ranks: map[string]map[string]string{},
	}
	// Locate the header row by content rather than a fixed index,
	// so the parser survives layout shifts such as an added note row between metadata and header.
	headerRowIdx := -1
	for i, row := range existing {
		for _, cell := range row {
			if fmt.Sprint(cell) == "request_id" {
				headerRowIdx = i
				break
			}
		}
		if headerRowIdx >= 0 {
			break
		}
	}
	if headerRowIdx < 0 {
		return out, false
	}

	headerRow := existing[headerRowIdx]
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

	for _, h := range baseCols {
		// Only a missing REQUIRED column means the sheet is unrecognizable and the
		// history must be discarded. url and commute columns are newer/optional, so
		// older sheets that predate them parse normally and get them backfilled.
		if !requiredBaseCols[h] {
			continue
		}
		if _, ok := colIndex[h]; !ok {
			return parsedSheet{rows: map[string]map[string]string{}, ranks: map[string]map[string]string{}}, legacy
		}
	}

	for r := headerRowIdx + 1; r < len(existing); r++ {
		row := existing[r]
		if len(row) == 0 {
			continue
		}
		id := fmt.Sprint(row[colIndex["request_id"]])
		if id == "" {
			continue
		}
		base := make(map[string]string, len(baseCols))
		for _, h := range baseCols {
			if idx, ok := colIndex[h]; ok && idx < len(row) {
				base[h] = fmt.Sprint(row[idx])
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
	commuteCols []string,
) rowInfo {
	if row, ok := scrape[id]; ok {
		return rowInfo{
			requestID: id,
			dorm:      row.Dorm,
			url:       row.URL,
			roomType:  row.RoomType,
			size:      row.Size,
			commute:   row.Commute,
			todayRank: row.RankOrder,
			hasToday:  true,
		}
	}

	for i := len(snapshots) - 1; i >= 0; i-- {
		if row, ok := snapshots[i].Rows[id]; ok {
			return rowInfo{
				requestID: id,
				dorm:      row.Dorm,
				url:       row.URL,
				roomType:  row.RoomType,
				size:      row.Size,
				commute:   row.Commute,
			}
		}
	}

	if base, ok := existingRows[id]; ok {
		return rowInfo{
			requestID: id,
			dorm:      base["dorm"],
			url:       base["url"],
			roomType:  base["room_type"],
			size:      base["size_sqm"],
			commute:   commuteFromBase(base, commuteCols),
		}
	}

	return rowInfo{requestID: id}
}

// commuteFromBase pulls the commute columns out of an existing sheet row, so a
// row no longer in the live scrape keeps the commute values already on the sheet.
func commuteFromBase(base map[string]string, commuteCols []string) map[string]string {
	if len(commuteCols) == 0 {
		return nil
	}
	out := make(map[string]string, len(commuteCols))
	for _, col := range commuteCols {
		if v := base[col]; v != "" {
			out[col] = v
		}
	}
	return out
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
