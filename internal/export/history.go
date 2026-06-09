package export

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"

	"denmark-housing-waitlist/internal/parser"
)

var waitlistCSVName = regexp.MustCompile(`^(\d{8})\d{4}_kkik_waitlist\.csv$`)

// DailySnapshot holds ranks and row metadata for one calendar day.
type DailySnapshot struct {
	DateHeader string
	Date       time.Time
	Ranks      map[string]int
	Rows       map[string]parser.WaitlistRow
}

// DateHeader returns a ddmmyy column header for t.
func DateHeader(t time.Time) string {
	return t.Format("020106")
}

// ParseDateHeader parses a ddmmyy header into a date (year 2000–2099).
func ParseDateHeader(header string) (time.Time, bool) {
	t, err := time.Parse("020106", header)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// LoadDailySnapshots reads all *_kkik_waitlist.csv files in dir, deduping same-day files
// by keeping the latest timestamp in the filename.
func LoadDailySnapshots(dir string) ([]DailySnapshot, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*_kkik_waitlist.csv"))
	if err != nil {
		return nil, fmt.Errorf("glob history csv: %w", err)
	}
	if len(matches) == 0 {
		return nil, nil
	}

	sort.Strings(matches)
	byDay := make(map[string]string)
	for _, path := range matches {
		base := filepath.Base(path)
		m := waitlistCSVName.FindStringSubmatch(base)
		if m == nil {
			continue
		}
		dayKey := m[1]
		if prev, ok := byDay[dayKey]; !ok || base > filepath.Base(prev) {
			byDay[dayKey] = path
		}
	}

	dayKeys := make([]string, 0, len(byDay))
	for k := range byDay {
		dayKeys = append(dayKeys, k)
	}
	sort.Strings(dayKeys)

	out := make([]DailySnapshot, 0, len(dayKeys))
	for _, dayKey := range dayKeys {
		snap, err := loadSnapshotFromCSV(byDay[dayKey], dayKey)
		if err != nil {
			return nil, err
		}
		out = append(out, snap)
	}
	return out, nil
}

func loadSnapshotFromCSV(path, dayKey string) (DailySnapshot, error) {
	date, err := time.Parse("20060102", dayKey)
	if err != nil {
		return DailySnapshot{}, fmt.Errorf("parse date from %s: %w", path, err)
	}

	f, err := os.Open(path)
	if err != nil {
		return DailySnapshot{}, fmt.Errorf("open history csv %s: %w", path, err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		return DailySnapshot{}, fmt.Errorf("read history csv %s: %w", path, err)
	}
	if len(records) < 2 {
		return DailySnapshot{
			DateHeader: DateHeader(date),
			Date:       date,
			Ranks:      map[string]int{},
			Rows:       map[string]parser.WaitlistRow{},
		}, nil
	}

	header := records[0]
	idIdx, dormIdx, roomIdx, sizeIdx, rankIdx := -1, -1, -1, -1, -1
	for i, col := range header {
		switch col {
		case "request_id":
			idIdx = i
		case "dorm":
			dormIdx = i
		case "room_type":
			roomIdx = i
		case "size_sqm":
			sizeIdx = i
		case "your_rank":
			rankIdx = i
		}
	}
	if idIdx < 0 || rankIdx < 0 {
		return DailySnapshot{}, fmt.Errorf("history csv %s: missing request_id or your_rank column", path)
	}

	ranks := make(map[string]int, len(records)-1)
	rows := make(map[string]parser.WaitlistRow, len(records)-1)
	for _, rec := range records[1:] {
		if len(rec) <= idIdx || len(rec) <= rankIdx {
			continue
		}
		id := rec[idIdx]
		if id == "" {
			continue
		}
		rank, err := strconv.Atoi(rec[rankIdx])
		if err != nil {
			return DailySnapshot{}, fmt.Errorf("history csv %s: invalid rank for %s: %w", path, id, err)
		}
		ranks[id] = rank
		row := parser.WaitlistRow{RequestID: id, YourRank: rank}
		if dormIdx >= 0 && len(rec) > dormIdx {
			row.Dorm = rec[dormIdx]
		}
		if roomIdx >= 0 && len(rec) > roomIdx {
			row.RoomType = rec[roomIdx]
		}
		if sizeIdx >= 0 && len(rec) > sizeIdx {
			row.Size = rec[sizeIdx]
		}
		rows[id] = row
	}

	return DailySnapshot{
		DateHeader: DateHeader(date),
		Date:       date,
		Ranks:      ranks,
		Rows:       rows,
	}, nil
}
