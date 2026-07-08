// Package scoring ranks waitlist rooms by desirability (rent + commute + size)
// and opportunity (desirability blended with waitlist position), producing the
// merged best-candidates list. Each factor is normalized to [0,1] (1 = best)
// via a hybrid scheme — fixed absolute bands for commute and rent, relative
// min-max for size — then weight-summed. Rank is normalized per source because
// a KKIK queue position and an s.dk letter grade are not comparable.
package scoring

import (
	"sort"
	"strconv"
	"strings"

	"housing-waitlist/internal/commute"
	"housing-waitlist/internal/model"
)

// Weights set each factor's share of the desirability score.
type Weights struct{ Commute, Size, Rent float64 }

// Settings is the source-agnostic scoring configuration, projected from config.
type Settings struct {
	Enabled         bool
	MaxRent         int     // DKK/mo budget gate; 0 disables it and rent falls back to relative min-max.
	RentFloor       int     // DKK/mo lower bound of the rent band.
	CommuteBestMin  int     // commute ≤ this scores 1.0.
	CommuteWorstMin int     // commute ≥ this scores 0.0.
	Weights         Weights // desirability factor weights.
	RankWeight      float64 // opportunity's blend toward waitlist position.
	SortBy          string  // "desirability" (default) or "opportunity".
	DestNames       []string
}

// Input is one row tagged with the source it came from (rank is normalized
// within a source, so the tag is load-bearing).
type Input struct {
	Source string
	Row    model.WaitlistRow
}

// ScoredRow is an Input with its two computed scores (0–100).
type ScoredRow struct {
	Input
	Desirability float64
	Opportunity  float64
}

// raw holds a row's extracted factor values and which ones are present.
type raw struct {
	commute    float64
	hasCommute bool
	size       float64
	hasSize    bool
	rent       float64
	hasRent    bool
	rank       int
	hasRank    bool
}

// Rank applies the budget gate, normalizes factors (commute/rent/size globally,
// rank per source), computes both scores, and returns the rows sorted best-first.
func Rank(in []Input, s Settings) []ScoredRow {
	kept := make([]Input, 0, len(in))
	for _, it := range in {
		if rentExceeds(it.Row.RentMin, it.Row.RentMax, s.MaxRent) {
			continue
		}
		kept = append(kept, it)
	}

	raws := make([]raw, len(kept))
	for i, it := range kept {
		var r raw
		if c, ok := avgCommute(it.Row, s.DestNames); ok {
			r.commute, r.hasCommute = c, true
		}
		if sz, ok := parseSize(it.Row.Size); ok {
			r.size, r.hasSize = sz, true
		}
		if rv, ok := rentValue(it.Row.RentMin, it.Row.RentMax); ok {
			r.rent, r.hasRent = rv, true
		}
		if !isUnranked(it.Row) {
			r.rank, r.hasRank = it.Row.RankOrder, true
		}
		raws[i] = r
	}

	sizeMin, sizeMax, _ := minMax(raws, func(r raw) (float64, bool) { return r.size, r.hasSize })
	rentMin, rentMax, _ := minMax(raws, func(r raw) (float64, bool) { return r.rent, r.hasRent })
	rankBounds := perSourceRankBounds(kept, raws)

	out := make([]ScoredRow, len(kept))
	for i, it := range kept {
		r := raws[i]
		var sum, sumW float64
		add := func(w, norm float64) {
			sum += w * norm
			sumW += w
		}
		if r.hasCommute {
			add(s.Weights.Commute, normCommute(r.commute, s.CommuteBestMin, s.CommuteWorstMin))
		}
		if r.hasSize {
			add(s.Weights.Size, normRelative(r.size, sizeMin, sizeMax, true))
		}
		if r.hasRent {
			add(s.Weights.Rent, normRent(r.rent, s, rentMin, rentMax))
		}
		var desUnit float64
		if sumW > 0 {
			desUnit = sum / sumW
		}

		var oppUnit float64
		if r.hasRank {
			b := rankBounds[it.Source]
			normRank := normRelative(float64(r.rank), float64(b[0]), float64(b[1]), false)
			oppUnit = (1-s.RankWeight)*desUnit + s.RankWeight*normRank
		} else {
			// No waitlist rank (catalog row or "Not set") — opportunity equals
			// desirability so the room still sorts by quality alone.
			oppUnit = desUnit
		}

		out[i] = ScoredRow{Input: it, Desirability: desUnit * 100, Opportunity: oppUnit * 100}
	}

	sortScored(out, s.SortBy)
	return out
}

