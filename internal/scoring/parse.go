package scoring

import (
	"regexp"
	"strconv"
	"strings"
)

// numRe matches a single number, allowing a Danish decimal comma or a dot.
var numRe = regexp.MustCompile(`\d+(?:[.,]\d+)?`)

// rangeRe matches a leading "a - b" range so "43-45 m²" collapses to its
// midpoint, while "39 (27) m²" (parenthetical, not a range) does not.
var rangeRe = regexp.MustCompile(`^\s*(\d+(?:[.,]\d+)?)\s*-\s*(\d+(?:[.,]\d+)?)`)

// parseSize extracts a numeric m² value from a source's raw size string.
// A range collapses to its midpoint; anything else uses the first number.
func parseSize(s string) (float64, bool) {
	if m := rangeRe.FindStringSubmatch(s); m != nil {
		lo, okLo := parseNum(m[1])
		hi, okHi := parseNum(m[2])
		if okLo && okHi {
			return (lo + hi) / 2, true
		}
	}
	if first := numRe.FindString(s); first != "" {
		return parseNum(first)
	}
	return 0, false
}

func parseNum(s string) (float64, bool) {
	f, err := strconv.ParseFloat(strings.Replace(s, ",", ".", 1), 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// rentValue is the representative monthly rent used for scoring: the midpoint
// when both bounds are known, otherwise whichever single bound is present.
// Reports false when rent is unknown (both bounds zero).
func rentValue(min, max int) (float64, bool) {
	switch {
	case min > 0 && max > 0:
		return float64(min+max) / 2, true
	case max > 0:
		return float64(max), true
	case min > 0:
		return float64(min), true
	default:
		return 0, false
	}
}

// rentExceeds is the hard budget gate: true when a room's cheapest rent is
// above maxRent. A zero maxRent disables the gate, and unknown rent is never
// excluded.
func rentExceeds(min, max, maxRent int) bool {
	if maxRent <= 0 {
		return false
	}
	floor := min
	if floor <= 0 {
		floor = max
	}
	if floor <= 0 {
		return false
	}
	return floor > maxRent
}
