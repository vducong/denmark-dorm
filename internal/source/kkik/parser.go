package kkik

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"housing-waitlist/internal/model"
)

var rankRe = regexp.MustCompile(`\d+`)

// parseHTML extracts waitlist rows from KKIK housing requests HTML.
func parseHTML(html string) (*model.Result, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}

	res := &model.Result{}
	res.Meta = extractMeta(doc)

	grid := doc.Find("div.func-housingrequests div.body.grid").First()
	if grid.Length() == 0 {
		return nil, fmt.Errorf("housing requests grid not found")
	}

	var rows []model.WaitlistRow
	grid.Find("div.row.header").Each(func(_ int, row *goquery.Selection) {
		if row.Find("input[name*='housingRequestRepeater'][name*='checkBox']").Length() == 0 {
			return
		}

		requestID := findRequestID(row)
		dorm := cleanText(row.Find("div.col-xs-8.col-md-3").First())
		room := normalizeSpace(roomCell(row.Find("div.col-xs-8.col-md-4").First()))
		size := cleanText(row.Find("div.col-xs-8.col-md-2").Eq(0))
		rankText := cleanText(row.Find("div.col-xs-8.col-md-2").Eq(1))
		order, ok := rankOrder(rankText)
		if !ok {
			return
		}

		rows = append(rows, model.WaitlistRow{
			RequestID:   requestID,
			Dorm:        dorm,
			RoomType:    room,
			Size:        size,
			RankDisplay: strconv.Itoa(order),
			RankOrder:   order,
		})
	})

	if len(rows) == 0 {
		return nil, fmt.Errorf("no housing request rows found")
	}

	res.Rows = rows
	return res, nil
}

func extractMeta(doc *goquery.Document) model.Meta {
	var meta model.Meta

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

// rankOrder maps a KKIK rank string ("3", "3.") to its numeric order.
func rankOrder(text string) (int, bool) {
	m := rankRe.FindString(strings.ReplaceAll(text, ".", ""))
	if m == "" {
		return 0, false
	}
	n, err := strconv.Atoi(m)
	if err != nil {
		return 0, false
	}
	return n, true
}