// avgCommute averages the morning-transit minutes across the configured
// destinations, ignoring blanks. Reports false when none are available.
func avgCommute(row model.WaitlistRow, dests []string) (float64, bool) {
	var sum float64
	var n int
	for _, d := range dests {
		v := strings.TrimSpace(row.Commute[commute.TransitMorningCol(d)])
		if v == "" {
			continue
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			continue
		}
		sum += f
		n++
	}
	if n == 0 {
		return 0, false
	}
	return sum / float64(n), true
}

// isUnranked reports rows with no real waitlist position — s.dk's "Not set"
// (caught by display, not the 99 sentinel, which is a valid KKIK position).
func isUnranked(row model.WaitlistRow) bool {
	rd := strings.ToLower(strings.TrimSpace(row.RankDisplay))
	return rd == "" || rd == "not set" || row.RankOrder <= 0
}

func perSourceRankBounds(kept []Input, raws []raw) map[string][2]int {
	bounds := map[string][2]int{}
	for i, it := range kept {
		if !raws[i].hasRank {
			continue
		}
		v := raws[i].rank
		b, ok := bounds[it.Source]
		if !ok {
			bounds[it.Source] = [2]int{v, v}
			continue
		}
		if v < b[0] {
			b[0] = v
		}
		if v > b[1] {
			b[1] = v
		}
		bounds[it.Source] = b
	}
	return bounds
}

// normCommute maps minutes onto [0,1] via the fixed best/worst band.
func normCommute(x float64, best, worst int) float64 {
	b, w := float64(best), float64(worst)
	if w <= b {
		if x <= b {
			return 1
		}
		return 0
	}
	return clamp((w - x) / (w - b))
}

// normRent uses the budget band when MaxRent is set, else relative min-max.
func normRent(rent float64, s Settings, relMin, relMax float64) float64 {
	if s.MaxRent <= 0 {
		return normRelative(rent, relMin, relMax, false)
	}
	mr, fl := float64(s.MaxRent), float64(s.RentFloor)
	if mr <= fl {
		if rent <= fl {
			return 1
		}
		return 0
	}
	return clamp((mr - rent) / (mr - fl))
}

// normRelative min-max scales x; higherBetter flips lower-is-better factors.
// A degenerate range (all equal / single value) scores 1.0.
func normRelative(x, min, max float64, higherBetter bool) float64 {
	if max <= min {
		return 1
	}
	frac := (x - min) / (max - min)
	if !higherBetter {
		frac = 1 - frac
	}
	return clamp(frac)
}

func minMax(raws []raw, get func(raw) (float64, bool)) (min, max float64, ok bool) {
	for _, r := range raws {
		v, has := get(r)
		if !has {
			continue
		}
		if !ok {
			min, max, ok = v, v, true
			continue
		}
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	return min, max, ok
}

func clamp(x float64) float64 {
	switch {
	case x < 0:
		return 0
	case x > 1:
		return 1
	default:
		return x
	}
}

// sortScored orders by the chosen score descending, with the other score, then
// source and request id, as deterministic tie-breakers.
func sortScored(rows []ScoredRow, sortBy string) {
	byOpp := strings.EqualFold(sortBy, "opportunity")
	sort.SliceStable(rows, func(i, j int) bool {
		pi, pj := rows[i].Desirability, rows[j].Desirability
		si, sj := rows[i].Opportunity, rows[j].Opportunity
		if byOpp {
			pi, pj, si, sj = si, sj, pi, pj
		}
		if pi != pj {
			return pi > pj
		}
		if si != sj {
			return si > sj
		}
		if rows[i].Source != rows[j].Source {
			return rows[i].Source < rows[j].Source
		}
		return rows[i].Row.RequestID < rows[j].Row.RequestID
	})
}
