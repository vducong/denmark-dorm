package sdk

import (
	"fmt"
	"regexp"
	"strings"

	"housing-waitlist/internal/model"

	"github.com/PuerkitoBio/goquery"
)

var (
	tenancyIDRe  = regexp.MustCompile(`/studiebolig/tenancy/(\d+)/`)
	rankLetterRe = regexp.MustCompile(`\b[A-Z]\b`)
	sizeRe       = regexp.MustCompile(`\d+`)
)

const (
	notSetDisplay = "Not set"
	notSetOrder   = 99
)

// parseHTML extracts waitlist rows from the stitched s.dk building pages.
//
// The scraper wraps each building's ranking tables in a
// <section class="sdk-building" data-name="...">.
// Every table row that the applicant is signed up for carries a waiting-list category label
// (a letter A–G, or "Not set" until s.dk calculates its position).
func parseHTML(htmlStr string) (*model.Result, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlStr))
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}

	var rows []model.WaitlistRow
	doc.Find("section.sdk-building").Each(func(_ int, sec *goquery.Selection) {
		dorm := strings.TrimSpace(sec.AttrOr("data-name", ""))
		groups := parseRentGroups(sec)
		sec.Find("tr").Each(func(_ int, tr *goquery.Selection) {
			row, ok := parseRow(tr, dorm)
			if !ok {
				return
			}
			if min, max, ok := rentForSize(groups, row.Size); ok {
				row.RentMin, row.RentMax = min, max
			}
			rows = append(rows, row)
		})
	})

	if len(rows) == 0 {
		return nil, fmt.Errorf("no ranked tenancies found")
	}
	return &model.Result{Rows: rows}, nil
}

// parseRow turns one tenancy table row into a WaitlistRow.
// The presence of a waiting-list-category label marks a tenancy the applicant joined;
// rows without one (headers, or not-signed-up tenancies) are skipped.
func parseRow(tr *goquery.Selection, dorm string) (model.WaitlistRow, bool) {
	label := tr.Find(selRankCell).First()
	if label.Length() == 0 {
		return model.WaitlistRow{}, false
	}

	link := tr.Find(`a[href*="/studiebolig/tenancy/"]`).First()
	href := link.AttrOr("href", "")
	m := tenancyIDRe.FindStringSubmatch(href)
	if m == nil {
		return model.WaitlistRow{}, false
	}

	display, order := rankFromLabel(label)
	// The link text is a full street address (e.g. "Nørrebrogade 9E 2 206,
	// 2200 København N"), so it doubles as both the room label and the commute
	// origin; KKIK has no such address and resolves its origin from config.
	addr := normalizeSpace(link.Text())
	return model.WaitlistRow{
		RequestID:   m[1],
		Dorm:        dorm,
		URL:         absoluteURL(href),
		RoomType:    addr,
		Size:        sizeRe.FindString(tr.Find("td:has(sup)").First().Text()),
		RankDisplay: display,
		RankOrder:   order,
		Address:     addr,
	}, true
}

// absoluteURL turns an s.dk tenancy href into a full URL.
// Building and tenancy links render as site-relative paths,
// so a relative href gets the baseURL host prepended;
// an already-absolute href is returned unchanged.
func absoluteURL(href string) string {
	if href == "" || strings.HasPrefix(href, "http") {
		return href
	}
	return baseURL + href
}

// rankFromLabel reads the rank from a waiting-list-category label:
// a letter span when calculated, otherwise the "Not set" placeholder.
func rankFromLabel(label *goquery.Selection) (string, int) {
	if letter := rankLetterRe.FindString(label.Find(selRankLetter).First().Text()); letter != "" {
		order, _ := rankOrder(letter)
		return letter, order
	}
	return notSetDisplay, notSetOrder
}

func normalizeSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// rankOrder maps an s.dk rank to its sortable order (lower = better):
// a letter category A=1…G=7, or "Not set" last.
// It is also reused to convert a stored rank back to an order when diffing against the previous run.
func rankOrder(display string) (int, bool) {
	d := strings.TrimSpace(display)
	if d == notSetDisplay {
		return notSetOrder, true
	}
	if len(d) == 1 && d[0] >= 'A' && d[0] <= 'Z' {
		return int(d[0]-'A') + 1, true
	}
	return 0, false
}
