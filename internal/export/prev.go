package export

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// LoadPrevRanks reads request_id → RankOrder from the latest *_waitlist.csv in dir,
// converting each your_rank display value via rankOrder.
// Returns an empty map when no previous files exist.
func LoadPrevRanks(dir string, rankOrder func(string) (int, bool)) (map[string]int, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*_waitlist.csv"))
	if err != nil {
		return nil, fmt.Errorf("glob prev csv: %w", err)
	}
	if len(matches) == 0 {
		return map[string]int{}, nil
	}

	sort.Strings(matches)
	latest := matches[len(matches)-1]

	f, err := os.Open(latest)
	if err != nil {
		return nil, fmt.Errorf("open prev csv %s: %w", latest, err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read prev csv %s: %w", latest, err)
	}
	if len(records) < 2 {
		return map[string]int{}, nil
	}

	header := records[0]
	idIdx, rankIdx := -1, -1
	for i, col := range header {
		switch col {
		case "request_id":
			idIdx = i
		case "your_rank":
			rankIdx = i
		}
	}
	if idIdx < 0 || rankIdx < 0 {
		return nil, fmt.Errorf("prev csv %s: missing request_id or your_rank column", latest)
	}

	out := make(map[string]int, len(records)-1)
	for _, rec := range records[1:] {
		if len(rec) <= idIdx || len(rec) <= rankIdx {
			continue
		}
		id := rec[idIdx]
		if id == "" {
			continue
		}
		order, ok := rankOrder(rec[rankIdx])
		if !ok {
			continue
		}
		out[id] = order
	}
	return out, nil
}
