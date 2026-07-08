package sdk

import (
	"regexp"
	"strconv"

	"github.com/PuerkitoBio/goquery"
)

// rentGroupRe parses a building page's room-type group line, e.g.
// "1-room tenancies 37-37 m2, 2588-2875 kr." — a size band and a rent band (DKK,
// both written as "min-max" even when equal). The leading label is ignored.
var rentGroupRe = regexp.MustCompile(`(\d+)(?:-(\d+))?\s*m2?\s*,\s*(\d+)(?:-(\d+))?\s*kr`)

// rentGroup is one room type's size band and monthly-rent band within a building.
type rentGroup struct {
	sizeMin, sizeMax int
	rentMin, rentMax int
}

// parseRentGroups reads the per-building rent groups the scraper captured into
// <div class="sdk-rent-groups"> (one span per room type). s.dk shows rent only
// per group, so a tenancy's rent is its group's band, matched by size.
func parseRentGroups(sec *goquery.Selection) []rentGroup {
	var groups []rentGroup
	sec.Find("div.sdk-rent-groups span").Each(func(_ int, s *goquery.Selection) {
		m := rentGroupRe.FindStringSubmatch(s.Text())
		if m == nil {
			return
		}
		groups = append(groups, rentGroup{
			sizeMin: atoi(m[1]), sizeMax: atoiOr(m[2], m[1]),
			rentMin: atoi(m[3]), rentMax: atoiOr(m[4], m[3]),
		})
	})
	return groups
}

// rentForSize returns the rent band of the first group whose size band contains
// the given size (in m²), or ok=false when none matches.
func rentForSize(groups []rentGroup, sizeStr string) (min, max int, ok bool) {
	n, err := strconv.Atoi(sizeStr)
	if err != nil {
		return 0, 0, false
	}
	for _, g := range groups {
		if n >= g.sizeMin && n <= g.sizeMax {
			return g.rentMin, g.rentMax, true
		}
	}
	return 0, 0, false
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func atoiOr(s, fallback string) int {
	if s == "" {
		return atoi(fallback)
	}
	return atoi(s)
}
