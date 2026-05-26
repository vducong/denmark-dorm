package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// WaitlistRow is one housing request on the accommodation wishes page.
type WaitlistRow struct {
	RequestID string
	Dorm      string
	RoomType  string
	Size      string
	YourRank  int
}

// PageMeta holds optional header information from the list page.
type PageMeta struct {
	ApplicantName   string
	RenewalDeadline string
}

// Result is the parsed page content.
type Result struct {
	Rows []WaitlistRow
	Meta PageMeta
}

var rankRe = regexp.MustCompile(`\d+`)

// ParseHTML extracts waitlist rows from KKIK housing requests HTML.
func ParseHTML(html string) (*Result, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}

	res := &Result{}
	res.Meta = extractMeta(doc)

	grid := doc.Find("div.func-housingrequests div.body.grid").First()
	if grid.Length() == 0 {
		return nil, fmt.Errorf("housing requests grid not found")
	}

	var rows []WaitlistRow
	grid.Find("div.row.header").Each(func(_ int, row *goquery.Selection) {
		if row.Find("input[name*='housingRequestRepeater'][name*='checkBox']").Length() == 0 {
			return
		}

		requestID := findRequestID(row)
		dorm := cleanText(row.Find("div.col-xs-8.col-md-3").First())
		room := normalizeSpace(roomCell(row.Find("div.col-xs-8.col-md-4").First()))
		size := cleanText(row.Find("div.col-xs-8.col-md-2").Eq(0))
		rankText := cleanText(row.Find("div.col-xs-8.col-md-2").Eq(1))
		rank, err := parseRank(rankText)
		if err != nil {
			return
		}

		rows = append(rows, WaitlistRow{
			RequestID: requestID,
			Dorm:      dorm,
			RoomType:  room,
			Size:      size,
			YourRank:  rank,
		})
	})

	if len(rows) == 0 {
		return nil, fmt.Errorf("no housing request rows found")
	}

	res.Rows = rows
	return res, nil
}

func extractMeta(doc *goquery.Document) PageMeta {
	var meta PageMeta

	doc.Find("div.func-housingrequests h3").Each(func(_ int, s *goquery.Selection) {
		text := cleanText(s)
		const prefix = "Housing requests for"
		if strings.HasPrefix(text, prefix) {
			meta.ApplicantName = strings.TrimSpace(strings.TrimPrefix(text, prefix))
		}
	})

	bodyText := doc.Find("div.func-housingrequests").First().Text()
	if idx := strings.Index(bodyText, "renewed before"); idx >= 0 {
		fragment := bodyText[idx+len("renewed before"):]
		if end := strings.IndexAny(fragment, "\n\r<"); end >= 0 {
			fragment = fragment[:end]
		}
		meta.RenewalDeadline = strings.TrimSpace(fragment)
	}

	return meta
}

func findRequestID(row *goquery.Selection) string {
	prev := row.Prev()
	for prev.Length() > 0 {
		if prev.Is("input[type='hidden']") {
			if name, _ := prev.Attr("name"); strings.Contains(name, "housingRequestRepeater") && strings.Contains(name, "hiddenField") {
				if val, ok := prev.Attr("value"); ok {
					return strings.TrimSpace(val)
				}
			}
		}
		if val, ok := prev.Find("input[type='hidden'][name*='housingRequestRepeater'][name*='hiddenField']").Attr("value"); ok {
			return strings.TrimSpace(val)
		}
		if prev.Is("hr") {
			prev = prev.Prev()
			continue
		}
		prev = prev.Prev()
	}
	return ""
}

func roomCell(s *goquery.Selection) string {
	if link := s.Find("a").First(); link.Length() > 0 {
		return link.Text()
	}
	return s.Text()
}

func cleanText(s *goquery.Selection) string {
	return normalizeSpace(s.Text())
}

func normalizeSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func parseRank(text string) (int, error) {
	m := rankRe.FindString(strings.ReplaceAll(text, ".", ""))
	if m == "" {
		return 0, fmt.Errorf("no rank in %q", text)
	}
	n, err := strconv.Atoi(m)
	if err != nil {
		return 0, fmt.Errorf("parse rank %q: %w", text, err)
	}
	return n, nil
}
