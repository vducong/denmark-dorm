package sdk

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"housing-waitlist/internal/model"
)

var (
	tenancyIDRe  = regexp.MustCompile(`/studiebolig/tenancy/(\d+)/`)
	rankLetterRe = regexp.MustCompile(`\b[A-Z]\b`)
	sizeRe       = regexp.MustCompile(`\d+`)
)

// notSet is the rank shown for a tenancy whose waiting-list position s.dk has
// not calculated yet; it sorts after every lettered rank.
const (
	notSetDisplay = "Not set"
	notSetOrder   = 99
)

// parseHTML extracts waitlist rows from the stitched s.dk building pages.
//
// The scraper wraps each building's ranking tables in a
// <section class="sdk-building" data-name="...">. Every table row that the
// applicant is signed up for carries a waiting-list category label (a letter
// A–G, or "Not set" until s.dk calculates its position).
func parseHTML(htmlStr string) (*model.Result, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlStr))
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}

	var rows []model.WaitlistRow
	doc.Find("section.sdk-building").Each(func(_ int, sec *goquery.Selection) {
		dorm := strings.TrimSpace(sec.AttrOr("data-name", ""))
		sec.Find("tr").Each(func(_ int, tr *goquery.Selection) {
			if row, ok := parseRow(tr, dorm); ok {
				rows = append(rows, row)
			}
		})
	})

	if len(rows) == 0 {
		return nil, fmt.Errorf("no ranked tenancies found")
	}
	return &model.Result{Rows: rows}, nil
}

// parseRow turns one tenancy table row into a WaitlistRow. The presence of a
// waiting-list-category label marks a tenancy the applicant joined; rows without
// one (headers, or not-signed-up tenancies) are skipped.
func parseRow(tr *goquery.Selection, dorm string) (model.WaitlistRow, bool) {
	label := tr.Find(selRankCell).First()
	if label.Length() == 0 {
		return model.WaitlistRow{}, false
	}

	link := tr.Find(`a[href*="/studiebolig/tenancy/"]`).First()
	m := tenancyIDRe.FindStringSubmatch(link.AttrOr("href", ""))
	if m == nil {
		return model.WaitlistRow{}, false
	}

	display, order := rankFromLabel(label)
	return model.WaitlistRow{
		RequestID:   m[1],
		Dorm:        dorm,
		RoomType:    normalizeSpace(link.Text()),
		Size:        sizeRe.FindString(tr.Find("td:has(sup)").First().Text()),
		RankDisplay: display,
		RankOrder:   order,
	}, true
}

// rankFromLabel reads the rank from a waiting-list-category label: a letter span
// when calculated, otherwise the "Not set" placeholder.
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

// rankOrder maps an s.dk rank to its sortable order (lower = better): a letter
// category A=1…G=7, or "Not set" last. It is also reused to convert a stored
// rank back to an order when diffing against the previous run.
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
