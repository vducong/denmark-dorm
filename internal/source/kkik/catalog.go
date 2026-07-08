package kkik

import (
	"context"
	"fmt"
	"strings"

	"housing-waitlist/internal/model"

	"github.com/PuerkitoBio/goquery"
)

// FetchCatalog fetches KKIK's public catalog page and returns one WaitlistRow
// per room type across every dorm, unranked (RankOrder=99). It needs no
// login and no pagination: the same page EnrichRent uses already lists every
// dorm and room type at once. Address is left blank — the commute resolver's
// existing AddressResolver fallback (LookupAddress) fills it per building.
func (k *KKIK) FetchCatalog(ctx context.Context) (*model.Result, error) {
	body, err := fetchKollegiumList(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := parseCatalog(body)
	if err != nil {
		return nil, err
	}
	return &model.Result{Rows: rows}, nil
}

// parseCatalog walks each dorm block (div.kollegium.row) and returns one row
// per room type found inside it. A room row is one div.row holding a
// roomtypedetails link and exactly three sibling col-md-2 cells (size, unit
// count, price, in that fixed order); rows with a different cell count —
// the column-header row, or one still missing its price — are skipped.
func parseCatalog(htmlStr string) ([]model.WaitlistRow, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlStr))
	if err != nil {
		return nil, fmt.Errorf("parse catalog html: %w", err)
	}

	var rows []model.WaitlistRow
	doc.Find("div.kollegium.row").Each(func(_ int, block *goquery.Selection) {
		dorm := cleanText(block.Find("h3 a").First())
		if dorm == "" {
			return
		}
		block.Find("a[href*='roomtypedetails']").Each(func(_ int, link *goquery.Selection) {
			href := link.AttrOr("href", "")
			arid := aridFromURL(href)
			if arid == "" {
				return
			}
			cells := link.Closest("div.row").ChildrenFiltered("div.col-md-2")
			if cells.Length() != 3 {
				return
			}
			rent, ok := parseKr(cleanText(cells.Eq(2)))
			if !ok {
				return
			}
			rows = append(rows, model.WaitlistRow{
				RequestID:   arid,
				Dorm:        dorm,
				URL:         absoluteURL(href),
				RoomType:    cleanText(link),
				Size:        cleanText(cells.Eq(0)),
				RankDisplay: "",
				RankOrder:   99,
				RentMin:     rent,
				RentMax:     rent,
			})
		})
	})
	return rows, nil
}
